package auth

import (
	"context"
	"testing"

	"github.com/gdg-garage/garage-trip-api/internal/config"
	"github.com/gdg-garage/garage-trip-api/internal/models"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestHandleMe(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to connect database: %v", err)
	}

	db.AutoMigrate(&models.User{}, &models.Registration{})

	user := models.User{
		DiscordID: "123456",
		Username:  "testuser",
		Email:     "test@example.com",
		Avatar:    "avatar_url",
	}
	db.Create(&user)

	cfg := &config.Config{JWTSecret: "test-secret"}
	handler := NewAuthHandler(cfg, db, nil)

	t.Run("Authenticated", func(t *testing.T) {
		token, _ := handler.GenerateToken(user.ID)
		input := &AuthInput{
			Cookie: "auth_token=" + token,
		}
		resp, err := handler.HandleMe(context.Background(), input)
		if err != nil {
			t.Fatalf("HandleMe returned error: %v", err)
		}

		if resp.Body.Username != user.Username {
			t.Errorf("expected username %s, got %s", user.Username, resp.Body.Username)
		}
		if resp.Body.Email != user.Email {
			t.Errorf("expected email %s, got %s", user.Email, resp.Body.Email)
		}

		// Verify Paid status (default false in this test setup)
		if resp.Body.Paid != false {
			t.Errorf("expected paid false, got %v", resp.Body.Paid)
		}

		// Verify Registrations is nil or empty initially
		if len(resp.Body.Registrations) != 0 {
			t.Errorf("expected registrations empty, got %d", len(resp.Body.Registrations))
		}

		// Add a registration and check again
		db.Create(&models.Registration{
			UserID: user.ID,
			Event:  "test-event",
			RegistrationFields: models.RegistrationFields{
				ChildrenCount: 2,
			},
		})

		resp, err = handler.HandleMe(context.Background(), input)
		if err != nil {
			t.Fatalf("HandleMe returned error after registration: %v", err)
		}

		if len(resp.Body.Registrations) != 1 {
			t.Fatal("expected 1 registration")
		}
		if resp.Body.Registrations[0].ChildrenCount != 2 {
			t.Errorf("expected children count 2, got %d", resp.Body.Registrations[0].ChildrenCount)
		}
	})

	t.Run("Unauthenticated", func(t *testing.T) {
		input := &AuthInput{}
		_, err := handler.HandleMe(context.Background(), input)
		if err == nil {
			t.Fatal("expected error for unauthenticated request, got nil")
		}
	})
}
