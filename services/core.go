package services

import (
	"context"
	"encoding/json"
	"log"
	"sync"
	"time"

	box "github.com/sagernet/sing-box"
	_ "github.com/sagernet/sing-box/include"
	"github.com/sagernet/sing-box/option"

	"github.com/sagernet/sing-box/experimental/clashapi/trafficontrol"

	"freegfw/database"
	"freegfw/models"
	"strings"
)

type CoreService struct {
	ConfigContent  []byte
	instance       *box.Box
	cancel         context.CancelFunc
	TrafficManager *trafficontrol.Manager
}

var (
	coreInstance *CoreService
	coreOnce     sync.Once
)

func NewCoreService() *CoreService {
	coreOnce.Do(func() {
		coreInstance = &CoreService{}
	})
	return coreInstance
}

func (c *CoreService) Refresh() error {
	var s models.Setting
	database.DB.Where("key = ?", "server").Limit(1).Find(&s)

	var server map[string]interface{}
	if len(s.Value) > 0 {
		// Try unmarshal directly
		if err := json.Unmarshal(s.Value, &server); err != nil {
			// If failed, maybe it is a stringified JSON?
			var str string
			if err2 := json.Unmarshal(s.Value, &str); err2 == nil {
				json.Unmarshal([]byte(str), &server)
			} else {
				log.Println("Failed to parse server setting:", err)
			}
		}
	}

	var t models.Setting
	database.DB.Where("key = ?", "template").Limit(1).Find(&t)
	var templateName string
	if len(t.Value) > 0 {
		// Template name might be just a string "foo" or JSON string "\"foo\""
		if err := json.Unmarshal(t.Value, &templateName); err != nil {
			// fallback: assume it is raw stringbytes
			templateName = string(t.Value)
		}
	}

	if templateName == "" {
		return nil
	}

	delete(server, "flow")

	if tlsConfig, ok := server["tls"].(map[string]interface{}); ok {
		if reality, ok := tlsConfig["reality"].(map[string]interface{}); ok {
			if pk, ok := reality["private_key"].(string); ok {
				// Clean up padding for older generated keys
				reality["private_key"] = strings.TrimRight(pk, "=")
			}
			delete(reality, "public_key")
		}
	}

	users, _ := BuildUsers(templateName)
	tls, _ := BuildServerTLS(templateName)

	server["users"] = users
	if tls != nil {
		if serverTls, ok := server["tls"].(map[string]interface{}); ok {
			for k, v := range tls {
				serverTls[k] = v
			}
		}
	}

	config := map[string]interface{}{
		"inbounds": []map[string]interface{}{
			server,
		},
		"outbounds": []map[string]interface{}{
			{
				"type": "direct",
				"tag":  "direct",
			},
		},
		"experimental": map[string]interface{}{
			"clash_api": map[string]interface{}{
				"external_controller": "127.0.0.1:0",
			},
		},
	}

	data, _ := json.MarshalIndent(config, "", "  ")
	c.ConfigContent = data
	return nil
}

func (c *CoreService) IsRunning() bool {
	return c.instance != nil
}

func (c *CoreService) Kill() error {
	if c.cancel != nil {
		c.cancel()
		c.cancel = nil
	}
	if c.instance != nil {
		c.instance.Close()
		c.instance = nil
	}

	time.Sleep(1 * time.Second)
	return nil
}

func (c *CoreService) Start() error {
	log.Println("start singbox")
	if len(c.ConfigContent) == 0 {
		return nil
	}
	c.Kill()

	var options option.Options
	if err := json.Unmarshal(c.ConfigContent, &options); err != nil {
		log.Println("Failed to parse singbox config:", err)
		return err
	}

	ctx, cancel := context.WithCancel(context.Background())
	c.cancel = cancel

	instance, err := box.New(box.Options{
		Context: ctx,
		Options: options,
	})
	if err != nil {
		cancel()
		log.Println("Failed to create singbox instance:", err)
		return err
	}
	c.instance = instance

	c.TrafficManager = trafficontrol.NewManager()
	tracker := NewStatisticsTracker(c.TrafficManager, instance.Outbound())
	instance.Router().AppendTracker(tracker)

	if err := instance.Start(); err != nil {
		c.Kill()
		log.Println("Failed to start singbox:", err)
		return err
	}

	return nil
}

func (c *CoreService) Restart() error {
	return c.Start()
}
