package services

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"
	"sync"
	"time"

	box "github.com/sagernet/sing-box"
	_ "github.com/sagernet/sing-box/include"
	"github.com/sagernet/sing-box/option"

	"github.com/sagernet/sing-box/experimental/clashapi/trafficontrol"

	xray_core "github.com/xtls/xray-core/core"
	"github.com/xtls/xray-core/infra/conf/serial"
	_ "github.com/xtls/xray-core/main/distro/all"

	"freegfw/database"
	"freegfw/models"
)

type CoreService struct {
	ConfigContent  []byte
	instance       *box.Box            // Singbox instance
	xrayInstance   *xray_core.Instance // Xray instance
	cancel         context.CancelFunc
	TrafficManager *trafficontrol.Manager
	CurrentEngine  string // "singbox" or "xray"
}

var (
	coreInstance *CoreService
	coreOnce     sync.Once
)

func NewCoreService() *CoreService {
	coreOnce.Do(func() {
		coreInstance = &CoreService{
			CurrentEngine: "singbox",
		}
	})
	return coreInstance
}

func (c *CoreService) Refresh() error {
	var s models.Setting
	database.DB.Where("key = ?", "server").Limit(1).Find(&s)

	var server map[string]interface{}
	if len(s.Value) > 0 {
		if err := json.Unmarshal(s.Value, &server); err != nil {
			var str string
			if err2 := json.Unmarshal(s.Value, &str); err2 == nil {
				json.Unmarshal([]byte(str), &server)
			}
		}
	}

	var t models.Setting
	database.DB.Where("key = ?", "template").Limit(1).Find(&t)
	var templateName string
	if len(t.Value) > 0 {
		if err := json.Unmarshal(t.Value, &templateName); err != nil {
			templateName = string(t.Value)
		}
	}

	if templateName == "" {
		return nil
	}

	// Determine Engine
	tmpl, err := LoadTemplate(templateName)
	if err == nil {
		coreName, _ := tmpl.Core.(string)
		if coreName == "xray" {
			c.CurrentEngine = "xray"
			return c.refreshXray(server, templateName)
		}
	}

	c.CurrentEngine = "singbox"
	return c.refreshSingbox(server, templateName)
}

func (c *CoreService) refreshXray(server map[string]interface{}, templateName string) error {
	users, _ := BuildUsers(templateName)
	// Singbox users: uuid, flow. Xray users: id, flow.
	xrayUsers := []map[string]interface{}{}
	for _, u := range users {
		xu := map[string]interface{}{}
		if id, ok := u["uuid"]; ok {
			xu["id"] = id
		} else if pass, ok := u["password"]; ok {
			xu["id"] = pass
		}
		if flow, ok := u["flow"]; ok {
			xu["flow"] = flow
		}
		xrayUsers = append(xrayUsers, xu)
	}

	tlsConfig, _ := server["tls"].(map[string]interface{})
	reality, _ := tlsConfig["reality"].(map[string]interface{})
	transport, _ := server["transport"].(map[string]interface{})

	port := 443
	if p, ok := server["listen_port"].(float64); ok {
		port = int(p)
	}

	// Build Xray Config (Simplified for JSON loader)
	// Note: Xray's JSON format expected by serial.LoadJSONConfig is standard Xray config
	inbound := map[string]interface{}{
		"port":     port,
		"protocol": "vless",
		"settings": map[string]interface{}{
			"clients":    xrayUsers,
			"decryption": "none",
		},
		"streamSettings": map[string]interface{}{
			"network": "xhttp",
			"xhttpSettings": map[string]interface{}{
				"path": "/xhttp", // Default
			},
			"security": "reality",
			"realitySettings": map[string]interface{}{
				"show":        false,
				"dest":        "www.microsoft.com:443",
				"xver":        0,
				"serverNames": []string{"www.microsoft.com"},
				"privateKey":  "",
				"shortIds":    []string{""},
			},
		},
	}

	// Updates from config
	if transport != nil {
		if path, ok := transport["path"].(string); ok {
			inbound["streamSettings"].(map[string]interface{})["xhttpSettings"].(map[string]interface{})["path"] = path
		}
	}

	if reality != nil {
		rSettings := inbound["streamSettings"].(map[string]interface{})["realitySettings"].(map[string]interface{})
		if pk, ok := reality["private_key"].(string); ok {
			rSettings["privateKey"] = pk
		}
		if sids, ok := reality["short_id"].([]interface{}); ok {
			newSids := []string{}
			for _, sid := range sids {
				if s, ok := sid.(string); ok {
					newSids = append(newSids, s)
				}
			}
			rSettings["shortIds"] = newSids
		}
		if sni, ok := tlsConfig["server_name"].(string); ok {
			rSettings["serverNames"] = []string{sni}
		}
		if handshake, ok := reality["handshake"].(map[string]interface{}); ok {
			server := "www.microsoft.com"
			port := "443"
			if s, ok := handshake["server"].(string); ok {
				server = s
			}
			if p, ok := handshake["server_port"].(float64); ok {
				port = fmt.Sprintf("%d", int(p))
			}
			rSettings["dest"] = server + ":" + port
		}
	}

	config := map[string]interface{}{
		"inbounds": []interface{}{inbound},
		"outbounds": []interface{}{
			map[string]interface{}{
				"protocol": "freedom",
			},
		},
	}

	data, _ := json.MarshalIndent(config, "", "  ")
	c.ConfigContent = data
	// Also write to file for debugging if needed
	os.WriteFile("data/config.json", data, 0644)
	return nil
}

