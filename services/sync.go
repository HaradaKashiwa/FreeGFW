package services

import (
	"bytes"
	"context"
	"encoding/json"
	"freegfw/database"
	"freegfw/models"
	"io/ioutil"
	"net"
	"net/http"
	"time"
)

func StartSyncLoop() {
	for {
		change := false
		time.Sleep(1 * time.Second)

		var links []models.Link
		threshold := time.Now().Add(-1 * time.Minute).Unix()

		// Note: SQLite might store integers or strings depending on how we saved using GORM.
		// models.Link uses int64.
		// Logic: last_sync_at < threshold OR last_sync_status = 'pending'

		// Handling pointers in query
		// We use map for updates.

		if err := database.DB.Where("last_sync_at < ? OR last_sync_status = ?", threshold, "pending").Find(&links).Error; err != nil {
			continue
		}

		if len(links) == 0 {
			continue
		}

		for _, link := range links {
			myLink, _ := GetMyLink(link.LocalCode)
			payload := map[string]string{"link": myLink}
			jsonData, _ := json.Marshal(payload)

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
			resp, err := client.Post(link.Link, "application/json", bytes.NewBuffer(jsonData))

			if err != nil {
				// Handle error
				errMsg := err.Error()
				database.DB.Model(&link).Updates(map[string]interface{}{
					"last_sync_status": "failed",
					"last_sync_at":     time.Now().Unix(),
					"error":            errMsg,
				})
				continue
			}

			defer resp.Body.Close()
			body, _ := ioutil.ReadAll(resp.Body)
			var data struct {
				Success bool            `json:"success"`
				ETag    string          `json:"eTag"`
				Server  json.RawMessage `json:"server"`
				Title   string          `json:"title"`
				Users   json.RawMessage `json:"users"`
				IP      string          `json:"ip"`
				Error   string          `json:"message"`
			}

			if err := json.Unmarshal(body, &data); err != nil {
				// update error
				continue
			}

			if resp.StatusCode != 200 {
				// failed
				users := models.JSON(nil)
				server := models.JSON(nil)
				if resp.StatusCode == 401 {
					// clear data
				}
				database.DB.Model(&link).Updates(map[string]interface{}{
					"last_sync_status": "failed",
					"last_sync_at":     time.Now().Unix(),
					"error":            data.Error,
					"users":            users,
					"server":           server,
				})
				continue
			}

			if link.ETag != nil && *link.ETag == data.ETag {
				continue
			}
			change = true

			serverBytes, _ := data.Server.MarshalJSON()

			// Inject title into server JSON
			// Inject title into server JSON
			var serverMap map[string]interface{}
			if err := json.Unmarshal(serverBytes, &serverMap); err != nil || serverMap == nil {
				serverMap = make(map[string]interface{})
			}

			if data.Title != "" {
				serverMap["title"] = data.Title
				serverBytes, _ = json.Marshal(serverMap)
			}

			usersBytes, _ := data.Users.MarshalJSON()

			updates := map[string]interface{}{
				"last_sync_status": "success",
				"last_sync_at":     time.Now().Unix(),
				"server":           models.JSON(serverBytes),
				"users":            models.JSON(usersBytes),
				"ip":               data.IP,
				"error":            nil,
				"e_tag":            data.ETag,
			}
			if data.Title != "" {
				updates["name"] = data.Title
			}
			database.DB.Model(&link).Updates(updates)
		}

		if change {
			core := NewCoreService()
			core.Refresh()
			core.Start()
		}
	}
}
