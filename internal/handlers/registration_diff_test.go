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

func TestHandleHistory_Diff(t *testing.T) {
	// Setup in-memory DB
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to connect database: %v", err)
	}

	db.AutoMigrate(&models.Registration{}, &models.User{}, &models.RegistrationHistory{})

	// Create user
	user := models.User{DiscordID: "diff-user"}
	db.Create(&user)

	authHandler := auth.NewAuthHandler(&config.Config{JWTSecret: "test-secret"}, db, nil)
	handler := NewRegistrationHandler(db, nil, authHandler)

	token, _ := authHandler.GenerateToken(user.ID)
	authCookie := "auth_token=" + token

	// Create 3 history entries manually to control timestamps and values
	baseTime := time.Now()

	// Entry 1 (Oldest)
	e1 := models.RegistrationHistory{
		UserID: user.ID,
		Event:  "ev1",
		RegistrationFields: models.RegistrationFields{
			ChildrenCount:    1,
			Note:             "Note 1",
			FoodRestrictions: "None",
		},
		Model: gorm.Model{CreatedAt: baseTime},
	}
	db.Create(&e1)

	// Entry 2 (Middle) - Changed Note
	e2 := models.RegistrationHistory{
		UserID: user.ID,
		Event:  "ev1",
		RegistrationFields: models.RegistrationFields{
			ChildrenCount:    1,        // Same
			Note:             "Note 2", // Changed
			FoodRestrictions: "None",   // Same
		},
		Model: gorm.Model{CreatedAt: baseTime.Add(1 * time.Minute)},
	}
	db.Create(&e2)

	// Entry 3 (Newest) - Changed ChildrenCount
	e3 := models.RegistrationHistory{
		UserID: user.ID,
		Event:  "ev1",
		RegistrationFields: models.RegistrationFields{
			ChildrenCount:    5,        // Changed
			Note:             "Note 2", // Same as prev
			FoodRestrictions: "None",   // Same
		},
		Model: gorm.Model{CreatedAt: baseTime.Add(2 * time.Minute)},
	}
	db.Create(&e3)

	// Test case 1: Diff = true (default)
	t.Run("DiffEnabled", func(t *testing.T) {
		req := HistoryRequest{}
		req.Cookie = authCookie
		req.Diff = true

		resp, err := handler.HandleHistory(context.Background(), &req)
		if err != nil {
			t.Fatalf("HandleHistory failed: %v", err)
		}

		if len(resp.Body.History) != 3 {
			t.Fatalf("expected 3 history items, got %d", len(resp.Body.History))
		}

		// Newest (Index 0) - Should compare with Middle (Index 1)
		// e3 vs e2: ChildrenCount changed (5 vs 1), Note same ("Note 2"), FoodRestrictions same ("None")
		h0 := resp.Body.History[0]
		if h0.RegistrationFields.ChildrenCount == nil || *h0.RegistrationFields.ChildrenCount != 5 {
			t.Error("expected h0 ChildrenCount to be 5")
		}
		if h0.RegistrationFields.Note != nil {
			t.Errorf("expected h0 Note to be nil (unchanged), got %v", *h0.RegistrationFields.Note)
		}
		if h0.RegistrationFields.FoodRestrictions != nil {
			t.Error("expected h0 FoodRestrictions to be nil (unchanged)")
		}

		// Middle (Index 1) - Should compare with Oldest (Index 2)
		// e2 vs e1: Note changed ("Note 2" vs "Note 1"), others same
		h1 := resp.Body.History[1]
		if h1.RegistrationFields.Note == nil || *h1.RegistrationFields.Note != "Note 2" {
			t.Error("expected h1 Note to be 'Note 2'")
		}
		if h1.RegistrationFields.ChildrenCount != nil {
			t.Error("expected h1 ChildrenCount to be nil (unchanged)")
		}

		// Oldest (Index 2) - Should be full dump (no previous)
		h2 := resp.Body.History[2]
		if h2.RegistrationFields.ChildrenCount == nil || *h2.RegistrationFields.ChildrenCount != 1 {
			t.Error("expected h2 ChildrenCount to be 1")
		}
		if h2.RegistrationFields.Note == nil || *h2.RegistrationFields.Note != "Note 1" {
			t.Error("expected h2 Note to be 'Note 1'")
		}
	})

	// Test case 2: Diff = false
	t.Run("DiffDisabled", func(t *testing.T) {
		req := HistoryRequest{}
		req.Cookie = authCookie
		req.Diff = false

		resp, err := handler.HandleHistory(context.Background(), &req)
		if err != nil {
			t.Fatalf("HandleHistory failed: %v", err)
		}

		// All fields should be present
		h0 := resp.Body.History[0]
		if h0.RegistrationFields.ChildrenCount == nil || *h0.RegistrationFields.ChildrenCount != 5 {
			t.Error("expected h0 ChildrenCount 5")
		}
		if h0.RegistrationFields.Note == nil || *h0.RegistrationFields.Note != "Note 2" {
			t.Error("expected h0 Note 'Note 2'")
		}

		h1 := resp.Body.History[1]
		if h1.RegistrationFields.ChildrenCount == nil || *h1.RegistrationFields.ChildrenCount != 1 {
			t.Error("expected h1 ChildrenCount 1")
		}
	})
}
