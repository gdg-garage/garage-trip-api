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
	config.Components.SecuritySchemes = map[string]*huma.SecurityScheme{
		"cookieAuth": {
			Type: "apiKey",
			In:   "cookie",
			Name: "auth_token",
		},
	}
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
		huma.Get(api, "/me", authHandler.HandleMe, func(o *huma.Operation) {
			o.Security = []map[string][]string{{"cookieAuth": {}}}
		})
		huma.Post(api, "/register", registrationHandler.HandleRegister, func(o *huma.Operation) {
			o.Security = []map[string][]string{{"cookieAuth": {}}}
		})
		huma.Get(api, "/history", registrationHandler.HandleHistory, func(o *huma.Operation) {
			o.Security = []map[string][]string{{"cookieAuth": {}}}
		})
	})
}
