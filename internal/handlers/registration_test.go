package handlers

import (
	"context"
	"testing"
	"time"

	"github.com/gdg-garage/garage-trip-api/internal/auth"
	"github.com/gdg-garage/garage-trip-api/internal/config"
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

	db.AutoMigrate(&models.Registration{}, &models.User{}, &models.RegistrationHistory{})

	// Create a dummy user
	user := models.User{DiscordID: "123456789"}
	db.Create(&user)

	authHandler := auth.NewAuthHandler(&config.Config{JWTSecret: "test-secret"}, db, nil)
	handler := NewRegistrationHandler(db, nil, authHandler)

	arrival := time.Now().Add(24 * time.Hour)
	departure := time.Now().Add(48 * time.Hour)
	reqBody := RegistrationRequest{}
	reqBody.Body.ArrivalDate = arrival
	reqBody.Body.DepartureDate = departure
	reqBody.Body.FoodRestrictions = "No peanuts"
	reqBody.Body.ChildrenCount = 2
	reqBody.Body.Note = "Special note"

	// Generate a token for the test
	token, _ := authHandler.GenerateToken(user.ID)
	reqBody.Cookie = "auth_token=" + token

	resp, err := handler.HandleRegister(context.Background(), &reqBody)
	if err != nil {
		t.Fatalf("First HandleRegister returned error: %v", err)
	}

	if resp == nil {
		t.Fatal("expected response from first call, got nil")
	}

	// Update data for second registration
	reqBody.Body.ChildrenCount = 5
	reqBody.Body.FoodRestrictions = "Vegan"
	reqBody.Body.Note = "Updated note"

	resp, err = handler.HandleRegister(context.Background(), &reqBody)
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

	var historyCount int64
	db.Model(&models.RegistrationHistory{}).Count(&historyCount)
	if historyCount != 2 {
		t.Errorf("expected 2 history entries in DB, got %d", historyCount)
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
	if registration.Note != "Updated note" {
		t.Errorf("expected 'Updated note', got '%s'", registration.Note)
	}

	var histories []models.RegistrationHistory
	db.Order("id asc").Find(&histories)
	if len(histories) != 2 {
		t.Fatalf("expected 2 history entries, got %d", len(histories))
	}

	if histories[0].FoodRestrictions != "No peanuts" {
		t.Errorf("expected first history to have 'No peanuts', got '%s'", histories[0].FoodRestrictions)
	}
	if histories[1].FoodRestrictions != "Vegan" {
		t.Errorf("expected second history to have 'Vegan', got '%s'", histories[1].FoodRestrictions)
	}
	if histories[1].Note != "Updated note" {
		t.Errorf("expected second history to have 'Updated note', got '%s'", histories[1].Note)
	}

	// Test cancellation
	reqBody.Body.Cancelled = true
	_, err = handler.HandleRegister(context.Background(), &reqBody)
	if err != nil {
		t.Fatalf("Third HandleRegister (cancel) returned error: %v", err)
	}

	db.First(&registration)
	if !registration.Cancelled {
		t.Errorf("expected registration to be cancelled")
	}

	db.Model(&models.RegistrationHistory{}).Count(&historyCount)
	if historyCount != 3 {
		t.Errorf("expected 3 history entries in DB, got %d", historyCount)
	}
}
