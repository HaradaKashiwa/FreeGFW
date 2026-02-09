package services

import (
	"context"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"regexp"
	"strings"
	"time"

	"crypto/ecdh"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"

	"freegfw/database"
	"freegfw/models"
	"freegfw/utils"
)

type TemplateConfig struct {
	Name        string                 `json:"_name"`
	Description string                 `json:"_description"`
	Server      map[string]interface{} `json:"server"`
	Client      map[string]interface{} `json:"client"`
}

type TemplateInfo struct {
	Type        string `json:"type"`
	Name        string `json:"name"`
	Description string `json:"description"`
}

// GetIPv4 fetches public IPv4
func GetIPv4() (string, error) {
	return fetchIP(4)
}

func GetIPv6() (string, error) {
	return fetchIP(6)
}

func fetchIP(family int) (string, error) {
	var network string
	if family == 4 {
		network = "tcp4"
	} else if family == 6 {
		network = "tcp6"
	} else {
		network = "tcp"
	}

	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.DialContext = func(ctx context.Context, _, addr string) (net.Conn, error) {
		dialer := net.Dialer{
			Timeout:   5 * time.Second,
			KeepAlive: 30 * time.Second,
		}
		return dialer.DialContext(ctx, network, addr)
	}

	client := http.Client{
		Timeout:   5 * time.Second,
		Transport: transport,
	}

	resp, err := client.Get("https://cloudflare.com/cdn-cgi/trace")
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	s := string(body)
	re := regexp.MustCompile(`ip=(.+)`)
	match := re.FindStringSubmatch(s)
	if len(match) > 1 {
		ip := strings.TrimSpace(match[1])
		// Save to DB
		key := "ip"
		if family == 6 {
			key = "ipv6"
		}

		// Only valid if using family-specific dial, but simplified here.
		// Logic to emulate updateOrCreate
		var setting models.Setting
		if database.DB.Where("key = ?", key).Limit(1).Find(&setting).RowsAffected == 0 {
			setting = models.Setting{Key: key}
		}
		val, _ := json.Marshal(ip)
		setting.Value = models.JSON(val)
		database.DB.Save(&setting)

		return ip, nil
	}
	return "", errors.New("ip not found")
}

//go:embed templates/*.json
var templateFS embed.FS

func MigrateTemplates() {
	files, err := templateFS.ReadDir("templates")
	if err != nil {
		log.Println("Error reading templates directory:", err)
		return
	}
	for _, f := range files {
		if strings.HasSuffix(f.Name(), ".json") && !f.IsDir() {
			slug := strings.TrimSuffix(f.Name(), ".json")
			var count int64
			database.DB.Model(&models.Template{}).Where("slug = ?", slug).Count(&count)
			if count == 0 {
				content, _ := templateFS.ReadFile("templates/" + f.Name())
				var t TemplateConfig
				if json.Unmarshal(content, &t) == nil {
					database.DB.Create(&models.Template{
						Slug:        slug,
						Name:        t.Name,
						Description: t.Description,
						Content:     models.JSON(content),
					})
					log.Println("Migrated template:", slug)
				}
			}
		}
	}
}

func GetTemplates() ([]TemplateInfo, error) {
	var templates []models.Template
	if err := database.DB.Find(&templates).Error; err != nil {
		return nil, err
	}

	_, err := os.Stat("data/certificate.crt")
	certExists := !os.IsNotExist(err)

	var list []TemplateInfo
	for _, t := range templates {
		var tc TemplateConfig
		if err := json.Unmarshal(t.Content, &tc); err != nil {
			log.Println("Error unmarshal template:", err)
			continue
		}

		if tlsConfig, ok := tc.Server["tls"].(map[string]interface{}); ok {
			if enabled, ok := tlsConfig["enabled"].(bool); ok && enabled {
				isReality := false
				if reality, ok := tlsConfig["reality"].(map[string]interface{}); ok {
					if rEnabled, ok := reality["enabled"].(bool); ok && rEnabled {
						isReality = true
					}
				}
				if !isReality && !certExists {
					continue
				}
			}
		}

		list = append(list, TemplateInfo{
			Type:        t.Slug,
			Name:        t.Name,
			Description: t.Description,
		})
	}
	return list, nil
}

func LoadTemplate(name string) (*TemplateConfig, error) {
	var tmpl models.Template
	if err := database.DB.Where("slug = ?", name).First(&tmpl).Error; err != nil {
		return nil, err
	}
	var t TemplateConfig
	err := json.Unmarshal(tmpl.Content, &t)
	return &t, err
}

