package handlers

import (
	"context"
	"testing"
	"time"

	"github.com/gdg-garage/garage-trip-api/internal/auth"
	"github.com/gdg-garage/garage-trip-api/internal/models"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestHandleRegister(t *testing.T) {
	// Setup in-memory DB
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to connect database: %v", err)
	}

	db.AutoMigrate(&models.Registration{}, &models.User{})

	// Create a dummy user
	user := models.User{DiscordID: "123456789"}
	db.Create(&user)

	handler := NewRegistrationHandler(db)

	arrival := time.Now().Add(24 * time.Hour)
	departure := time.Now().Add(48 * time.Hour)
	reqBody := RegistrationRequest{}
	reqBody.Body.ArrivalDate = arrival
	reqBody.Body.DepartureDate = departure
	reqBody.Body.FoodRestrictions = "No peanuts"
	reqBody.Body.ChildrenCount = 2

	// Create context with UserID
	ctx := context.WithValue(context.Background(), auth.UserIDKey, user.ID)

	resp, err := handler.HandleRegister(ctx, &reqBody)
	if err != nil {
		t.Fatalf("First HandleRegister returned error: %v", err)
	}

	if resp == nil {
		t.Fatal("expected response from first call, got nil")
	}

	// Update data for second registration
	reqBody.Body.ChildrenCount = 5
	reqBody.Body.FoodRestrictions = "Vegan"

	resp, err = handler.HandleRegister(ctx, &reqBody)
	if err != nil {
		t.Fatalf("Second HandleRegister (upsert) returned error: %v", err)
	}

	if resp == nil {
		t.Fatal("expected response from second call, got nil")
	}

	// Verify DB entry
	var count int64
	db.Model(&models.Registration{}).Count(&count)
	if count != 1 {
		t.Errorf("expected 1 registration in DB, got %d", count)
	}

	var registration models.Registration
	if err := db.Preload("User").First(&registration).Error; err != nil {
		t.Fatalf("failed to find registration: %v", err)
	}

	if registration.FoodRestrictions != "Vegan" {
		t.Errorf("expected 'Vegan', got '%s'", registration.FoodRestrictions)
	}

	if registration.ChildrenCount != 5 {
		t.Errorf("expected 5 children, got %d", registration.ChildrenCount)
	}
}
