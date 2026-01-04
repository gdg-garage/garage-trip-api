package handlers

import (
	"net/http"

	"github.com/gdg-garage/garage-trip-api/internal/auth"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

func RegisterRoutes(r *chi.Mux, authHandler *auth.AuthHandler) {
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("OK"))
	})

	r.Route("/auth", func(r chi.Router) {
		r.Route("/discord", func(r chi.Router) {
			r.Get("/login", authHandler.HandleLogin)
			r.Get("/callback", authHandler.HandleCallback)
		})
	})
}