func InitTemplate(name string) error {
	tmpl, err := LoadTemplate(name)
	if err != nil {
		return err
	}

	isReality := false
	if tlsConfig, ok := tmpl.Server["tls"].(map[string]interface{}); ok {
		if enabled, ok := tlsConfig["enabled"].(bool); ok && enabled {
			if reality, ok := tlsConfig["reality"].(map[string]interface{}); ok {
				if rEnabled, ok := reality["enabled"].(bool); ok && rEnabled {
					isReality = true
				}
			}

			if isReality {
				// Generate Reality Keys
				curve := ecdh.X25519()
				privKey, err := curve.GenerateKey(rand.Reader)
				if err != nil {
					return err
				}
				pubKey := privKey.PublicKey()

				privStr := base64.RawURLEncoding.EncodeToString(privKey.Bytes())
				pubStr := base64.RawURLEncoding.EncodeToString(pubKey.Bytes())

				// Generate Short ID
				sidBytes := make([]byte, 8)
				rand.Read(sidBytes)
				sidStr := hex.EncodeToString(sidBytes)

				if reality, ok := tlsConfig["reality"].(map[string]interface{}); ok {
					reality["private_key"] = privStr
					reality["short_id"] = []string{sidStr}
					reality["public_key"] = pubStr
				}

				saveSetting("reality_public_key", []byte(fmt.Sprintf("%q", pubStr)))
			} else {
				if _, err := os.Stat("data/certificate.crt"); os.IsNotExist(err) {
					return errors.New("certificate not found, please apply for a certificate first")
				}
			}
		}
	}

	ip, _ := GetIPv4()
	if ip != "" {
		// already saved in GetIPv4
	}
	GetIPv6()

	server := tmpl.Server
	server["listen"] = "::"
	server["listen_port"] = utils.RandomPort()
	if isReality {
		server["listen_port"] = 443
	}
	if _, ok := server["password"]; ok {
		server["password"] = utils.RandomUUID()
	}

	serverBytes, _ := json.Marshal(server)
	saveSetting("server", serverBytes)
	saveSetting("inited", []byte("1"))
	saveSetting("template", []byte(strings.Trim(fmt.Sprintf("%q", name), "\"")))

	// Create default user if no users exist
	var userCount int64
	database.DB.Model(&models.User{}).Count(&userCount)
	if userCount == 0 {
		defaultUser := models.User{
			Username: "default",
			UUID:     utils.RandomUUID(),
		}
		database.DB.Create(&defaultUser)
		log.Println("Created default user during initialization")
	}

	return nil
}

func saveSetting(key string, val []byte) {
	var s models.Setting
	if database.DB.Where("key = ?", key).Limit(1).Find(&s).RowsAffected == 0 {
		s = models.Setting{Key: key}
	}
	s.Value = models.JSON(val)
	database.DB.Save(&s)
}

func BuildUsers(templateName string) ([]map[string]interface{}, error) {
	tmpl, err := LoadTemplate(templateName)
	if err != nil {
		return nil, err
	}

	var users []models.User
	database.DB.Find(&users)

	var links []models.Link
	database.DB.Where("last_sync_status = ?", "success").Find(&links)

	serverType, _ := tmpl.Server["type"].(string)

	flow, _ := tmpl.Server["flow"].(string)

	res := []map[string]interface{}{}

	for _, u := range users {
		userMap := map[string]interface{}{
			"name": u.Username,
		}
		// Customize keys based on protocol type to avoid "unknown field" errors
		switch serverType {
		case "vmess", "vless":
			userMap["uuid"] = u.UUID
			if serverType == "vmess" {
				userMap["alterId"] = 0
			}
			if serverType == "vless" && flow != "" {
				userMap["flow"] = flow
			}
		case "tuic":
			userMap["uuid"] = u.UUID
			userMap["password"] = u.UUID // tuic might use password too? usually uuid. check docs if failure.
		default:
			// shadowsocks, trojan, hysteria2, etc. use "password"
			userMap["password"] = u.UUID
		}
		res = append(res, userMap)
	}

	for _, l := range links {
		var lUsers []string
		json.Unmarshal(l.Users, &lUsers)
		for _, uid := range lUsers {
			userMap := map[string]interface{}{
				"name": uid,
			}
			switch serverType {
			case "vmess", "vless":
				userMap["uuid"] = uid
				if serverType == "vmess" {
					userMap["alterId"] = 0
				}
				if serverType == "vless" && flow != "" {
					userMap["flow"] = flow
				}
			case "tuic":
				userMap["uuid"] = uid
			default:
				userMap["password"] = uid
			}
			res = append(res, userMap)
		}
	}
	return res, nil
}

func BuildServerTLS(templateName string) (map[string]interface{}, error) {
	tmpl, err := LoadTemplate(templateName)
	if err != nil {
		return nil, err
	}

	tlsConfig, ok := tmpl.Server["tls"].(map[string]interface{})
	if ok && tlsConfig["enabled"] == true {
		if reality, ok := tlsConfig["reality"].(map[string]interface{}); ok {
			if rEnabled, ok := reality["enabled"].(bool); ok && rEnabled {
				return nil, nil
			}
		}

		certPath := "data/certificate.crt"
		if _, err := os.Stat(certPath); os.IsNotExist(err) {
			return nil, errors.New("certificate not found")
		}

		var s models.Setting
		database.DB.Where("key = ?", "letsencrypt_domain").Limit(1).Find(&s)
		var ip string
		json.Unmarshal(s.Value, &ip)
		if ip == "" {
			var i models.Setting
			database.DB.Where("key = ?", "ip").First(&i)
			json.Unmarshal(i.Value, &ip)
		}
		if ip == "" {
			ip, _ = GetIPv4()
		}

		certContent, _ := os.ReadFile(certPath)
		keyPath := "data/private.key"
		if _, err := os.Stat(keyPath); os.IsNotExist(err) {
			return nil, errors.New("private key not found")
		}
		keyContent, _ := os.ReadFile(keyPath)

		return map[string]interface{}{
			"enabled":     true,
			"server_name": ip,
			"certificate": strings.Split(string(certContent), "\n"),
			"key":         strings.Split(string(keyContent), "\n"),
		}, nil
	}
	return nil, nil
}
