package controllers

import (
	"encoding/json"
	"freegfw/database"
	"freegfw/models"
	"freegfw/services"
	"net/http"

	"github.com/gin-gonic/gin"
)

func GetConfigs(c *gin.Context) {
	var serverSettings models.Setting
	database.DB.Where("key = ?", "server").Limit(1).Find(&serverSettings)

	var titleSettings models.Setting
	database.DB.Where("key = ?", "title").Limit(1).Find(&titleSettings)
	title := "FreeGFW"
	if len(titleSettings.Value) > 0 {
		json.Unmarshal(titleSettings.Value, &title)
	}

	var ipSettings models.Setting
	database.DB.Where("key = ?", "ip").Limit(1).Find(&ipSettings)
	var ip string
	if len(ipSettings.Value) > 0 {
		json.Unmarshal(ipSettings.Value, &ip)
	}

	var ipv6Settings models.Setting
	database.DB.Where("key = ?", "ipv6").Limit(1).Find(&ipv6Settings)
	var ipv6 string
	if len(ipv6Settings.Value) > 0 {
		json.Unmarshal(ipv6Settings.Value, &ipv6)
	}

	var passSettings models.Setting
	database.DB.Where("key = ?", "password").Limit(1).Find(&passSettings)
	hasPassword := len(passSettings.Value) > 0

	var sslSettings models.Setting
	database.DB.Where("key = ?", "letsencrypt_domain").Limit(1).Find(&sslSettings)
	ssl := len(sslSettings.Value) > 0

	core := services.NewCoreService()

	// serverSettings.Value is RawMessage (text/string in DB), we might need to unmarshal to object if frontend expects object
	var serverObj interface{}
	if len(serverSettings.Value) > 0 {
		json.Unmarshal(serverSettings.Value, &serverObj)
	}

	c.JSON(http.StatusOK, gin.H{
		"server":       serverObj,
		"title":        title,
		"inited":       len(serverSettings.Value) > 0,
		"running":      core.IsRunning(),
		"ip":           ip,
		"ipv6":         ipv6,
		"has_password": hasPassword,
		"ssl":          ssl,
	})
}

func UpdateConfig(c *gin.Context) {
	var payload map[string]interface{}
	if err := c.ShouldBindJSON(&payload); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	allowed := []string{"username", "password", "title"}
	for _, key := range allowed {
		if val, ok := payload[key]; ok {
			jsonVal, _ := json.Marshal(val) // Handle null/empty logic
			var s models.Setting
			if database.DB.Where("key = ?", key).Limit(1).Find(&s).RowsAffected == 0 {
				s = models.Setting{Key: key}
			}
			if val == nil || val == "" {
				s.Value = nil
			} else {
				s.Value = models.JSON(jsonVal)
			}
			database.DB.Save(&s)
		}
	}

	c.JSON(http.StatusOK, gin.H{"success": true})
}

func ReloadConfig(c *gin.Context) {
	core := services.NewCoreService()
	core.Refresh()
	core.Start()
	c.JSON(http.StatusOK, gin.H{"success": true})
}

func ResetConfig(c *gin.Context) {
	// Delete settings except letsencrypt
	database.DB.Where("key NOT IN ?", []string{"letsencrypt_domain", "letsencrypt_email", "letsencrypt_updated_at"}).Delete(&models.Setting{})
	// Truncate Users
	database.DB.Exec("DELETE FROM users") // SQLite doesn't have TRUNCATE

	core := services.NewCoreService()
	core.Kill()
	c.JSON(http.StatusOK, gin.H{"success": true})
}

func SetTitle(c *gin.Context) {
	var payload struct {
		Title string `json:"title"`
	}
	if err := c.ShouldBindJSON(&payload); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var s models.Setting
	if database.DB.Where("key = ?", "title").Limit(1).Find(&s).RowsAffected == 0 {
		s = models.Setting{Key: "title"}
	}
	val, _ := json.Marshal(payload.Title)
	s.Value = models.JSON(val)
	database.DB.Save(&s)

	c.JSON(http.StatusOK, gin.H{"success": true})
}
