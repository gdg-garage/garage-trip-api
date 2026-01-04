package models

import (
	"gorm.io/gorm"
)

type User struct {
	gorm.Model
	DiscordID string `gorm:"uniqueIndex"`
	Username  string
	Email     string
	Avatar    string
}
