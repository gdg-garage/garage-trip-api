package models

import (
	"gorm.io/gorm"
)

type RegistrationHistory struct {
	gorm.Model
	RegistrationID     uint   `json:"registration_id"`
	UserID             uint   `json:"user_id"`
	Event              string `json:"event"`
	RegistrationFields `gorm:"embedded"`
}
