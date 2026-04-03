package services

import (
	"context"
	"errors"
	"fmt"
	"log"
	"reflect"
	"unsafe"

	"freegfw/database"
	"freegfw/models"

	"github.com/xtls/xray-core/common/protocol"
	xray_inbound "github.com/xtls/xray-core/features/inbound"
	xray_proxy "github.com/xtls/xray-core/proxy"
)

type getInbound interface {
	GetInbound() xray_proxy.Inbound
}

func (c *CoreService) HotReloadUsers() error {
	log.Println("[HotReload] Attempting to hot-reload users into memory...")

	// 1. Fetch current users from database dynamically just like BuildUsers
	var t models.Setting
	database.DB.Where("key = ?", "template").Limit(1).Find(&t)
	var templateName string
	if len(t.Value) > 0 {
		templateName = string(t.Value)
		// strip quotes if any
		if len(templateName) >= 2 && templateName[0] == '"' {
			templateName = templateName[1 : len(templateName)-1]
		}
	}

	users, err := BuildUsers(templateName)
	if err != nil {
		return err
	}

	// Also update tracker limits
	// Tracker is already updated in Refresh(), but we'll ensure limits are synced.
	if c.tracker != nil && c.UserLimits != nil {
		c.tracker.UpdateLimits(c.UserLimits)
	}

	if c.CurrentEngine == "xray" {
		if c.xrayInstance == nil {
			return errors.New("xray instance is nil")
		}

		inboundManagerFeature := c.xrayInstance.GetFeature(xray_inbound.ManagerType())
		if inboundManagerFeature == nil {
			return errors.New("inbound manager not found in xray instance")
		}

		inboundManager := inboundManagerFeature.(xray_inbound.Manager)
		handler, err := inboundManager.GetHandler(context.Background(), "proxy") // We use "proxy" tag
		if err != nil {
			return fmt.Errorf("failed to get 'proxy' inbound handler: %v", err)
		}

		gi, ok := handler.(getInbound)
		if !ok {
			return errors.New("inbound handler does not implement GetInbound")
		}

		pi := gi.GetInbound()
		um, ok := pi.(xray_proxy.UserManager)
		if !ok {
			return fmt.Errorf("inbound %T does not implement UserManager", pi)
		}

		// Currently, Xray's UserManager interface ONLY supports RemoveUser by email.
		// Since we want to ensure exact synchronization, a safe way is to just AddUser.
		// To truly sync, we need to know existing users. For simplicity and robustness,
		// we can try to add the users. AddUser usually just appends or overwrites if exists.
		// Actually, Xray's vless memory validator allows adding a user. Look at our users mapping:
		for _, u := range users {
			var id, email, flow string
			if d, ok := u["id"].(string); ok {
				id = d
			}
			if uuid, ok := u["uuid"].(string); ok {
				id = uuid // vless/vmess
			}
			if pass, ok := u["password"].(string); ok {
				id = pass // trojan/shadowsocks
			}
			if f, ok := u["flow"].(string); ok {
				flow = f
			}
			if e, ok := u["name"].(string); ok {
				email = e
			}
			if email == "" {
				email = id
			}

			if id == "" {
				continue
			}

			// AddUser takes a MemoryUser
			memoryUser := &protocol.MemoryUser{
				Email: email,
			}
			
			// We MUST provide an Account based on the protocol. For vless it's *vless.MemoryAccount
			// Wait, the interface is protocol.Account. We can construct it.
			// But FreeGFW supports vmess, vless, trojan, etc.
			// Constructing the right Account type requires importing them.
			// Let's use reflection to call RemoveUser first for all old users? Too complex without keeping state.
			// For Hot-Reload in Xray: building MemoryAccount is protocol-specific.
			_ = id
			_ = flow
			_ = um
			_ = memoryUser
		}

		// Because constructing MemoryAccount requires protocol specific structs (vless.MemoryAccount, vmess.MemoryAccount, trojan.MemoryAccount),
		// and we do not know the protocol dynamically without checking the type of `pi`.
		// It's cleaner to just fallback to Restart for Xray if we hit this limitation, or implement it fully.
		return errors.New("xray hot reload is complex due to Account types, fallback to restart")
	}

	if c.CurrentEngine == "singbox" {
		if c.instance == nil {
			return errors.New("singbox instance is nil")
		}

		// Penetrate sing-box box.Box to get the specific inbound and call UpdateUsers
		vBox := reflect.ValueOf(c.instance)
		if vBox.Kind() == reflect.Ptr {
			vBox = vBox.Elem()
		}

		inboundManagerField := vBox.FieldByName("inbound")
		if !inboundManagerField.IsValid() || inboundManagerField.IsNil() {
			return errors.New("singbox inbound manager not found")
		}
		
		inboundManagerVal := inboundManagerField.Elem()
		// manager has inboundByTag map[string]adapter.Inbound
		inboundsMapField := inboundManagerVal.FieldByName("inboundByTag")
		if !inboundsMapField.IsValid() {
			return errors.New("inboundByTag field not found in inbound manager")
		}

		inboundsMapPtr := unsafe.Pointer(inboundsMapField.UnsafeAddr())
		inboundsMapVal := reflect.NewAt(inboundsMapField.Type(), inboundsMapPtr).Elem()
		
		var targetInbound reflect.Value
		var selectedVal reflect.Value
		var usersField reflect.Value
		// Find our server inbound.
		for _, key := range inboundsMapVal.MapKeys() {
			val := inboundsMapVal.MapIndex(key)
			
			vInbound := val.Elem()
			if vInbound.Kind() == reflect.Ptr {
				vInbound = vInbound.Elem()
			}
			
			uField := vInbound.FieldByName("users")
			if uField.IsValid() {
				targetInbound = vInbound
				selectedVal = val
				usersField = uField
				break
			}
		}

		if !targetInbound.IsValid() {
			return errors.New("could not find a compatible singbox inbound for hot reload")
		}

		// Try to find UpdateUsers method either directly on inbound or on its service field
		updateMethod := selectedVal.MethodByName("UpdateUsers")
		if !updateMethod.IsValid() {
			updateMethod = targetInbound.Addr().MethodByName("UpdateUsers")
		}

		var serviceVal reflect.Value
		if !updateMethod.IsValid() {
			serviceField := targetInbound.FieldByName("service")
			if serviceField.IsValid() && !serviceField.IsNil() {
				servicePtr := unsafe.Pointer(serviceField.UnsafeAddr())
				serviceVal = reflect.NewAt(serviceField.Type(), servicePtr).Elem()
				updateMethod = serviceVal.MethodByName("UpdateUsers")
			}
		}

		if !updateMethod.IsValid() {
			return errors.New("service or inbound does not have UpdateUsers method")
		}

		// Prepare new users
		// We need to know what protocol users arrays look like. 
		// For VLESS it's []option.VLESSUser 
		// We can use reflection to dynamically create the slice!
		usersField = targetInbound.FieldByName("users")
		usersSliceType := usersField.Type() // e.g. []option.VLESSUser
		elemType := usersSliceType.Elem() // e.g. option.VLESSUser

		newUsersSlice := reflect.MakeSlice(usersSliceType, 0, len(users))
		userUUIDList := make([]string, 0, len(users))
		userFlowList := make([]string, 0, len(users))
		userNameList := make([]string, 0, len(users))
		
		// In VLESS service, T is int (index of user in the array)
		userIndexList := make([]int, 0, len(users))

		for i, u := range users {
			var uuid, flow, name string
			if val, ok := u["uuid"].(string); ok {
				uuid = val
			} else if val, ok := u["password"].(string); ok {
				uuid = val
			}
			if val, ok := u["flow"].(string); ok {
				flow = val
			}
			if val, ok := u["name"].(string); ok {
				name = val
			}
			
			if uuid == "" {
				continue
			}

			// Create a new option.VLESSUser (or VMESSUser dynamically)
			newUserObj := reflect.New(elemType).Elem()
			
			// Set Name
			nameField := newUserObj.FieldByName("Name")
			if nameField.IsValid() && nameField.CanSet() {
				nameField.SetString(name)
			}
			
			// Set UUID/Password
			uuidField := newUserObj.FieldByName("UUID")
			if !uuidField.IsValid() {
				uuidField = newUserObj.FieldByName("Password")
			}
			if uuidField.IsValid() && uuidField.CanSet() {
				uuidField.SetString(uuid)
			}
			
			// Set Flow
			flowField := newUserObj.FieldByName("Flow")
			if flowField.IsValid() && flowField.CanSet() {
				flowField.SetString(flow)
			}

			newUsersSlice = reflect.Append(newUsersSlice, newUserObj)
			userUUIDList = append(userUUIDList, uuid)
			userFlowList = append(userFlowList, flow)
			userIndexList = append(userIndexList, i)
			userNameList = append(userNameList, name)
		}

		// Update users strictly inside Inbound memory
		usersPtr := unsafe.Pointer(usersField.UnsafeAddr())
		usersFieldActual := reflect.NewAt(usersField.Type(), usersPtr).Elem()
		usersFieldActual.Set(newUsersSlice)

		// Call UpdateUsers on service
		methodType := updateMethod.Type()
		var args []reflect.Value
		
		arg0Type := methodType.In(0)
		var arg0 reflect.Value
		if arg0Type.Elem().Kind() == reflect.Int {
			arg0 = reflect.ValueOf(userIndexList)
		} else if arg0Type.Elem().Kind() == reflect.String {
			// e.g. MultiInbound UpdateUsers takes []string (names), []string (passwords)
			arg0 = reflect.ValueOf(userNameList)
		} else {
			return fmt.Errorf("unsupported generic type for UpdateUsers: %v", arg0Type.Kind())
		}

		if methodType.NumIn() == 3 {
			args = []reflect.Value{
				arg0,
				reflect.ValueOf(userUUIDList),
				reflect.ValueOf(userFlowList),
			}
		} else if methodType.NumIn() == 2 {
			args = []reflect.Value{
				arg0,
				reflect.ValueOf(userUUIDList),
			}
		} else {
			return errors.New("UpdateUsers method does not have 2 or 3 arguments")
		}

		results := updateMethod.Call(args)
		if len(results) > 0 {
			if errIf, ok := results[0].Interface().(error); ok && errIf != nil {
				return errIf
			}
		}
		
		log.Println("[HotReload] Sing-box memory users updated successfully using Reflection!")
		return nil
	}

	return errors.New("unsupported core engine")
}
