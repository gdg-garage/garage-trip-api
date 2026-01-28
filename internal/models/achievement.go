package models

import (
	"gorm.io/gorm"
)

type Achievement struct {
	gorm.Model
	Name          string `json:"name"`
	Image         string `json:"image"` // URL to image
	DiscordRoleID string `json:"discord_role_id"`
	Code          string `gorm:"uniqueIndex" json:"code"`
}

type AchievementGrant struct {
	gorm.Model
	AchievementID uint        `json:"achievement_id"`
	Achievement   Achievement `json:"achievement"`
	UserID        uint        `json:"user_id"`
	User          User        `json:"user"`
	GrantedByID   uint        `json:"granted_by_id"`
	GrantedBy     User        `json:"granted_by" gorm:"foreignKey:GrantedByID"`
}
