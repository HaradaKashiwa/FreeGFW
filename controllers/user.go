package controllers

import (
	"freegfw/database"
	"freegfw/models"
	"freegfw/services"
	"freegfw/utils"
	"log"
	"net/http"
	"net/url"

	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/gin-gonic/gin"
	"gopkg.in/yaml.v3"
)

func AddUsers(c *gin.Context) {
	var payload struct {
		Count    int    `json:"count"`
		Title    string `json:"title"`
		Name     string `json:"name"`
		Username string `json:"username"`
	}
	if err := c.ShouldBindJSON(&payload); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	log.Printf("AddUsers payload: %+v", payload)

	title := payload.Title
	if title == "" {
		title = payload.Name
	}
	if title == "" {
		title = payload.Username
	}

	if payload.Count == 0 && title != "" {
		payload.Count = 1
	}

	if payload.Count > 0 && title != "" {
		for i := 0; i < payload.Count; i++ {
			var exists int64
			database.DB.Model(&models.User{}).Where("username = ?", title).Count(&exists)
			if exists > 0 {
				c.JSON(http.StatusBadRequest, gin.H{"error": "Username already exists"})
				return
			}
			user := models.User{
				Username: title,
				UUID:     utils.RandomUUID(),
			}
			if err := database.DB.Create(&user).Error; err != nil {
				log.Println("Failed to create user in DB:", err)
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
		}
		core := services.NewCoreService()
		if err := core.Refresh(); err != nil {
			log.Println("Failed to refresh core:", err)
		}
		if err := core.Restart(); err != nil {
			log.Println("Failed to restart core:", err)
		}
	} else {
		log.Println("Invalid payload: count or title missing")
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid payload: count or title/name/username required"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}

func GetUsers(c *gin.Context) {
	var users []models.User
	database.DB.Find(&users)
	c.JSON(http.StatusOK, users)
}

func DeleteUser(c *gin.Context) {
	id := c.Param("id")
	database.DB.Delete(&models.User{}, id)
	core := services.NewCoreService()
	core.Refresh()
	core.Restart()
	c.JSON(http.StatusOK, gin.H{"success": true})
}

func GetSubscribe(c *gin.Context) {
	uuid := c.Param("uuid")

	var user models.User
	if err := database.DB.Where("uuid = ?", uuid).First(&user).Error; err != nil {
		c.String(http.StatusNotFound, "")
		return
	}

	// Browser detection
	ua := strings.ToLower(c.GetHeader("User-Agent"))
	isBrowser := strings.Contains(ua, "mozilla") &&
		!strings.Contains(ua, "clash") &&
		!strings.Contains(ua, "shadowrocket") &&
		!strings.Contains(ua, "hiddify") &&
		!strings.Contains(ua, "stash") &&
		!strings.Contains(ua, "quantumult")

	if isBrowser {
		var tS models.Setting
		database.DB.Where("key = ?", "title").Limit(1).Find(&tS)
		title := "FreeGFW"
		if len(tS.Value) > 0 {
			json.Unmarshal(tS.Value, &title)
		}

		scheme := "http"
		if c.Request.TLS != nil || c.GetHeader("X-Forwarded-Proto") == "https" {
			scheme = "https"
		}
		host := c.Request.Host
		fullURL := fmt.Sprintf("%s://%s%s", scheme, host, c.Request.RequestURI)
		encodedURL := url.QueryEscape(fullURL)
		encodedName := url.QueryEscape(title)
		subB64 := base64.StdEncoding.EncodeToString([]byte(fullURL + "#" + title))

		html := fmt.Sprintf(`<!DOCTYPE html>
<html>
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>%s</title>
<style>
body { font-family: system-ui, -apple-system, sans-serif; display: flex; flex-direction: column; align-items: center; justify-content: center; height: 100vh; margin: 0; background: #f9f9f9; }
.card { background: white; padding: 2rem; border-radius: 16px; box-shadow: 0 4px 20px rgba(0,0,0,0.08); width: 90%%; max-width: 400px; text-align: center; }
h1 { font-size: 1.5rem; margin-bottom: 2rem; color: #1a1a1a; font-weight: 700; }
.btn { display: block; width: 100%%; padding: 14px 0; margin-bottom: 12px; border-radius: 12px; font-weight: 600; text-decoration: none; color: white; transition: opacity 0.2s, transform 0.1s; box-sizing: border-box; box-shadow: 0 2px 4px rgba(0,0,0,0.1); }
.btn:active { transform: scale(0.98); }
.btn:hover { opacity: 0.9; }
.sr { background: linear-gradient(135deg, #3b82f6, #2563eb); }
.hf { background: linear-gradient(135deg, #8b5cf6, #7c3aed); }
.cl { background: linear-gradient(135deg, #10b981, #059669); }
</style>
</head>
<body>
<div class="card">
<h1>%s</h1>
<a href="sub://%s" class="btn sr">导入到小火箭</a>
<a href="hiddify://import/%s" class="btn hf">导入到Hiddify</a>
<a href="clash://install-config?url=%s&name=%s" class="btn cl">导入到Clash</a>
</div>
</body>
</html>`, title, title, subB64, fullURL, encodedURL, encodedName)

		c.Header("Content-Type", "text/html; charset=utf-8")
		c.String(http.StatusOK, html)
		return
	}

	var s models.Setting
	database.DB.Where("key = ?", "server").Limit(1).Find(&s)
	// If local server is not configured, we might still have remote links, but proceeding with caution.
	var localServer map[string]interface{}
	// Try unmarshal directly or as stringified json
	if len(s.Value) > 0 {
		if err := json.Unmarshal(s.Value, &localServer); err != nil {
			var str string
			if err2 := json.Unmarshal(s.Value, &str); err2 == nil {
				json.Unmarshal([]byte(str), &localServer)
			}
		}
	}

	var ipS models.Setting
	database.DB.Where("key = ?", "ip").Limit(1).Find(&ipS)
	var localIP string
	json.Unmarshal(ipS.Value, &localIP)

	var tS models.Setting
	database.DB.Where("key = ?", "title").Limit(1).Find(&tS)
	title := "FreeGFW"
	if len(tS.Value) > 0 {
		json.Unmarshal(tS.Value, &title)
	}

	if localIP == "" {
		// Fallback to request host if IP is not set
		host := c.Request.Host
		if strings.Contains(host, ":") {
			host = strings.Split(host, ":")[0]
		}
		localIP = host
	}

	var links []string
	var clashProxies []map[string]interface{}
	isClash := strings.Contains(strings.ToLower(c.GetHeader("User-Agent")), "clash")

	generateLink := func(server map[string]interface{}, ip, titleAlias string) string {
		if server == nil {
			return ""
		}
		serverType, _ := server["type"].(string)
		portVal := server["listen_port"]
		port := fmt.Sprintf("%v", portVal)

		// Handles float64 from json
		if f, ok := portVal.(float64); ok {
			port = fmt.Sprintf("%d", int(f))
		}

		if isClash {
			if p := utils.ToClashProxy(server, ip, port, uuid, titleAlias); p != nil {
				clashProxies = append(clashProxies, p)
			}
		}

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
					// If public key is missing in config, try fallback to DB setting?
					// Only for local node. Remote nodes depend on synced config.
					if realityPub == "" {
						var pkS models.Setting
						database.DB.Where("key = ?", "reality_public_key").Limit(1).Find(&pkS)
						json.Unmarshal(pkS.Value, &realityPub)
					}

					if sids, ok := reality["short_id"].([]interface{}); ok && len(sids) > 0 {
						if sid, ok := sids[0].(string); ok {
							realitySid = sid
						}
					}
				}
			}
		}

		// Handle IPv6 formatting for URI authority
		hostname := ip
		if strings.Contains(ip, ":") && !strings.HasPrefix(ip, "[") {
			hostname = "[" + ip + "]"
		}

		var link string
		switch serverType {
		case "vmess":
			v := map[string]interface{}{
				"v":    "2",
				"ps":   titleAlias,
				"add":  ip,
				"port": port,
				"id":   uuid,
				"aid":  "0",
				"scy":  "auto",
				"net":  "tcp",
				"type": "none",
				"host": "",
				"path": "",
				"tls":  "",
			}
			if isTLS {
				v["tls"] = "tls"
				v["sni"] = serverName
			}
			b, _ := json.Marshal(v)
			link = "vmess://" + base64.StdEncoding.EncodeToString(b)

		case "vless":
			// vless://uuid@ip:port?security=reality&sni=...&fp=...&type=tcp&headerType=none#title
			flowVal, _ := server["flow"].(string)
			params := []string{}
			if isTLS {
				if isReality {
					params = append(params, "security=reality")
					params = append(params, "sni="+serverName)
					if realityPub != "" {
						params = append(params, "pbk="+realityPub)
					}
					if realitySid != "" {
						params = append(params, "sid="+realitySid)
					}
					params = append(params, "fp=chrome")
				} else {
					params = append(params, "security=tls")
					params = append(params, "sni="+serverName)
				}
				if flowVal != "" {
					params = append(params, "flow="+flowVal)
				}
			} else {
				params = append(params, "security=none")
			}
			params = append(params, "type=tcp")

			link = fmt.Sprintf("vless://%s@%s:%s?%s#%s", uuid, hostname, port, strings.Join(params, "&"), titleAlias)

		case "trojan":
			params := []string{}
			if isTLS {
				params = append(params, "security=tls")
				params = append(params, "sni="+serverName)
			}
			link = fmt.Sprintf("trojan://%s@%s:%s?%s#%s", uuid, hostname, port, strings.Join(params, "&"), titleAlias)

		case "shadowsocks":
			method, _ := server["method"].(string)
			userInfo := fmt.Sprintf("%s:%s", method, uuid) // uuid used as password
			base64User := base64.URLEncoding.EncodeToString([]byte(userInfo))
			link = fmt.Sprintf("ss://%s@%s:%s#%s", base64User, hostname, port, titleAlias)

		case "tuic":
			// tuic implementation
		case "hysteria2":
			params := []string{}
			if isTLS {
				params = append(params, "sni="+serverName)
				params = append(params, "alpn=h3")
			}
			link = fmt.Sprintf("hy2://%s@%s:%s?%s#%s", uuid, hostname, port, strings.Join(params, "&"), titleAlias)
		}
		return link
	}

	// Add local node if configured
	if localServer != nil {
		if l := generateLink(localServer, localIP, title); l != "" {
			links = append(links, l)
		}
	}

	// Fetch remote links
	var remoteLinks []models.Link
	// We only care about links that have successfully synced
	database.DB.Where("last_sync_status = ?", "success").Find(&remoteLinks)

	for _, rl := range remoteLinks {
		var remoteServer map[string]interface{}
		if err := json.Unmarshal(rl.Server, &remoteServer); err == nil {
			ip := ""
			if rl.IP != nil {
				ip = *rl.IP
			}

			itemTitle := ""
			if t, ok := remoteServer["title"].(string); ok && t != "" {
				itemTitle = t
			} else {
				itemTitle = title
				if ip != "" && ip != localIP {
					itemTitle = fmt.Sprintf("%s (%s)", title, ip)
				}
			}

			if l := generateLink(remoteServer, ip, itemTitle); l != "" {
				links = append(links, l)
			}
		}
	}

	if isClash {
		clashConfig := utils.GenClashConfig(clashProxies)
		y, err := yaml.Marshal(clashConfig)
		if err == nil {
			c.String(http.StatusOK, string(y))
			return
		}
		log.Println("Failed to marshal clash config:", err)
	}

	c.String(http.StatusOK, base64.StdEncoding.EncodeToString([]byte(strings.Join(links, "\n"))))
}
