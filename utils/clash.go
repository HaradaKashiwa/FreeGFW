package utils

import (
	"fmt"
)

func ToClashProxy(server map[string]interface{}, ip, port, uuid, title string) map[string]interface{} {
	serverType, _ := server["type"].(string)

	proxy := map[string]interface{}{
		"name":   title,
		"server": ip,
		"port":   port, // Clash expects int or string? Usually int.
	}

	// Helper to handle port as int
	// In the original code port was cast to string for links.
	// Clash yaml usually prefers int but string works often. Let's try to keep it as int if possible.
	// But the incoming 'port' arg is string.
	// We can leave it as is or convert.

	tlsConfig, _ := server["tls"].(map[string]interface{})
	isTLS := false
	serverName := ""
	isReality := false
	realityPub := ""
	realitySid := ""

	if tlsConfig != nil && tlsConfig["enabled"] == true {
		isTLS = true
		serverName, _ = tlsConfig["server_name"].(string)
		if serverName == "" {
			serverName = ip
		}

		if reality, ok := tlsConfig["reality"].(map[string]interface{}); ok {
			if rEnabled, ok := reality["enabled"].(bool); ok && rEnabled {
				isReality = true
				if pk, ok := reality["public_key"].(string); ok {
					realityPub = pk
				}
				if sids, ok := reality["short_id"].([]interface{}); ok && len(sids) > 0 {
					if sid, ok := sids[0].(string); ok {
						realitySid = sid
					}
				}
			}
		}
	}

	switch serverType {
	case "vmess":
		proxy["type"] = "vmess"
		proxy["uuid"] = uuid
		proxy["alterId"] = 0
		proxy["cipher"] = "auto"

		if isTLS {
			proxy["tls"] = true
			if serverName != "" {
				proxy["servername"] = serverName
			}
		}
		// Assuming network is TCP as per existing logic (vmess case hardcodes net: tcp)
		// But in original code: "net": "tcp", "type": "none"
		proxy["network"] = "tcp"

	case "vless":
		proxy["type"] = "vless"
		proxy["uuid"] = uuid
		if flow, ok := server["flow"].(string); ok && flow != "" {
			proxy["flow"] = flow
		}

		if isTLS {
			proxy["tls"] = true
			proxy["servername"] = serverName
			if isReality {
				proxy["reality-opts"] = map[string]interface{}{
					"public-key": realityPub,
					"short-id":   realitySid,
				}
				proxy["client-fingerprint"] = "chrome"
			}
		}
		proxy["network"] = "tcp"

	case "trojan":
		proxy["type"] = "trojan"
		proxy["password"] = uuid
		if isTLS {
			proxy["tls"] = true // Trojan is always TLS but Clash config has 'tls: true' explicit? Usually implies it.
			// Actually standard Trojan implies TLS.
			// Checking clash docs: type: trojan, server, port, password, udp(opt), sni(opt), alpn(opt), skip-cert-verify(opt)
			proxy["sni"] = serverName
		}

	case "shadowsocks":
		proxy["type"] = "ss"
		proxy["cipher"], _ = server["method"].(string)
		proxy["password"] = uuid

	case "hysteria2":
		proxy["type"] = "hysteria2"
		proxy["password"] = uuid
		if isTLS {
			proxy["sni"] = serverName
			proxy["alpn"] = []string{"h3"}
		}

	case "tuic":
		proxy["type"] = "tuic"
		proxy["uuid"] = uuid
		proxy["password"] = uuid // TUIC uses uuid usually
		if isTLS {
			proxy["server-name"] = serverName
			proxy["alpn"] = []string{"h3"}
		}

	default:
		return nil
	}

	// Parse port to int if possible for cleaner YAML
	var portInt int
	if _, err := fmt.Sscanf(port, "%d", &portInt); err == nil {
		proxy["port"] = portInt
	} else {
		// fallback
		proxy["port"] = port
	}

	return proxy
}

func GenClashConfig(proxies []map[string]interface{}) map[string]interface{} {
	proxyNames := make([]string, 0, len(proxies))
	for _, p := range proxies {
		if name, ok := p["name"].(string); ok {
			proxyNames = append(proxyNames, name)
		}
	}

	// Define Proxy Groups
	groups := []map[string]interface{}{
		{
			"name":    "Proxy",
			"type":    "select",
			"proxies": append([]string{"Auto"}, proxyNames...),
		},
		{
			"name":      "Auto",
			"type":      "url-test",
			"url":       "http://www.gstatic.com/generate_204",
			"interval":  300,
			"tolerance": 50,
			"proxies":   proxyNames,
		},
	}

	// Define Rules
	rules := []string{
		"GEOIP,LAN,DIRECT",
		"GEOIP,CN,DIRECT",
		"MATCH,Proxy",
	}

	// Construct the full configuration
	return map[string]interface{}{
		"port":                7890,
		"socks-port":          7891,
		"allow-lan":           true,
		"mode":                "Rule",
		"log-level":           "info",
		"external-controller": "127.0.0.1:9090",
		"proxies":             proxies,
		"proxy-groups":        groups,
		"rules":               rules,
	}
}
