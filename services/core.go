package services

import (
	"bytes"
	"context"
	"encoding/json"
	"log"
	"sync"
	"time"

	box "github.com/sagernet/sing-box"
	"github.com/sagernet/sing-box/include"
	"github.com/sagernet/sing-box/option"

	"github.com/sagernet/sing-box/experimental/clashapi/trafficontrol"

	xray_core "github.com/xtls/xray-core/core"
	"github.com/xtls/xray-core/infra/conf/serial"
	_ "github.com/xtls/xray-core/main/distro/all"

	"freegfw/database"
	"freegfw/models"
)

type CoreService struct {
	ConfigContent  []byte
	instance       *box.Box            // Singbox instance
	xrayInstance   *xray_core.Instance // Xray instance
	cancel         context.CancelFunc
	TrafficManager *trafficontrol.Manager
	CurrentEngine  string // "singbox" or "xray"
}

var (
	coreInstance *CoreService
	coreOnce     sync.Once
)

func NewCoreService() *CoreService {
	coreOnce.Do(func() {
		coreInstance = &CoreService{
			CurrentEngine: "singbox",
		}
	})
	return coreInstance
}

func (c *CoreService) Refresh() error {
	var s models.Setting
	database.DB.Where("key = ?", "server").Limit(1).Find(&s)

	var server map[string]interface{}
	if len(s.Value) > 0 {
		if err := json.Unmarshal(s.Value, &server); err != nil {
			var str string
			if err2 := json.Unmarshal(s.Value, &str); err2 == nil {
				json.Unmarshal([]byte(str), &server)
			}
		}
	}

	var t models.Setting
	database.DB.Where("key = ?", "template").Limit(1).Find(&t)
	var templateName string
	if len(t.Value) > 0 {
		if err := json.Unmarshal(t.Value, &templateName); err != nil {
			templateName = string(t.Value)
		}
	}

	if templateName == "" {
		return nil
	}

	// Determine Engine
	tmpl, err := LoadTemplate(templateName)
	if err == nil {
		coreName, _ := tmpl.Core.(string)
		if coreName == "xray" {
			c.CurrentEngine = "xray"
			return c.refreshXray(server, templateName)
		}
	}

	c.CurrentEngine = "singbox"
	return c.refreshSingbox(server, templateName)
}

func (c *CoreService) IsRunning() bool {
	if c.CurrentEngine == "xray" {
		return c.xrayInstance != nil
	}
	return c.instance != nil
}

func (c *CoreService) Kill() error {
	if c.cancel != nil {
		c.cancel()
		c.cancel = nil
	}
	// Kill Singbox
	if c.instance != nil {
		c.instance.Close()
		c.instance = nil
	}
	// Kill Xray
	if c.xrayInstance != nil {
		c.xrayInstance.Close()
		c.xrayInstance = nil
	}

	time.Sleep(1 * time.Second)
	return nil
}

func (c *CoreService) Start() error {
	log.Println("start engine:", c.CurrentEngine)
	if len(c.ConfigContent) == 0 {
		return nil
	}
	c.Kill()

	if c.CurrentEngine == "xray" {
		// Parse JSON config to Xray Core Config
		coreConfig, err := serial.LoadJSONConfig(bytes.NewReader(c.ConfigContent))
		if err != nil {
			log.Println("Failed to parse xray config (json):", err)
			return err
		}

		instance, err := xray_core.New(coreConfig)
		if err != nil {
			log.Println("Failed to create xray instance:", err)
			return err
		}

		if err := instance.Start(); err != nil {
			log.Println("Failed to start xray:", err)
			return err
		}

		c.xrayInstance = instance
		c.TrafficManager = nil // Xray internal traffic tracking is different
		return nil
	}

	// Singbox Start
	ctx, cancel := context.WithCancel(context.Background())
	ctx = include.Context(ctx)
	c.cancel = cancel

	var options option.Options
	if err := options.UnmarshalJSONContext(ctx, c.ConfigContent); err != nil {
		log.Println("Failed to parse singbox config:", err)
		return err
	}

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
