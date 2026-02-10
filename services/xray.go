package services

import (
	"encoding/json"
	"fmt"
	"time"

	"freegfw/database"
	"freegfw/models"

	xray_core "github.com/xtls/xray-core/core"
	"github.com/xtls/xray-core/features/stats"
)

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
		if name, ok := u["name"]; ok {
			xu["email"] = name
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

	// Build stream settings
	streamSettings := map[string]interface{}{
		"network": "tcp",
	}

	if transport != nil {
		if t, ok := transport["type"].(string); ok {
			streamSettings["network"] = t
		}
	}

	network, _ := streamSettings["network"].(string)

	if network == "xhttp" {
		path := "/xhttp" // Default
		if transport != nil {
			if p, ok := transport["path"].(string); ok {
				path = p
			}
		}
		streamSettings["xhttpSettings"] = map[string]interface{}{
			"path": path,
		}
	}

	// Security settings
	isReality := false
	if reality != nil {
		if enabled, ok := reality["enabled"].(bool); ok && enabled {
			isReality = true
		}
	}

	isTLS := false
	if tlsConfig != nil {
		if enabled, ok := tlsConfig["enabled"].(bool); ok && enabled {
			isTLS = true
		}
	}

	if isReality {
		streamSettings["security"] = "reality"
		rSettings := map[string]interface{}{
			"show":        false,
			"dest":        "www.microsoft.com:443", // Default
			"xver":        0,
			"serverNames": []string{"www.microsoft.com"}, // Default
		}

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
		streamSettings["realitySettings"] = rSettings

	} else if isTLS {
		streamSettings["security"] = "tls"

		ts, err := BuildServerTLS(templateName)
		if err == nil && ts != nil {
			certEntry := map[string]interface{}{
				"certificate": ts["certificate"],
				"key":         ts["key"],
			}
			streamSettings["tlsSettings"] = map[string]interface{}{
				"certificates": []interface{}{certEntry},
			}
		}
	} else {
		streamSettings["security"] = "none"
	}

	// Inbound Config
	inbound := map[string]interface{}{
		"tag":      "proxy",
		"port":     port,
		"protocol": "vless",
		"settings": map[string]interface{}{
			"clients":    xrayUsers,
			"decryption": "none",
		},
		"streamSettings": streamSettings,
	}

	// Add stats and policy
	policy := map[string]interface{}{
		"levels": map[string]interface{}{
			"0": map[string]interface{}{
				"statsUserUplink":   true,
				"statsUserDownlink": true,
			},
		},
		"system": map[string]interface{}{
			"statsInboundUplink":   true,
			"statsInboundDownlink": true,
		},
	}

	stats := map[string]interface{}{}

	config := map[string]interface{}{
		"stats":    stats,
		"policy":   policy,
		"inbounds": []interface{}{inbound},
		"outbounds": []interface{}{
			map[string]interface{}{
				"protocol": "freedom",
			},
		},
	}

	data, _ := json.MarshalIndent(config, "", "  ")
	c.ConfigContent = data
	return nil
}

func monitorXrayLoop(instance *xray_core.Instance) {
	v := instance.GetFeature(stats.ManagerType())
	if v == nil {
		return
	}
	mgr := v.(stats.Manager)

	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	userTraffic := make(map[string]struct{ Up, Down int64 })
	var flushCounter int
	lastStats := make(map[string]int64)
	currentEngine := coreInstance.CurrentEngine

	for range ticker.C {
		if coreInstance.xrayInstance != instance || coreInstance.CurrentEngine != currentEngine {
			return
		}

		var users []models.User
		database.DB.Find(&users)

		var links []models.Link
		database.DB.Where("last_sync_status = ?", "success").Find(&links)

		allNames := []string{}
		for _, u := range users {
			allNames = append(allNames, u.Username)
		}
		for _, l := range links {
			var lUsers []string
			json.Unmarshal(l.Users, &lUsers)
			allNames = append(allNames, lUsers...)
		}
		allNames = append(allNames, "default")

		totalUp := int64(0)
		totalDown := int64(0)

		diffUpTotal := int64(0)
		diffDownTotal := int64(0)

		processedUsers := make(map[string]bool)

		for _, name := range allNames {
			if name == "" || processedUsers[name] {
				continue
			}
			processedUsers[name] = true

			upName := "user>>>" + name + ">>>traffic>>>uplink"
			downName := "user>>>" + name + ">>>traffic>>>downlink"

			cUp := getCounterVal(mgr, upName)
			cDown := getCounterVal(mgr, downName)

			if cUp > 0 || cDown > 0 {
				totalUp += cUp
				totalDown += cDown

				prevUp, okUp := lastStats[upName]
				prevDown, okDown := lastStats[downName]

				if !okUp || !okDown {
					lastStats[upName] = cUp
					lastStats[downName] = cDown
					continue
				}

				dUp := cUp - prevUp
				dDown := cDown - prevDown

				if dUp < 0 {
					dUp = 0
				}
				if dDown < 0 {
					dDown = 0
				}

				diffUpTotal += dUp
				diffDownTotal += dDown

				lastStats[upName] = cUp
				lastStats[downName] = cDown

				uT := userTraffic[name]
				uT.Up += dUp
				uT.Down += dDown
				userTraffic[name] = uT
			}
		}

		if Hub != nil {
			speed := map[string]float64{
				"up":   float64(diffUpTotal) * 8 / 1000000,
				"down": float64(diffDownTotal) * 8 / 1000000,
			}
			Hub.Broadcast("speed", speed)

			total := map[string]int64{
				"up":   totalUp,
				"down": totalDown,
			}
			Hub.Broadcast("traffic", total)

			Hub.Broadcast("connections", map[string]interface{}{"connections": []interface{}{}})
		}

		flushCounter++
		if flushCounter >= 10 {
			for name, traffic := range userTraffic {
				if traffic.Up > 0 || traffic.Down > 0 {
					var user models.User
					if err := database.DB.Where("uuid = ?", name).Or("username = ?", name).First(&user).Error; err == nil {
						database.DB.Model(&user).Updates(map[string]interface{}{
							"upload":   user.Upload + traffic.Up,
							"download": user.Download + traffic.Down,
						})
					}
				}
			}
			userTraffic = make(map[string]struct{ Up, Down int64 })
			flushCounter = 0
		}
	}
}

func getCounterVal(mgr stats.Manager, name string) int64 {
	c := mgr.GetCounter(name)
	if c != nil {
		return c.Value()
	}
	return 0
}
