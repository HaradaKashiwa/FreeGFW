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

func GetTemplates(c *gin.Context) {
	templates, err := services.GetTemplates()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, templates)
}

func CreateTemplate(c *gin.Context) {
	var payload struct {
		Data string `json:"data"`
	}
	if err := c.ShouldBindJSON(&payload); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var template map[string]interface{}
	if err := json.Unmarshal([]byte(payload.Data), &template); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid JSON"})
		return
	}

	if template["server"] == nil || template["client"] == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid template"})
		return
	}

	templateName := "custom-" + time.Now().Format("20060102150405")

	name, _ := template["_name"].(string)
	description, _ := template["_description"].(string)

	newTemplate := models.Template{
		Slug:        templateName,
		Name:        name,
		Description: description,
		Content:     models.JSON([]byte(payload.Data)),
	}

	if err := database.DB.Create(&newTemplate).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	doInit(c, templateName)
}

func InitTemplate(c *gin.Context) {
	var payload struct {
		Type string `json:"type"`
	}
	if err := c.ShouldBindJSON(&payload); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	doInit(c, payload.Type)
}

func doInit(c *gin.Context, typeName string) {
	var count int64
	database.DB.Model(&models.Setting{}).Where("key = ?", "server").Count(&count)
	if count > 0 {
		c.JSON(http.StatusOK, gin.H{"success": true})
		return
	}

	if err := services.InitTemplate(typeName); err != nil {
		database.DB.Exec("DELETE FROM settings")
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	core := services.NewCoreService()
	core.Refresh()
	core.Start()

	c.JSON(http.StatusOK, gin.H{"success": true})
}
