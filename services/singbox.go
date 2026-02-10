package services

import (
	"encoding/json"
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

func monitorSingboxLoop() {

	type MyConnection struct {
		ID       string                 `json:"id"`
		Metadata map[string]interface{} `json:"metadata"`
		Upload   uint64                 `json:"upload"`
		Download uint64                 `json:"download"`
	}
	type MySnapshot struct {
		Connections []MyConnection `json:"connections"`
	}

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
				Hub.Broadcast("connections", snapshot)

				// Process Per-User Traffic via JSON
				// Bypass strict type checking for internal/unexported fields by marshalling to JSON
				var s MySnapshot
				data, err := json.Marshal(snapshot)
				if err == nil {
					json.Unmarshal(data, &s)

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
					for _, conn := range s.Connections {
						id := conn.ID
						if id == "" {
							continue
						}
						currentConns[id] = true

						cUp := conn.Upload
						cDown := conn.Download

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
						var inboundUser string
						if v, ok := conn.Metadata["inboundUser"].(string); ok {
							inboundUser = v
						} else if v, ok := conn.Metadata["user"].(string); ok {
							inboundUser = v
						} else if v, ok := conn.Metadata["username"].(string); ok {
							inboundUser = v
						} else if v, ok := conn.Metadata["name"].(string); ok {
							inboundUser = v
						}

						// Fallback if not identified
						if inboundUser == "" && defaultUsername != "" {
							inboundUser = defaultUsername
						}

						if inboundUser != "" {
							user := inboundUser
							uT := userTraffic[user]
							uT.Up += dUp
							uT.Down += dDown
							userTraffic[user] = uT
						}
					}

					// Cleanup stale connection stats
					for id := range connStats {
						if !currentConns[id] {
							delete(connStats, id)
						}
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
