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
}

type Registration struct {
	gorm.Model
	UserID             uint `json:"user_id"`
	User               User `gorm:"foreignKey:UserID"`
	RegistrationFields `gorm:"embedded"`
}
