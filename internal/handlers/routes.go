package handlers

import (
	"net/http"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humachi"
	"github.com/gdg-garage/garage-trip-api/internal/auth"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

func RegisterRoutes(r *chi.Mux, authHandler *auth.AuthHandler, registrationHandler *RegistrationHandler) {
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	// Initialize Huma API
	config := huma.DefaultConfig("Garage Trip API", "1.0.0")
	api := humachi.New(r, config)

	// Public routes
	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("OK"))
	})

	// Auth routes
	huma.Get(api, "/auth/discord/login", authHandler.HandleLogin)
	huma.Get(api, "/auth/discord/callback", authHandler.HandleCallback)

	// Protected routes
	r.Group(func(r chi.Router) {
		r.Use(authHandler.JWTMiddleware)
		huma.Post(api, "/register", registrationHandler.HandleRegister)
	})
}
