package models

import (
	"database/sql/driver"
	"encoding/json"
	"errors"
	"time"
)

type JSON json.RawMessage

// Scan scan value into Jsonb, implements sql.Scanner interface
func (j *JSON) Scan(value interface{}) error {
	bytes, ok := value.([]byte)
	if !ok {
		s, ok := value.(string)
		if !ok {
			if value == nil {
				*j = nil
				return nil
			}
			return errors.New("type assertion to []byte failed")
		}
		bytes = []byte(s)
	}

	result := json.RawMessage{}
	// If unmarshal fails (e.g. it is a plain string), we just accept it as raw bytes
	// This usually happens if legacy data was stored as plain text
	if err := json.Unmarshal(bytes, &result); err != nil {
		*j = JSON(bytes)
		return nil // Swallow error to allow reading corrupted/legacy data
	}
	*j = JSON(result)
	return nil
}

// Value return json value, implement driver.Valuer interface
func (j JSON) Value() (driver.Value, error) {
	if len(j) == 0 {
		return nil, nil
	}
	return string(j), nil
}

type User struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	UUID      string    `json:"uuid"`
	Username  string    `json:"username"`
	Upload    int64     `json:"upload" gorm:"default:0"`
	Download  int64     `json:"download" gorm:"default:0"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

type Link struct {
	ID             uint      `gorm:"primaryKey" json:"id"`
	LocalCode      string    `json:"localCode"`
	Link           string    `json:"link"`
	LastSyncAt     *int64    `json:"lastSyncAt"`
	LastSyncStatus string    `json:"lastSyncStatus"`
	Server         JSON      `gorm:"type:text" json:"server"`
	Users          JSON      `gorm:"type:text" json:"users"`
	IP             *string   `json:"ip"`
	Name           *string   `json:"name"`
	Error          *string   `json:"error"`
	ETag           *string   `json:"eTag"`
	CreatedAt      time.Time `json:"createdAt"`
	UpdatedAt      time.Time `json:"updatedAt"`
}

type Setting struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	Key       string    `json:"key"`
	Value     JSON      `gorm:"type:text" json:"value"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

type Template struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	Slug        string    `json:"slug" gorm:"uniqueIndex"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Content     JSON      `gorm:"type:text" json:"content"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
}
