package services

import (
	"encoding/json"
	"log"
	"strings"
	"time"

	"freegfw/database"
	"freegfw/models"
)

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
	c.UserLimits = make(map[string]uint64)
	for _, u := range users {
		var limit uint64
		if l, ok := u["limit"].(uint64); ok {
			limit = l
		} else if l, ok := u["limit"].(float64); ok {
			limit = uint64(l)
		}

		if limit > 0 {
			if name, ok := u["name"].(string); ok && name != "" {
				c.UserLimits[name] = limit
			}
			if uuid, ok := u["uuid"].(string); ok && uuid != "" {
				c.UserLimits[uuid] = limit
			}
			if pass, ok := u["password"].(string); ok && pass != "" {
				c.UserLimits[pass] = limit
			}
		}
		delete(u, "limit")
	}

	if len(users) == 1 {
		// Set default limit to the only user's limit
		// We need to find the limit from the map we just populated
		// But wait, the loop above populates c.UserLimits. It doesn't modify users array's limit permanently (it deletes it).
		// We can just iterate the map.
		for _, limit := range c.UserLimits {
			if limit > 0 {
				c.UserLimits["__DEFAULT__"] = limit
				break
			}
		}
	}

	tls, _ := BuildServerTLS(templateName)

	// Set timeouts to prevent Goroutine leaks
	server["tcp_fast_open"] = true
	server["udp_timeout"] = "5m"
	
	server["users"] = users
	if tls != nil {
		if serverTls, ok := server["tls"].(map[string]interface{}); ok {
			for k, v := range tls {
				serverTls[k] = v
			}
		}
	}

	outbounds := []map[string]interface{}{}
	var warpEnabledSetting models.Setting
	database.DB.Where("key = ?", "warp_enabled").Limit(1).Find(&warpEnabledSetting)
	
	warpEnabled := false
	if len(warpEnabledSetting.Value) > 0 {
		json.Unmarshal(warpEnabledSetting.Value, &warpEnabled)
	}

	if warpEnabled {
		var warpAccountSetting models.Setting
		var warpAccount *WarpAccount
		
		database.DB.Where("key = ?", "warp_account").Limit(1).Find(&warpAccountSetting)
		if len(warpAccountSetting.Value) > 0 {
			var acc WarpAccount
			if err := json.Unmarshal(warpAccountSetting.Value, &acc); err == nil {
				warpAccount = &acc
			}
		}

		if warpAccount == nil || warpAccount.PrivateKey == "" {
			// Auto register
			log.Println("Auto-registering Cloudflare WARP account...")
			acc, err := RegisterWarp()
			if err != nil {
				log.Println("Failed to register warp:", err)
				// fallback to direct
				outbounds = append(outbounds, map[string]interface{}{"type": "direct", "tag": "direct"})
			} else {
				warpAccount = acc
				accBytes, _ := json.Marshal(acc)
				if warpAccountSetting.Key == "" {
					warpAccountSetting.Key = "warp_account"
				}
				warpAccountSetting.Value = accBytes
				database.DB.Save(&warpAccountSetting)
			}
		}

		if warpAccount != nil && warpAccount.PrivateKey != "" {
			outbounds = append(outbounds, map[string]interface{}{
				"type": "wireguard",
				"tag": "direct", // keep tag direct because inbounds route to "direct"
				"server": "engage.cloudflareclient.com",
				"server_port": 2408,
				"local_address": []string{warpAccount.LocalAddressV4, warpAccount.LocalAddressV6},
				"private_key": warpAccount.PrivateKey,
				"peer_public_key": "bmXOC+F1FxEMF9dyiK2H5/1SUtzH0JuVo51h2wPfgyo=", // Default WARP peer public key
				"reserved": warpAccount.Reserved,
				"mtu": 1280,
				"domain_strategy": "prefer_ipv4",
			})
		}

	} else {
		outbounds = append(outbounds, map[string]interface{}{"type": "direct", "tag": "direct"})
	}

	config := map[string]interface{}{
		"inbounds": []map[string]interface{}{server},
		"outbounds": outbounds,
		"experimental": map[string]interface{}{
			"clash_api": map[string]interface{}{
				"external_controller": "127.0.0.1:0",
			},
		},
	}

	data, _ := json.MarshalIndent(config, "", "  ")
	c.ConfigContent = data

	if c.tracker != nil {
		c.tracker.UpdateLimits(c.UserLimits)
	}

	return nil
}

