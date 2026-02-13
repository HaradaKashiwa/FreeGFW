package services

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"testing"
	"time"

	xray_core "github.com/xtls/xray-core/core"
	"github.com/xtls/xray-core/infra/conf/serial"
	"golang.org/x/net/proxy"
)

func TestXrayDispatcher_Integration_VLESS_TCP(t *testing.T) {
	// 1. Setup Server (CoreService with XrayDispatcher)
	serverPort := 20001
	userEmail := "integration_test@example.com"
	userUUID := "00000000-0000-0000-0000-000000000001"

	c := NewCoreService()
	c.CurrentEngine = "xray"

	// Server Config (VLESS TCP)
	serverConfig := map[string]interface{}{
		"log": map[string]interface{}{
			"loglevel": "debug",
		},
		"inbounds": []interface{}{
			map[string]interface{}{
				"port":     serverPort,
				"protocol": "vless",
				"settings": map[string]interface{}{
					"clients": []interface{}{
						map[string]interface{}{
							"id":    userUUID,
							"email": userEmail,
						},
					},
					"decryption": "none",
				},
				"streamSettings": map[string]interface{}{
					"network": "tcp",
				},
			},
		},
		"outbounds": []interface{}{
			map[string]interface{}{
				"protocol": "freedom",
			},
		},
	}
	serverJSON, _ := json.Marshal(serverConfig)
	c.ConfigContent = serverJSON
	c.UserLimits = make(map[string]uint64)

	// Start Server
	err := c.Start()
	if err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}
	defer c.Kill()

	// Wait for server start
	time.Sleep(1 * time.Second)

	// 2. Setup Client (Direct Xray Instance)
	clientSocksPort := 20002
	clientConfig := map[string]interface{}{
		"log": map[string]interface{}{
			"loglevel": "debug",
		},
		"inbounds": []interface{}{
			map[string]interface{}{
				"port":     clientSocksPort,
				"protocol": "socks",
				"settings": map[string]interface{}{
					"auth": "noauth",
				},
			},
		},
		"outbounds": []interface{}{
			map[string]interface{}{
				"protocol": "vless",
				"settings": map[string]interface{}{
					"vnext": []interface{}{
						map[string]interface{}{
							"address": "127.0.0.1",
							"port":    serverPort,
							"users": []interface{}{
								map[string]interface{}{
									"id":         userUUID,
									"encryption": "none",
								},
							},
						},
					},
				},
				"streamSettings": map[string]interface{}{
					"network": "tcp",
				},
			},
		},
	}
	clientJSON, _ := json.Marshal(clientConfig)

	clientCoreConfig, err := serial.LoadJSONConfig(bytes.NewReader(clientJSON))
	if err != nil {
		t.Fatalf("Failed to parse client config: %v", err)
	}
	clientInstance, err := xray_core.New(clientCoreConfig)
	if err != nil {
		t.Fatalf("Failed to create client instance: %v", err)
	}
	if err := clientInstance.Start(); err != nil {
		t.Fatalf("Failed to start client: %v", err)
	}
	defer clientInstance.Close()

	// 3. Generate Traffic (HTTP via SOCKS Proxy)
	proxyURL, _ := url.Parse(fmt.Sprintf("socks5://127.0.0.1:%d", clientSocksPort))
	dialer, err := proxy.FromURL(proxyURL, proxy.Direct)
	if err != nil {
		t.Fatalf("Failed to create proxy dialer: %v", err)
	}

	transport := &http.Transport{
		Dial: dialer.Dial,
	}
	client := &http.Client{
		Transport: transport,
		Timeout:   5 * time.Second,
	}

	// Request something simple (e.g., self or google)
	// We can't guarantee internet access, so let's try to connect to a dummy listener or just fail and expect Up traffic.
	// Actually, "freedom" outbound on server will try to connect.
	// Let's rely on the fact that even if connection fails at destination, some handshake traffic occurs.
	// Or start a local HTTP server to echo.

	echoPort := 20003
	go func() {
		http.ListenAndServe(fmt.Sprintf(":%d", echoPort), http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte("hello"))
		}))
	}()
	time.Sleep(500 * time.Millisecond)

	resp, err := client.Get(fmt.Sprintf("http://127.0.0.1:%d", echoPort))
	if err != nil {
		t.Logf("Request failed (expected if network issue): %v", err)
	} else {
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		t.Logf("Response: %s", string(body))
	}

	// 4. Verify Stats
	time.Sleep(1 * time.Second) // Wait for stats sync

	stats := GetXrayUserStats(userEmail)
	t.Logf("Stats for %s: Up=%d, Down=%d", userEmail, stats.Up, stats.Down)

	if stats.Up == 0 && stats.Down == 0 {
		t.Errorf("Traffic stats are zero! Dispatcher/User Context fail.")
	} else {
		t.Logf("Success! Traffic detected.")
	}
}
