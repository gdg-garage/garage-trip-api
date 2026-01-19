package main

import (
	"fmt"
	"log"
	"net/http"

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

	// Initialize Auth Handler
	// Initialize Handlers
	discordNotifier, err := notifier.NewDiscordNotifier(cfg)
	if err != nil {
		log.Printf("Discord notifier not initialized: %v", err)
	}

	authHandler := auth.NewAuthHandler(cfg, db)
	registrationHandler := handlers.NewRegistrationHandler(db, discordNotifier, authHandler)

	// Initialize Router
	r := chi.NewRouter()

	// Register Routes
	handlers.RegisterRoutes(r, authHandler, registrationHandler)

	// Start Server
	log.Printf("Starting server on port %s", cfg.Port)
	if err := http.ListenAndServe(fmt.Sprintf(":%s", cfg.Port), r); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
