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

func TestHandleRegister_MultiUser(t *testing.T) {
	// Setup in-memory DB
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to connect database: %v", err)
	}

	db.AutoMigrate(&models.Registration{}, &models.User{}, &models.RegistrationHistory{})

	// Create two users
	user1 := models.User{DiscordID: "user1", Username: "user1"}
	if err := db.Create(&user1).Error; err != nil {
		t.Fatalf("Failed to create user1: %v", err)
	}
	user2 := models.User{DiscordID: "user2", Username: "user2"}
	if err := db.Create(&user2).Error; err != nil {
		t.Fatalf("Failed to create user2: %v", err)
	}

	t.Logf("User1 ID: %d, User2 ID: %d", user1.ID, user2.ID)

	authHandler := auth.NewAuthHandler(&config.Config{JWTSecret: "test-secret"}, db, nil)
	handler := NewRegistrationHandler(db, nil, authHandler)

	// User 1 registers
	arrival := time.Now().Add(24 * time.Hour)
	departure := time.Now().Add(48 * time.Hour)
	req1 := RegistrationRequest{}
	req1.Body.ArrivalDate = arrival
	req1.Body.DepartureDate = departure
	req1.Body.FoodRestrictions = "User1 Allergy"
	req1.Body.Event = "event-1"

	if user1.ID == 0 {
		t.Fatal("user1.ID is 0 after Create")
	}
	token1, _ := authHandler.GenerateToken(user1.ID)
	req1.Cookie = "auth_token=" + token1

	_, err = handler.HandleRegister(context.Background(), &req1)
	if err != nil {
		t.Fatalf("User 1 registration failed: %v", err)
	}

	// User 2 registers
	req2 := RegistrationRequest{}
	req2.Body.ArrivalDate = arrival
	req2.Body.DepartureDate = departure
	req2.Body.FoodRestrictions = "User2 Preference"
	req2.Body.Event = "event-2"

	token2, _ := authHandler.GenerateToken(user2.ID)
	req2.Cookie = "auth_token=" + token2

	_, err = handler.HandleRegister(context.Background(), &req2)
	if err != nil {
		t.Fatalf("User 2 registration failed: %v", err)
	}

	// Verify both exist
	var registrations []models.Registration
	db.Order("user_id asc").Find(&registrations)

	if len(registrations) != 2 {
		t.Errorf("expected 2 registrations, got %d", len(registrations))
		for _, reg := range registrations {
			t.Logf("Reg ID: %d, UserID: %d", reg.ID, reg.UserID)
		}
	}

	var reg1, reg2 models.Registration
	db.Where("user_id = ?", user1.ID).First(&reg1)
	db.Where("user_id = ?", user2.ID).First(&reg2)

	if reg1.UserID != user1.ID {
		t.Errorf("Reg1 UserID mismatch: expected %d, got %d", user1.ID, reg1.UserID)
	}
	if reg2.UserID != user2.ID {
		t.Errorf("Reg2 UserID mismatch: expected %d, got %d", user2.ID, reg2.UserID)
	}

	if reg1.FoodRestrictions != "User1 Allergy" {
		t.Errorf("Reg1 FoodRestrictions mismatch: expected 'User1 Allergy', got '%s'", reg1.FoodRestrictions)
	}
	if reg2.FoodRestrictions != "User2 Preference" {
		t.Errorf("Reg2 FoodRestrictions mismatch: expected 'User2 Preference', got '%s'", reg2.FoodRestrictions)
	}
	if reg1.Event != "event-1" {
		t.Errorf("Reg1 Event mismatch: expected 'event-1', got '%s'", reg1.Event)
	}
	if reg2.Event != "event-2" {
		t.Errorf("Reg2 Event mismatch: expected 'event-2', got '%s'", reg2.Event)
	}
}

func TestHandleRegister_ZeroUserID(t *testing.T) {
	// Setup in-memory DB
	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to connect database: %v", err)
	}
	// Use a unique name for each test or just don't share cache
	db, _ = gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})

	db.AutoMigrate(&models.Registration{}, &models.User{}, &models.RegistrationHistory{})

	// Create a user with ID 1
	user1 := models.User{DiscordID: "user1", Username: "user1"}
	db.Create(&user1) // Should get ID 1

	authHandler := auth.NewAuthHandler(&config.Config{JWTSecret: "test-secret"}, db, nil)
	handler := NewRegistrationHandler(db, nil, authHandler)

	// Register for user 1
	req1 := RegistrationRequest{}
	req1.Body.FoodRestrictions = "User1"
	token1, _ := authHandler.GenerateToken(user1.ID)
	req1.Cookie = "auth_token=" + token1
	handler.HandleRegister(context.Background(), &req1)

	// Now try to register with userID 0 (simulated by a bad token if Authorize allowed it)
	// Since HandleRegister calls Authorize, we need to bypass it or simulate it.
	// But HandleRegister explicitly checks err from Authorize.

	// If somehow userID becomes 0...
	registration := models.Registration{}
	db.Where("user_id = ?", 0).FirstOrInit(&registration)

	if registration.ID != 0 {
		t.Errorf("FirstOrInit with UserID=0 matched an existing record (ID=%d) even after fix!", registration.ID)
	}
}
