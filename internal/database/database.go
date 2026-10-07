package database

import (
	"log"
	"time"

	"github.com/gdg-garage/garage-trip-api/internal/config"
	"github.com/gdg-garage/garage-trip-api/internal/models"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func Connect(cfg *config.Config) *gorm.DB {
	db, err := gorm.Open(sqlite.Open(cfg.DatabasePath), &gorm.Config{})
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}

	// Auto Migrate
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
		log.Fatalf("Failed to auto migrate: %v", err)
	}

	SeedDefaultEvents(db)

	return db
}

func parseTime(s string) *time.Time {
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return nil
	}
	return &t
}

func SeedDefaultEvents(db *gorm.DB) {
	defaultEvents := []models.Event{
		{
			Code:        "g::t::7.0.0",
			Name:        "Garage Trip 7.0.0",
			StartDate:   parseTime("2026-09-12T17:00:00Z"),
			EndDate:     parseTime("2026-09-19T10:00:00Z"),
			Location:    "Nové Město na Moravě",
			Description: "Seventh iteration of the annual GDG Garage coding & gaming retreat.",
			Enabled:     false,
			Status:      "future",
		},
		{
			Code:        "g::t::6.9",
			Name:        "Garage Trip 6.9",
			StartDate:   parseTime("2025-09-20T17:00:00Z"),
			EndDate:     parseTime("2025-09-27T10:00:00Z"),
			Location:    "Nový Svět",
			Description: "Sixth-and-a-half edition in Nový Svět.",
			Enabled:     false,
			Status:      "past",
		},
		{
			Code:        "g::t::8.0.0",
			Name:        "Garage Trip 8.0.0",
			StartDate:   parseTime("2027-09-11T17:00:00Z"),
			EndDate:     parseTime("2027-09-18T10:00:00Z"),
			Description: "Eighth edition of Garage Trip. Coming soon!",
			Enabled:     false,
			Status:      "future",
		},
	}

	for _, e := range defaultEvents {
		var existing models.Event
		if err := db.Where("code = ?", e.Code).First(&existing).Error; err != nil {
			db.Create(&e)
		}
	}

	// Migrate any existing 'active' events to 'future'
	db.Model(&models.Event{}).Where("status = ?", "active").Update("status", "future")

	// Also ensure any events currently in registrations table are present
	var regEvents []string
	db.Model(&models.Registration{}).Distinct("event").Pluck("event", &regEvents)
	for _, code := range regEvents {
		if code == "" {
			continue
		}
		var count int64
		db.Model(&models.Event{}).Where("code = ?", code).Count(&count)
		if count == 0 {
			db.Create(&models.Event{
				Code:    code,
				Name:    code,
				Enabled: false,
				Status:  "past",
			})
		}
	}
}
