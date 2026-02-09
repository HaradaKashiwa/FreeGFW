package database

import (
	"freegfw/models"
	"log"
	"os"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var DB *gorm.DB

func Connect(path string) {
	newLogger := logger.New(
		log.New(os.Stdout, "\r\n", log.LstdFlags), // io writer
		logger.Config{
			SlowThreshold:        time.Second, // Slow SQL threshold
			LogLevel:             logger.Warn, // Log level
			ParameterizedQueries: true,        // Don't include params in the SQL log
			Colorful:             false,       // Disable color
		},
	)

	var err error
	DB, err = gorm.Open(sqlite.Open(path), &gorm.Config{
		Logger: newLogger,
	})
	if err != nil {
		log.Fatal("Failed to connect to database:", err)
	}

	err = DB.AutoMigrate(&models.User{}, &models.Link{}, &models.Setting{}, &models.Template{})
	if err != nil {
		log.Fatal("Failed to migrate database:", err)
	}
}
