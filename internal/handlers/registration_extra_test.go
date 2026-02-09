package handlers

import (
	"context"
	"testing"

	"github.com/gdg-garage/garage-trip-api/internal/auth"
	"github.com/gdg-garage/garage-trip-api/internal/config"
	"github.com/gdg-garage/garage-trip-api/internal/models"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestHandleRegister_SeparateEvents(t *testing.T) {
	// Setup in-memory DB
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to connect database: %v", err)
	}

	db.AutoMigrate(&models.Registration{}, &models.User{}, &models.RegistrationHistory{})

	// Create a dummy user
	user := models.User{DiscordID: "123456789"}
	db.Create(&user)

	testCfg := &config.Config{JWTSecret: "test-secret", EnabledEvents: []string{"Event-A", "Event-B"}}
	authHandler := auth.NewAuthHandler(testCfg, db, nil)
	handler := NewRegistrationHandler(db, nil, authHandler, testCfg)

	token, _ := authHandler.GenerateToken(user.ID)
	authCookie := "auth_token=" + token

	// Register for Event-A
	reqA := RegistrationRequest{}
	reqA.Cookie = authCookie
	reqA.Body.Event = "Event-A"
	reqA.Body.Note = "Note A"

	if _, err := handler.HandleRegister(context.Background(), &reqA); err != nil {
		t.Fatalf("Failed to register Event-A: %v", err)
	}

	// Register for Event-B
	reqB := RegistrationRequest{}
	reqB.Cookie = authCookie
	reqB.Body.Event = "Event-B"
	reqB.Body.Note = "Note B"

	if _, err := handler.HandleRegister(context.Background(), &reqB); err != nil {
		t.Fatalf("Failed to register Event-B: %v", err)
	}

	// Verify DB has 2 registrations
	var count int64
	db.Model(&models.Registration{}).Count(&count)
	if count != 2 {
		t.Errorf("expected 2 registrations, got %d", count)
	}

	var regA, regB models.Registration
	db.Where("event = ?", "Event-A").First(&regA)
	db.Where("event = ?", "Event-B").First(&regB)

	if regA.Note != "Note A" {
		t.Errorf("expected Note A, got %s", regA.Note)
	}
	if regB.Note != "Note B" {
		t.Errorf("expected Note B, got %s", regB.Note)
	}
}
