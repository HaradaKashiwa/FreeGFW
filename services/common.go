package services

import (
	"encoding/json"
	"fmt"
	"freegfw/database"
	"freegfw/models"
	"os"
)

var RestartChan = make(chan struct{})

func GetMyLink(code string) (string, error) {
	var s models.Setting
	database.DB.Where("key = ?", "ip").Limit(1).Find(&s)
	var ip string
	json.Unmarshal(s.Value, &ip)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	protocol := "http"
	if _, err := os.Stat("data/certificate.crt"); err == nil {
		protocol = "https"
		var d models.Setting
		database.DB.Where("key = ?", "letsencrypt_domain").Limit(1).Find(&d)
		var domain string
		json.Unmarshal(d.Value, &domain)
		if domain != "" {
			ip = domain
		}
	}

	return fmt.Sprintf("%s://%s:%s/link/%s", protocol, ip, port, code), nil
}
