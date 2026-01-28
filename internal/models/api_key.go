package models

import (
	"time"

	"gorm.io/gorm"
)

type APIKey struct {
	gorm.Model
	UserID     uint       `json:"user_id"`
	User       User       `json:"user"`
	Key        string     `json:"key" gorm:"uniqueIndex"`
	Name       string     `json:"name"`
	ExpiresAt  *time.Time `json:"expires_at"`
	LastUsedAt *time.Time `json:"last_used_at"`
}
