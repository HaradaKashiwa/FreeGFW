package controllers

import (
	"bytes"
	"context"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"freegfw/database"
	"freegfw/models"
	"freegfw/services"
	"freegfw/utils"
	"io/ioutil"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

var (
	linkCache = make(map[string]int64)
	linkMu    sync.Mutex
)

func CreateLink(c *gin.Context) {
	code := utils.RandomUUID()
	linkMu.Lock()
	linkCache[code] = time.Now().Add(10 * time.Minute).Unix()
	linkMu.Unlock()

	link, _ := services.GetMyLink(code)
	c.JSON(http.StatusOK, gin.H{"link": link})
}

func ListLinks(c *gin.Context) {
	var links []models.Link
	database.DB.Find(&links)
	c.JSON(http.StatusOK, links)
}

func DeleteLink(c *gin.Context) {
	id := c.Param("id")
	database.DB.Delete(&models.Link{}, id)
	c.JSON(http.StatusOK, gin.H{"success": true})
}

func SwapLink(c *gin.Context) {
	var payload struct {
		Link string `json:"link"`
	}
	if err := c.ShouldBindJSON(&payload); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var checkLink models.Link
	if database.DB.Where("link = ?", payload.Link).First(&checkLink).Error == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Link already exists"})
		return
	}

	code := utils.RandomUUID()
	myLink, _ := services.GetMyLink(code)

	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.DialContext = func(ctx context.Context, network, addr string) (net.Conn, error) {
		dialer := net.Dialer{
			Timeout:   10 * time.Second,
			KeepAlive: 30 * time.Second,
		}
		return dialer.DialContext(ctx, network, addr)
	}
	transport.TLSHandshakeTimeout = 10 * time.Second

	client := http.Client{
		Timeout:   10 * time.Second,
		Transport: transport,
	}
	body, _ := json.Marshal(map[string]string{"link": myLink})
	resp, err := client.Post(payload.Link, "application/json", bytes.NewBuffer(body))

	if err == nil {
		defer resp.Body.Close()
		var res map[string]interface{}
		content, _ := ioutil.ReadAll(resp.Body)
		json.Unmarshal(content, &res)
		if res["success"] == true {
			l := models.Link{
				LocalCode:      code,
				Link:           payload.Link,
				LastSyncStatus: "success",
				LastSyncAt:     func(v int64) *int64 { return &v }(time.Now().Unix()),
			}

			serverMap, ok := res["server"].(map[string]interface{})
			if !ok || serverMap == nil {
				serverMap = make(map[string]interface{})
			}

			if t, ok := res["title"].(string); ok && t != "" {
				serverMap["title"] = t
				l.Name = &t
			}

			if len(serverMap) > 0 {
				serverBytes, _ := json.Marshal(serverMap)
				l.Server = models.JSON(serverBytes)
			}

			if usersMap, ok := res["users"].(interface{}); ok { // users might be list or raw message in map
				usersBytes, _ := json.Marshal(usersMap)
				l.Users = models.JSON(usersBytes)
			}

			if ip, ok := res["ip"].(string); ok {
				l.IP = &ip
				if ip != "" {
					var checkIP models.Link
					if database.DB.Where("ip = ?", ip).First(&checkIP).Error == nil {
						c.JSON(http.StatusBadRequest, gin.H{"error": "IP already exists"})
						return
					}
				}
			}

			if etag, ok := res["eTag"].(string); ok {
				l.ETag = &etag
			}

			var existing models.Link
			if database.DB.Where("local_code = ?", code).Limit(1).Find(&existing).RowsAffected > 0 {
				l.ID = existing.ID
			}
			database.DB.Save(&l)
			c.JSON(http.StatusOK, gin.H{"success": true})
			return
		}
	}
	c.JSON(http.StatusOK, gin.H{"success": false})
}

func BindLink(c *gin.Context) {
	var payload struct {
		Link string `json:"link"`
	}
	c.ShouldBindJSON(&payload)

	code := c.Param("code")

	linkMu.Lock()
	exp, ok := linkCache[code]
	linkMu.Unlock()

	if ok {
		if time.Now().Unix() < exp {
			if payload.Link == "" {
				c.JSON(http.StatusBadRequest, gin.H{"error": "Missing link"})
				return
			}
			var checkLink models.Link
			if database.DB.Where("link = ?", payload.Link).First(&checkLink).Error == nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": "Link already exists"})
				return
			}
			l := models.Link{
				LocalCode:      code,
				Link:           payload.Link,
				LastSyncStatus: "pending",
			}
			database.DB.Create(&l)
			linkMu.Lock()
			delete(linkCache, code)
			linkMu.Unlock()
			c.JSON(http.StatusOK, getHandshakeData())
			return
		} else {
			linkMu.Lock()
			delete(linkCache, code)
			linkMu.Unlock()
		}
	}

	var existingLink models.Link
	if database.DB.Where("local_code = ?", code).Limit(1).Find(&existingLink).RowsAffected > 0 {
		c.JSON(http.StatusOK, getHandshakeData())
		return
	}

	c.JSON(http.StatusUnauthorized, gin.H{"success": false, "message": "Unauthorized"})
}

func getHandshakeData() gin.H {
	var ipSetting models.Setting
	database.DB.Where("key = ?", "ip").Limit(1).Find(&ipSetting)
	var ip string
	json.Unmarshal(ipSetting.Value, &ip)

	var serverSetting models.Setting
	database.DB.Where("key = ?", "server").Limit(1).Find(&serverSetting)
	var server interface{}
	json.Unmarshal(serverSetting.Value, &server)

	var users []models.User
	database.DB.Find(&users)
	var uuids []string
	for _, u := range users {
		uuids = append(uuids, u.UUID)
	}

	data := map[string]interface{}{
		"ip":     ip,
		"server": server,
		"users":  uuids,
	}

	var titleSetting models.Setting
	database.DB.Where("key = ?", "title").Limit(1).Find(&titleSetting)
	var title string
	if len(titleSetting.Value) > 0 {
		json.Unmarshal(titleSetting.Value, &title)
	}
	data["title"] = title

	dataBytes, _ := json.Marshal(data)
	h := sha1.New()
	h.Write(dataBytes)
	etag := hex.EncodeToString(h.Sum(nil))

	return gin.H{
		"success": true,
		"ip":      ip,
		"server":  server,
		"title":   title,
		"users":   uuids,
		"eTag":    etag,
	}
}
