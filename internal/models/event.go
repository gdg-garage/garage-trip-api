package models

import (
	"time"

	"gorm.io/gorm"
)

type Event struct {
	gorm.Model
	Code        string     `gorm:"uniqueIndex;not null" json:"code"`
	Name        string     `gorm:"not null" json:"name"`
	StartDate   *time.Time `json:"start_date,omitempty"`
	EndDate     *time.Time `json:"end_date,omitempty"`
	Location    string     `json:"location,omitempty"`
	Description string     `json:"description,omitempty"`
	Enabled     bool       `gorm:"default:false" json:"enabled"`
	Status      string     `gorm:"default:'future'" json:"status"` // "active", "future", "past"
}
