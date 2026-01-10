package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
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
	reqBody := RegistrationRequest{
		ArrivalDate:      arrival,
		DepartureDate:    departure,
		FoodRestrictions: "No peanuts",
		ChildrenCount:    2,
	}

	body, _ := json.Marshal(reqBody)
	req, _ := http.NewRequest("POST", "/register", bytes.NewBuffer(body))

	// Add UserID to context
	ctx := context.WithValue(req.Context(), auth.UserIDKey, user.ID)
	req = req.WithContext(ctx)

	rr := httptest.NewRecorder()
	handler.HandleRegister(rr, req)

	if rr.Code != http.StatusCreated {
		t.Errorf("expected status 201, got %d", rr.Code)
	}

	// Verify DB entry
	var registration models.Registration
	if err := db.Preload("User").First(&registration).Error; err != nil {
		t.Fatalf("failed to find registration: %v", err)
	}

	if registration.FoodRestrictions != "No peanuts" {
		t.Errorf("expected 'No peanuts', got '%s'", registration.FoodRestrictions)
	}

	if registration.ChildrenCount != 2 {
		t.Errorf("expected 2 children, got %d", registration.ChildrenCount)
	}
}