func monitorSingboxLoop() {



	// Map connection ID to usage {Up, Down}
	connStats := make(map[string]struct{ Up, Down uint64 })
	// User traffic accumulator: InboundUser -> {Up, Down}
	userTraffic := make(map[string]struct{ Up, Down int64 })
	var flushCounter int

	for {
		if coreInstance != nil && coreInstance.CurrentEngine != "singbox" {
			return
		}
		if coreInstance == nil || coreInstance.instance == nil {
			time.Sleep(1 * time.Second)
			continue
		}

		// Capture current instance to detect restarts
		currentInstance := coreInstance.instance

		tm := coreInstance.TrafficManager
		if tm == nil {
			time.Sleep(1 * time.Second)
			continue
		}

		// Initialize last values
		lastUp, lastDown := tm.Total()

		// 1 second interval
		ticker := time.NewTicker(1 * time.Second)

		for range ticker.C {
			if coreInstance.instance != currentInstance {
				break
			}

			// Get current totals
			currUp, currDown := tm.Total()

			// Calculate diff (bytes per second)
			// Using int64 to handle potential resets/overflows gracefully
			diffUp := int64(currUp) - int64(lastUp)
			diffDown := int64(currDown) - int64(lastDown)

			// Handle potential counter resets
			if diffUp < 0 {
				diffUp = 0
			}
			if diffDown < 0 {
				diffDown = 0
			}

			// Update last values
			lastUp = currUp
			lastDown = currDown

			// Speed (Mbps)
			speed := map[string]float64{
				"up":   float64(diffUp) * 8 / 1000000,
				"down": float64(diffDown) * 8 / 1000000,
			}

			if Hub != nil {
				// Broadcast Speed
				Hub.Broadcast("speed", speed)

				// Total Traffic
				total := map[string]int64{
					"up":   int64(currUp),
					"down": int64(currDown),
				}
				Hub.Broadcast("traffic", total)

				// Connections Snapshot
				snapshot := tm.Snapshot()

				// Watchdog for Goroutine/Connection Leaks
				if len(snapshot.Connections) > 8000 {
					log.Printf("[Watchdog] High connection count (%d) detected, possible leak. Restarting engine...\n", len(snapshot.Connections))
					go func() {
						coreInstance.Restart()
					}()
					return // Exit current loop
				}

				Hub.Broadcast("connections", snapshot)

				// Process Per-User Traffic natively
				// Check for single user fallback ONCE per tick
				var defaultUsername string
				var userCount int64
				database.DB.Model(&models.User{}).Count(&userCount)
				if userCount == 1 {
					var u models.User
					if err := database.DB.First(&u).Error; err == nil {
						defaultUsername = u.Username
					}
				}

				currentConns := make(map[string]bool)
				for _, t := range snapshot.Connections {
					tm := t.Metadata()
					if tm == nil {
						continue
					}
					id := tm.ID.String()
					if id == "" {
						continue
					}
					currentConns[id] = true

					cUp := uint64(tm.Upload.Load())
					cDown := uint64(tm.Download.Load())

					prev, exists := connStats[id]
					if !exists {
						prev = struct{ Up, Down uint64 }{0, 0}
					}

					// Calculate delta for this connection
					dUp := int64(cUp) - int64(prev.Up)
					dDown := int64(cDown) - int64(prev.Down)

					if dUp < 0 {
						dUp = 0
					}
					if dDown < 0 {
						dDown = 0
					}

					connStats[id] = struct{ Up, Down uint64 }{cUp, cDown}

					// Accumulate if user is identified
					inboundUser := tm.Metadata.User

					// Fallback if not identified
					if inboundUser == "" && defaultUsername != "" {
						inboundUser = defaultUsername
					}

					if inboundUser != "" {
						uT := userTraffic[inboundUser]
						uT.Up += dUp
						uT.Down += dDown
						userTraffic[inboundUser] = uT
					}
				}

				// Cleanup stale connection stats
				for id := range connStats {
					if !currentConns[id] {
						delete(connStats, id)
					}
				}

				// Periodically flush to user DB
				flushCounter++
				if flushCounter >= 10 { // Every 10 seconds
					for username, traffic := range userTraffic {
						if traffic.Up > 0 || traffic.Down > 0 {
							// Find user by Username or UUID and update traffic
							var user models.User
							if err := database.DB.Where("uuid = ?", username).Or("username = ?", username).First(&user).Error; err == nil {
								database.DB.Model(&user).Updates(map[string]interface{}{
									"upload":   user.Upload + traffic.Up,
									"download": user.Download + traffic.Down,
								})
							}
						}
					}
					// Reset accumulator
					userTraffic = make(map[string]struct{ Up, Down int64 })
					flushCounter = 0
				}
			}
		}

		ticker.Stop()
	}
}
