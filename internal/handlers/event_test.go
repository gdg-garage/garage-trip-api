package handlers

import (
	"context"
	"testing"
	"time"

	"github.com/gdg-garage/garage-trip-api/internal/auth"
	"github.com/gdg-garage/garage-trip-api/internal/config"
	"github.com/gdg-garage/garage-trip-api/internal/database"
	"github.com/gdg-garage/garage-trip-api/internal/models"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupTestDB(t *testing.T) *gorm.DB {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to connect database: %v", err)
	}

	err = db.AutoMigrate(
		&models.User{},
		&models.Registration{},
		&models.RegistrationHistory{},
		&models.Achievement{},
		&models.AchievementGrant{},
		&models.APIKey{},
		&models.Event{},
	)
	if err != nil {
		t.Fatalf("failed to auto migrate: %v", err)
	}

	database.SeedDefaultEvents(db)
	return db
}

func TestListEvents(t *testing.T) {
	db := setupTestDB(t)
	cfg := &config.Config{JWTSecret: "test-secret", OrgRole: "orgs"}
	authHandler := auth.NewAuthHandler(cfg, db, nil)
	eventHandler := NewEventHandler(db, authHandler, cfg)

	res, err := eventHandler.HandleList(context.Background(), &struct{}{})
	if err != nil {
		t.Fatalf("HandleList failed: %v", err)
	}

	if len(res.Body.Events) < 3 {
		t.Fatalf("expected at least 3 seeded events, got %d", len(res.Body.Events))
	}

	// Verify GT7 is present and disabled
	var gt7 *EventItem
	for i := range res.Body.Events {
		if res.Body.Events[i].Code == "g::t::7.0.0" {
			gt7 = &res.Body.Events[i]
			break
		}
	}
	if gt7 == nil {
		t.Fatal("g::t::7.0.0 not found in events")
	}
	if gt7.Enabled {
		t.Errorf("expected g::t::7.0.0 to be disabled by default, got enabled")
	}
}

func TestEventRegistration_EnabledCheck(t *testing.T) {
	db := setupTestDB(t)
	cfg := &config.Config{JWTSecret: "test-secret", OrgRole: "orgs"}
	authHandler := auth.NewAuthHandler(cfg, db, nil)
	regHandler := NewRegistrationHandler(db, nil, authHandler, cfg)

	// Create user
	user := models.User{DiscordID: "test-user-1", Username: "Tester"}
	db.Create(&user)
	token, _ := authHandler.GenerateToken(user.ID)

	// Attempt registration for disabled event (g::t::7.0.0) -> should fail
	req := &RegistrationRequest{}
	req.Cookie = "auth_token=" + token
	req.Body.Event = "g::t::7.0.0"
	req.Body.ArrivalDate = time.Now()
	req.Body.DepartureDate = time.Now().Add(24 * time.Hour)

	_, err := regHandler.HandleRegister(context.Background(), req)
	if err == nil {
		t.Fatal("expected error registering for disabled event, got nil")
	}

	// Enable g::t::7.0.0 in DB
	db.Model(&models.Event{}).Where("code = ?", "g::t::7.0.0").Update("enabled", true)

	// Attempt registration again -> should succeed
	_, err = regHandler.HandleRegister(context.Background(), req)
	if err != nil {
		t.Fatalf("expected registration to succeed when event enabled in DB, got: %v", err)
	}

	// Disable it again -> registration should fail
	db.Model(&models.Event{}).Where("code = ?", "g::t::7.0.0").Update("enabled", false)
	_, err = regHandler.HandleRegister(context.Background(), req)
	if err == nil {
		t.Fatal("expected error registering for re-disabled event, got nil")
	}
}
