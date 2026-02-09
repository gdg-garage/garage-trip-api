package main

import (
	"fmt"
	"log"
	"net/http"

	"github.com/bwmarrin/discordgo"
	"github.com/gdg-garage/garage-trip-api/internal/auth"
	"github.com/gdg-garage/garage-trip-api/internal/config"
	"github.com/gdg-garage/garage-trip-api/internal/database"
	"github.com/gdg-garage/garage-trip-api/internal/handlers"
	"github.com/gdg-garage/garage-trip-api/internal/notifier"
	"github.com/go-chi/chi/v5"
)

func main() {
	// Load Configuration
	cfg := config.LoadConfig()

	// Connect to Database
	db := database.Connect(cfg)

	// Initialize Notifier
	var discordSession *discordgo.Session
	var err error
	if cfg.DiscordBotToken != "" {
		discordSession, err = discordgo.New("Bot " + cfg.DiscordBotToken)
		if err != nil {
			log.Fatalf("Failed to create Discord session: %v", err)
		}
	}

	var discordNotifier *notifier.DiscordNotifier
	if discordSession != nil && cfg.DiscordNotificationsChannelID != "" {
		discordNotifier = notifier.NewDiscordNotifier(discordSession, cfg.DiscordNotificationsChannelID, cfg.DiscordGuildID, cfg.AchievementPrefix)
	}

	authHandler := auth.NewAuthHandler(cfg, db, discordSession)
	registrationHandler := handlers.NewRegistrationHandler(db, discordNotifier, authHandler)
	achievementHandler := handlers.NewAchievementHandler(db, discordNotifier, authHandler)
	apiKeyHandler := handlers.NewAPIKeyHandler(db, authHandler)

	// Initialize Router
	r := chi.NewRouter()

	// Register Routes
	handlers.RegisterRoutes(r, cfg, authHandler, registrationHandler, achievementHandler, apiKeyHandler)

	// Start Server
	log.Printf("Starting server on port %s", cfg.Port)
	if err := http.ListenAndServe(fmt.Sprintf(":%s", cfg.Port), r); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
