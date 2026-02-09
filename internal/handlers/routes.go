package handlers

import (
	"net/http"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humachi"
	"github.com/gdg-garage/garage-trip-api/internal/auth"
	"github.com/gdg-garage/garage-trip-api/internal/config"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

func RegisterRoutes(r *chi.Mux, cfg *config.Config, authHandler *auth.AuthHandler, registrationHandler *RegistrationHandler, achievementHandler *AchievementHandler, apiKeyHandler *APIKeyHandler) {
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	if cfg.EnableCORS {
		r.Use(CORSMiddleware)
	}

	// Initialize Huma API
	config := huma.DefaultConfig("Garage Trip API", "1.0.0")
	config.Components.SecuritySchemes = map[string]*huma.SecurityScheme{
		"cookieAuth": {
			Type: "apiKey",
			In:   "cookie",
			Name: "auth_token",
		},
		"apiKeyAuth": {
			Type: "apiKey",
			In:   "header",
			Name: "X-API-KEY",
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
		r.Use(authHandler.AuthMiddleware)

		// User & API Key agnostic routes (secured by AuthMiddleware)
		authSecurity := []map[string][]string{
			{"cookieAuth": {}},
			{"apiKeyAuth": {}},
		}

		huma.Get(api, "/me", authHandler.HandleMe, func(o *huma.Operation) {
			o.Security = authSecurity
		})
		huma.Post(api, "/register", registrationHandler.HandleRegister, func(o *huma.Operation) {
			o.Security = authSecurity
		})
		huma.Get(api, "/history", registrationHandler.HandleHistory, func(o *huma.Operation) {
			o.Security = authSecurity
		})
		huma.Get(api, "/registrations", registrationHandler.HandleListRegistrations, func(o *huma.Operation) {
			o.Summary = "List all registrations"
			o.Description = "Returns a list of all registrations. Restricted to users with the 'g::t::orgs' role."
			o.Security = authSecurity
		})

		huma.Post(api, "/achievements/create", achievementHandler.HandleCreateAchievement, func(o *huma.Operation) {
			o.Summary = "Create a new achievement"
			o.Description = "Creates a new achievement and a corresponding Discord role. Restricted to orgs."
			o.Security = authSecurity
		})
		huma.Post(api, "/achievements/grant", achievementHandler.HandleGrantAchievement, func(o *huma.Operation) {
			o.Summary = "Grant an achievement"
			o.Description = "Grants an achievement to a user."
			o.Security = authSecurity
		})

		// API Key Management Routes
		huma.Post(api, "/api-keys", apiKeyHandler.HandleCreate, func(o *huma.Operation) {
			o.Summary = "Create API Key"
			o.Security = authSecurity
		})
		huma.Get(api, "/api-keys", apiKeyHandler.HandleList, func(o *huma.Operation) {
			o.Summary = "List API Keys"
			o.Security = authSecurity
		})
		huma.Delete(api, "/api-keys/{id}", apiKeyHandler.HandleDelete, func(o *huma.Operation) {
			o.Summary = "Delete API Key"
			o.Security = authSecurity
		})
	})
}

func CORSMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		// Allow any origin that matches localhost or our production domains
		// In a production app, you might want to be more restrictive here
		if origin != "" {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS, PUT, DELETE")
			w.Header().Set("Access-Control-Allow-Headers", "Accept, Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, X-API-KEY")
			w.Header().Set("Access-Control-Allow-Credentials", "true")
		}

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}
