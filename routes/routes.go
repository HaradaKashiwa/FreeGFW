package routes

import (
	"encoding/base64"
	"encoding/json"
	"freegfw/controllers"
	"freegfw/database"
	"freegfw/models"
	"freegfw/services"
	"io/fs"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

type RateLimiter struct {
	mu       sync.Mutex
	attempts map[string][]time.Time
}

var limiter = &RateLimiter{
	attempts: make(map[string][]time.Time),
}

func (r *RateLimiter) Allow(ip string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Filter timestamps within 10 minutes
	now := time.Now()
	var validAttempts []time.Time
	for _, t := range r.attempts[ip] {
		if now.Sub(t) < 10*time.Minute {
			validAttempts = append(validAttempts, t)
		}
	}
	r.attempts[ip] = validAttempts

	// Block if there are 3 or more failed attempts
	return len(validAttempts) < 3
}

func (r *RateLimiter) Record(ip string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.attempts[ip] = append(r.attempts[ip], time.Now())
}

func (r *RateLimiter) Reset(ip string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.attempts, ip)
}

func SetupRouter(staticFS fs.FS) *gin.Engine {
	r := gin.Default()

	indexData, err := fs.ReadFile(staticFS, "index.html")
	if err != nil {
		indexData = []byte("Frontend not found")
	}

	r.Use(func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS, PUT, DELETE")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}
		c.Next()
	})

	api := r.Group("/")
	api.Use(AuthMiddleware())
	{
		api.GET("/templates", controllers.GetTemplates)
		api.POST("/templates/init", controllers.InitTemplate)
		api.POST("/templates/create", controllers.CreateTemplate)

		api.GET("/configs", controllers.GetConfigs)
		api.POST("/configs/reload", controllers.ReloadConfig)
		api.POST("/configs/reset", controllers.ResetConfig)
		api.POST("/configs/title", controllers.SetTitle)
		api.PUT("/configs/update", controllers.UpdateConfig)

		api.POST("/users", controllers.AddUsers)
		api.GET("/users", controllers.GetUsers)
		api.DELETE("/users/:id", controllers.DeleteUser)

		api.POST("/letsencrypt/init", controllers.InitLetsEncrypt)

		api.POST("/link/create", controllers.CreateLink)
		api.POST("/link/swap", controllers.SwapLink)
		api.GET("/link/list", controllers.ListLinks)
		api.DELETE("/link/:id", controllers.DeleteLink)
	}

	r.POST("/link/:code", controllers.BindLink)
	r.GET("/subscribe/:uuid", controllers.GetSubscribe)

	// Authorized group for frontend and internal operations
	authorized := r.Group("/")
	authorized.Use(AuthMiddleware())
	{
		authorized.GET("/stream/traffic", gin.WrapF(services.ServeSSE))

		// Static file serving from embedded FS
		authorized.StaticFileFS("/favicon.ico", "favicon.ico", http.FS(staticFS))
		authorized.StaticFileFS("/logo.svg", "logo.svg", http.FS(staticFS))

		if assetsFS, err := fs.Sub(staticFS, "assets"); err == nil {
			authorized.StaticFS("/assets", http.FS(assetsFS))
		}
		if imagesFS, err := fs.Sub(staticFS, "images"); err == nil {
			authorized.StaticFS("/images", http.FS(imagesFS))
		}

		// Serve index for root
		authorized.GET("/", func(c *gin.Context) {
			c.Data(http.StatusOK, "text/html; charset=utf-8", indexData)
		})
	}

	r.NoRoute(AuthMiddleware(), func(c *gin.Context) {
		path := c.Request.URL.Path
		// If it looks like an API call or static asset but didn't match, return 404
		if strings.HasPrefix(path, "/api") ||
			strings.HasPrefix(path, "/socket.io/") ||
			strings.HasPrefix(path, "/assets") ||
			strings.HasPrefix(path, "/images") ||
			path == "/logo.svg" ||
			path == "/favicon.ico" {
			c.Status(http.StatusNotFound)
			return
		}
		// Otherwise serve index.html for SPA
		c.Data(http.StatusOK, "text/html; charset=utf-8", indexData)
	})

	return r
}

func AuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		if os.Getenv("GO_ENV") == "development" {
			c.Next()
			return
		}

		var uSetting models.Setting
		database.DB.Where("key = ?", "username").Limit(1).Find(&uSetting)
		if uSetting.ID == 0 {
			c.Next()
			return
		}

		// If username not set (empty list of results or error), First returns error or zero value.
		// Check if value is actually set
		var uVal string
		json.Unmarshal(uSetting.Value, &uVal)
		if uVal == "" {
			c.Next()
			return
		}

		clientIP := c.ClientIP()
		if !limiter.Allow(clientIP) {
			c.AbortWithStatus(http.StatusTooManyRequests)
			return
		}

		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.Header("WWW-Authenticate", `Basic realm="Protected"`)
			c.AbortWithStatus(http.StatusUnauthorized)
			return
		}

		parts := strings.Split(authHeader, " ")
		if len(parts) == 2 && parts[0] == "Basic" {
			payload, _ := base64.StdEncoding.DecodeString(parts[1])
			pair := strings.SplitN(string(payload), ":", 2)
			if len(pair) == 2 {
				var storedUser string
				json.Unmarshal(uSetting.Value, &storedUser)

				var pSetting models.Setting
				database.DB.Where("key = ?", "password").Limit(1).Find(&pSetting)
				var storedPass string
				json.Unmarshal(pSetting.Value, &storedPass)

				if pair[0] == storedUser && pair[1] == storedPass {
					limiter.Reset(clientIP)
					c.Next()
					return
				}
			}
		}

		limiter.Record(clientIP)
		c.Header("WWW-Authenticate", `Basic realm="Protected"`)
		c.AbortWithStatus(http.StatusUnauthorized)
	}
}
