package services

import (
	"encoding/json"
	"fmt"
	"sync/atomic"
	"time"

	"freegfw/database"
	"freegfw/models"

	"log"
	"sync"

	xray_core "github.com/xtls/xray-core/core"
)

// XrayTrafficStats holds atomic counters for user traffic
type XrayTrafficStats struct {
	Up   int64
	Down int64
}

// XrayUserTraffic maps user email/ID to their traffic stats
var (
	XrayUserTraffic      = make(map[string]*XrayTrafficStats)
	XrayUserTrafficMutex sync.RWMutex
)

func GetXrayUserStats(user string) *XrayTrafficStats {
	XrayUserTrafficMutex.RLock()
	stats, ok := XrayUserTraffic[user]
	XrayUserTrafficMutex.RUnlock()
	if ok {
		return stats
	}

	XrayUserTrafficMutex.Lock()
	defer XrayUserTrafficMutex.Unlock()
	// Double check
	if stats, ok = XrayUserTraffic[user]; ok {
		return stats
	}
	stats = &XrayTrafficStats{}
	XrayUserTraffic[user] = stats
	return stats
}

func (c *CoreService) refreshXray(server map[string]interface{}, templateName string) error {
	users, _ := BuildUsers(templateName)
	// Singbox users: uuid, flow. Xray users: id, flow.
	xrayUsers := []map[string]interface{}{}
	c.UserLimits = make(map[string]uint64) // Initialize limits

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

		// Capture limits
		var limit uint64
		if l, ok := u["limit"]; ok {
			if v, ok := l.(uint64); ok {
				limit = v
			} else if v, ok := l.(float64); ok {
				limit = uint64(v)
			}
		}
		if limit > 0 {
			if name, ok := u["name"].(string); ok {
				c.UserLimits[name] = limit
			}
			if id, ok := u["uuid"].(string); ok {
				c.UserLimits[id] = limit
			}
		}
	}
	log.Printf("[XrayConfig] Configured %d users for Xray", len(xrayUsers))

	// Update tracker limits if exists (might be nil now, initialized in Start)
	if c.tracker != nil {
		c.tracker.UpdateLimits(c.UserLimits)
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
		"log": map[string]interface{}{
			"loglevel": "info",
		},
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
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	// Store last cumulative values to calculate diff
	// user -> {Up, Down}
	lastStats := make(map[string]struct{ Up, Down int64 })
	userTraffic := make(map[string]struct{ Up, Down int64 })
	var flushCounter int
	currentEngine := coreInstance.CurrentEngine

	for range ticker.C {
		if coreInstance.xrayInstance != instance || coreInstance.CurrentEngine != currentEngine {
			return
		}

		totalUp := int64(0)
		totalDown := int64(0)
		diffUpTotal := int64(0)
		diffDownTotal := int64(0)

		// Iterate XrayUserTraffic
		XrayUserTrafficMutex.RLock()

		// Create a set of all users to check (from UserLimits + XrayUserTraffic)
		usersToCheck := make(map[string]bool)
		for u := range XrayUserTraffic {
			usersToCheck[u] = true
		}
		if coreInstance != nil && coreInstance.UserLimits != nil {
			for u := range coreInstance.UserLimits {
				usersToCheck[u] = true
			}
		}

		for user := range usersToCheck {
			cUp := int64(0)
			cDown := int64(0)

			// 1. Get from Custom Dispatcher (XrayUserTraffic)
			if stats, ok := XrayUserTraffic[user]; ok {
				cUp += atomic.LoadInt64(&stats.Up)
				cDown += atomic.LoadInt64(&stats.Down)
			}

			// 2. Get from Internal Stats Manager
			if coreInstance != nil && coreInstance.XrayStats != nil {
				// Pattern: user>>>[email]>>>traffic>>>uplink
				if upCounter := coreInstance.XrayStats.GetCounter("user>>>" + user + ">>>traffic>>>uplink"); upCounter != nil {
					cUp += upCounter.Value()
				}
				if downCounter := coreInstance.XrayStats.GetCounter("user>>>" + user + ">>>traffic>>>downlink"); downCounter != nil {
					cDown += downCounter.Value()
				}
			}

			if cUp > 0 || cDown > 0 {
				totalUp += cUp
				totalDown += cDown

				prev, ok := lastStats[user]
				if !ok {
					lastStats[user] = struct{ Up, Down int64 }{cUp, cDown}
					continue
				}

				dUp := cUp - prev.Up
				dDown := cDown - prev.Down
				if dUp < 0 {
					dUp = 0
				}
				if dDown < 0 {
					dDown = 0
				}

				diffUpTotal += dUp
				diffDownTotal += dDown

				lastStats[user] = struct{ Up, Down int64 }{cUp, cDown}

				uT := userTraffic[user]
				uT.Up += dUp
				uT.Down += dDown
				userTraffic[user] = uT
			}
		}
		XrayUserTrafficMutex.RUnlock()

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
