package controllers

import (
	"encoding/json"
	"freegfw/database"
	"freegfw/models"
	"freegfw/services"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

func InitLetsEncrypt(c *gin.Context) {
	var payload struct {
		Email  string `json:"email" binding:"required"`
		Domain string `json:"domain"`
	}
	if err := c.ShouldBindJSON(&payload); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if payload.Domain == "" {
		ip, err := services.GetIPv4()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get IPv4: " + err.Error()})
			return
		}
		payload.Domain = ip
	}

	err := services.ApplyCertificate(payload.Domain, payload.Email)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Certificate application failed: " + err.Error()})
		return
	}

	saveLetSetting("letsencrypt_email", payload.Email)
	saveLetSetting("letsencrypt_domain", payload.Domain)
	saveLetSetting("letsencrypt_updated_at", time.Now().UnixMilli())

	c.JSON(http.StatusOK, gin.H{"success": true})
	go func() {
		time.Sleep(1 * time.Second)
		services.RestartChan <- struct{}{}
	}()
}

func saveLetSetting(key string, val interface{}) {
	v, _ := json.Marshal(val)
	var s models.Setting
	// Use Find/Limit(1) instead of First to avoid "record not found" log noise
	// when the setting doesn't exist yet.
	if result := database.DB.Where("key = ?", key).Limit(1).Find(&s); result.RowsAffected == 0 {
		s = models.Setting{Key: key}
	}
	s.Value = models.JSON(v)
	database.DB.Save(&s)
}
