package main

import (
	"context"
	"embed"
	"freegfw/database"
	"freegfw/models"
	"freegfw/routes"
	"freegfw/services"
	"io/fs"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

//go:embed frontend/dist
var distEmbed embed.FS

func main() {
	os.MkdirAll("data", 0755)

	database.Connect("data/freegfw.db")
	services.MigrateTemplates()

	services.InitSSEHub()

	services.ConnectSingboxAndBroadcast()
	go services.StartSyncLoop()
	go services.StartCertificateRenewalLoop()

	var inited models.Setting
	if database.DB.Where("key = ?", "inited").Limit(1).Find(&inited).RowsAffected > 0 {
		core := services.NewCoreService()
		core.Refresh()
		core.Start()
	}

	distFS, err := fs.Sub(distEmbed, "frontend/dist")
	if err != nil {
		panic(err)
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	// Channel to listen for OS signals
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	for {
		r := routes.SetupRouter(distFS)
		srv := &http.Server{
			Addr:    ":" + port,
			Handler: r,
		}

		certFile := "data/certificate.crt"
		keyFile := "data/private.key"
		hasCert := false
		if _, err := os.Stat(certFile); err == nil {
			if _, err := os.Stat(keyFile); err == nil {
				hasCert = true
			}
		}

		go func() {
			var err error
			if hasCert {
				log.Println("Starting HTTPS server on port " + port)
				err = srv.ListenAndServeTLS(certFile, keyFile)
			} else {
				log.Println("Starting HTTP server on port " + port)
				err = srv.ListenAndServe()
			}
			if err != nil && err != http.ErrServerClosed {
				log.Printf("Server error: %s\n", err)
			}
		}()

		select {
		case <-quit:
			log.Println("Shutting down server...")
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			if err := srv.Shutdown(ctx); err != nil {
				log.Fatal("Server forced to shutdown:", err)
			}
			return

		case <-services.RestartChan:
			log.Println("Restart signal received. Restarting server...")
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			if err := srv.Shutdown(ctx); err != nil {
				log.Println("Server shutdown error:", err)
			}
			// Loop continues, recreating router and server
		}
	}
}
