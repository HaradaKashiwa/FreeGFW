package services

import (
	"bytes"
	"encoding/json"
	"freegfw/database"
	"freegfw/models"
	"io"
	"log"
	"net"
	"net/http"
	"time"
)

// syncHTTPClient is a shared HTTP client with connection pool limits.
// Reusing the same client across iterations ensures keep-alive connections
// are reused and the total number of open sockets stays bounded.
var syncHTTPClient = &http.Client{
	Timeout: 15 * time.Second,
	Transport: &http.Transport{
		DialContext: (&net.Dialer{
			Timeout:   10 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		TLSHandshakeTimeout:   10 * time.Second,
		MaxIdleConns:          10,               // Maximum idle connections in the pool
		MaxIdleConnsPerHost:   3,                // Maximum idle connections per host
		MaxConnsPerHost:       5,                // Maximum concurrent connections per host (prevent connection spike)
		IdleConnTimeout:       60 * time.Second, // Idle connection timeout
		ResponseHeaderTimeout: 10 * time.Second, // Timeout waiting for response headers
		ExpectContinueTimeout: 1 * time.Second,
	},
}

// maxResponseBodyBytes limits the response body size to prevent memory exhaustion
// from a malicious or misbehaving remote server.
const maxResponseBodyBytes = 1 << 20 // 1 MiB

func StartSyncLoop() {
	for {
		time.Sleep(1 * time.Second)

		var links []models.Link
		threshold := time.Now().Add(-1 * time.Minute).Unix()

		if err := database.DB.Where("last_sync_at < ? OR last_sync_status = ?", threshold, "pending").Find(&links).Error; err != nil {
			continue
		}

		if len(links) == 0 {
			continue
		}

		change := false
		for i := range links {
			if syncOneLink(&links[i]) {
				change = true
			}
		}

		if change {
			core := NewCoreService()
			if err := core.Refresh(); err != nil {
				log.Println("[Sync] Refresh failed, skipping Start:", err)
				continue
			}
			core.Start()
		}
	}
}

// syncOneLink performs a single sync request for one link record.
// It returns true if the remote data changed (ETag mismatch → core should restart).
// Extracting this into its own function ensures defer runs promptly after each
// link is processed, preventing file-descriptor leaks from the old loop-level defer.
func syncOneLink(link *models.Link) (changed bool) {
	myLink, _ := GetMyLink(link.LocalCode)
	payload := map[string]string{"link": myLink}
	jsonData, _ := json.Marshal(payload)

	resp, err := syncHTTPClient.Post(link.Link, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		database.DB.Model(link).Updates(map[string]interface{}{
			"last_sync_status": "failed",
			"last_sync_at":     time.Now().Unix(),
			"error":            err.Error(),
		})
		return false
	}
	// defer runs immediately when function returns, preventing leaks
	defer resp.Body.Close()

	// Limit response body reading to prevent memory exhaustion from large responses
	bodyReader := io.LimitReader(resp.Body, maxResponseBodyBytes)
	body, err := io.ReadAll(bodyReader)
	if err != nil {
		log.Println("[Sync] Failed to read response body:", err)
		return false
	}

	// Must read body to completion to allow connection reuse in the pool
	// (discard remaining data manually if any remains after LimitReader)
	io.Copy(io.Discard, resp.Body) //nolint:errcheck

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
		log.Println("[Sync] Failed to unmarshal response:", err)
		return false
	}

	if resp.StatusCode != 200 {
		database.DB.Model(link).Updates(map[string]interface{}{
			"last_sync_status": "failed",
			"last_sync_at":     time.Now().Unix(),
			"error":            data.Error,
			// 401 indicates authorization failure, clear cached user/server data
			"users":  models.JSON(nil),
			"server": models.JSON(nil),
		})
		return false
	}

	// ETag unchanged, no update needed
	if link.ETag != nil && *link.ETag == data.ETag {
		if link.LastSyncStatus != "success" {
			database.DB.Model(link).Updates(map[string]interface{}{
				"last_sync_status": "success",
				"last_sync_at":     time.Now().Unix(),
				"error":            nil,
			})
		}
		return false
	}

	serverBytes, _ := data.Server.MarshalJSON()

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
	database.DB.Model(link).Updates(updates)

	return true
}
