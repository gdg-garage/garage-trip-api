package models

import (
	"time"

	"gorm.io/gorm"
)

type Registration struct {
	gorm.Model
	UserID        uint      `json:"user_id"`
	User          User      `gorm:"foreignKey:UserID"`
	ArrivalDate   time.Time `json:"arrival_date"`
	DepartureDate time.Time `json:"departure_date"`
	FoodRestrictions string    `json:"food_restrictions"`
	ChildrenCount    int       `json:"children_count"`
}
