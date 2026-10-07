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

	// 1. Unauthenticated request with all events disabled -> should return 0 events
	res, err := eventHandler.HandleList(context.Background(), &ListEventsRequest{})
	if err != nil {
		t.Fatalf("HandleList failed: %v", err)
	}
	if len(res.Body.Events) != 0 {
		t.Fatalf("expected 0 events for unauthenticated user when all are disabled, got %d", len(res.Body.Events))
	}

	// 2. Enable one event -> unauthenticated user should now see that 1 enabled event
	db.Model(&models.Event{}).Where("code = ?", "g::t::7.0.0").Update("enabled", true)
	res, err = eventHandler.HandleList(context.Background(), &ListEventsRequest{})
	if err != nil {
		t.Fatalf("HandleList failed: %v", err)
	}
	if len(res.Body.Events) != 1 || res.Body.Events[0].Code != "g::t::7.0.0" {
		t.Fatalf("expected only g::t::7.0.0 to be returned, got %v", res.Body.Events)
	}

	// Re-disable g::t::7.0.0
	db.Model(&models.Event{}).Where("code = ?", "g::t::7.0.0").Update("enabled", false)

	// 3. Authenticated user who registered to past event g::t::6.9
	user := models.User{DiscordID: "user-1", Username: "Alice"}
	db.Create(&user)
	db.Create(&models.Registration{
		UserID: user.ID,
		Event:  "g::t::6.9",
	})
	token, _ := authHandler.GenerateToken(user.ID)

	res, err = eventHandler.HandleList(context.Background(), &ListEventsRequest{
		AuthInput: auth.AuthInput{Cookie: "auth_token=" + token},
	})
	if err != nil {
		t.Fatalf("HandleList failed: %v", err)
	}
	// Alice should see only past event g::t::6.9, but NOT locked active/future events (g::t::7.0.0 or g::t::8.0.0)
	if len(res.Body.Events) != 1 || res.Body.Events[0].Code != "g::t::6.9" {
		t.Fatalf("expected Alice to see only past registered event g::t::6.9, got %v", res.Body.Events)
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
