package models

import (
	"time"

	"gorm.io/gorm"
)

type RegistrationFields struct {
	ArrivalDate      time.Time `json:"arrival_date"`
	DepartureDate    time.Time `json:"departure_date"`
	FoodRestrictions string    `json:"food_restrictions"`
	ChildrenCount    int       `json:"children_count"`
	Cancelled        bool      `json:"cancelled"`
	Note             string    `json:"note"`
}

type Registration struct {
	gorm.Model
	UserID             uint   `json:"user_id" gorm:"uniqueIndex:idx_user_event"`
	Event              string `json:"event" gorm:"uniqueIndex:idx_user_event"`
	User               User   `gorm:"foreignKey:UserID"`
	RegistrationFields `gorm:"embedded"`
}