func (c *CoreService) refreshSingbox(server map[string]interface{}, templateName string) error {
	// Existing Logic
	delete(server, "flow")

	if tlsConfig, ok := server["tls"].(map[string]interface{}); ok {
		if reality, ok := tlsConfig["reality"].(map[string]interface{}); ok {
			if pk, ok := reality["private_key"].(string); ok {
				reality["private_key"] = strings.TrimRight(pk, "=")
			}
			delete(reality, "public_key")
		}
	}

	users, _ := BuildUsers(templateName)
	tls, _ := BuildServerTLS(templateName)

	server["users"] = users
	if tls != nil {
		if serverTls, ok := server["tls"].(map[string]interface{}); ok {
			for k, v := range tls {
				serverTls[k] = v
			}
		}
	}

	config := map[string]interface{}{
		"inbounds": []map[string]interface{}{server},
		"outbounds": []map[string]interface{}{
			{"type": "direct", "tag": "direct"},
		},
		"experimental": map[string]interface{}{
			"clash_api": map[string]interface{}{
				"external_controller": "127.0.0.1:0",
			},
		},
	}

	data, _ := json.MarshalIndent(config, "", "  ")
	c.ConfigContent = data
	return nil
}

func (c *CoreService) IsRunning() bool {
	if c.CurrentEngine == "xray" {
		return c.xrayInstance != nil
	}
	return c.instance != nil
}

func (c *CoreService) Kill() error {
	if c.cancel != nil {
		c.cancel()
		c.cancel = nil
	}
	// Kill Singbox
	if c.instance != nil {
		c.instance.Close()
		c.instance = nil
	}
	// Kill Xray
	if c.xrayInstance != nil {
		c.xrayInstance.Close()
		c.xrayInstance = nil
	}

	time.Sleep(1 * time.Second)
	return nil
}

func (c *CoreService) Start() error {
	log.Println("start engine:", c.CurrentEngine)
	if len(c.ConfigContent) == 0 {
		return nil
	}
	c.Kill()

	if c.CurrentEngine == "xray" {
		// Parse JSON config to Xray Core Config
		coreConfig, err := serial.LoadJSONConfig(bytes.NewReader(c.ConfigContent))
		if err != nil {
			log.Println("Failed to parse xray config (json):", err)
			return err
		}

		instance, err := xray_core.New(coreConfig)
		if err != nil {
			log.Println("Failed to create xray instance:", err)
			return err
		}

		if err := instance.Start(); err != nil {
			log.Println("Failed to start xray:", err)
			return err
		}

		c.xrayInstance = instance
		c.TrafficManager = nil // Xray internal traffic tracking is different
		return nil
	}

	// Singbox Start
	var options option.Options
	if err := json.Unmarshal(c.ConfigContent, &options); err != nil {
		log.Println("Failed to parse singbox config:", err)
		return err
	}

	ctx, cancel := context.WithCancel(context.Background())
	c.cancel = cancel

	instance, err := box.New(box.Options{
		Context: ctx,
		Options: options,
	})
	if err != nil {
		cancel()
		log.Println("Failed to create singbox instance:", err)
		return err
	}
	c.instance = instance

	c.TrafficManager = trafficontrol.NewManager()
	tracker := NewStatisticsTracker(c.TrafficManager, instance.Outbound())
	instance.Router().AppendTracker(tracker)

	if err := instance.Start(); err != nil {
		c.Kill()
		log.Println("Failed to start singbox:", err)
		return err
	}

	return nil
}

func (c *CoreService) Restart() error {
	return c.Start()
}
