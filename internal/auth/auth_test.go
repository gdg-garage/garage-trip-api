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

	db.AutoMigrate(&models.User{})

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
	})

	t.Run("Unauthenticated", func(t *testing.T) {
		input := &AuthInput{}
		_, err := handler.HandleMe(context.Background(), input)
		if err == nil {
			t.Fatal("expected error for unauthenticated request, got nil")
		}
	})
}
