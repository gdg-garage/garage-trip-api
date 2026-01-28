package auth

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gdg-garage/garage-trip-api/internal/config"
	"github.com/golang-jwt/jwt/v5"
)

func TestJWTMiddleware_SlidingSession(t *testing.T) {
	cfg := &config.Config{JWTSecret: "test-secret"}
	handler := NewAuthHandler(cfg, nil, nil)

	t.Run("TokenRenewed", func(t *testing.T) {
		// Create a token that expires in 11 hours (less than TokenDuration/2 = 12 hours)
		userID := uint(1)
		claims := jwt.MapClaims{
			"user_id": userID,
			"exp":     time.Now().Add(11 * time.Hour).Unix(),
		}
		token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
		tokenString, _ := token.SignedString([]byte(cfg.JWTSecret))

		req, _ := http.NewRequest("GET", "/", nil)
		req.AddCookie(&http.Cookie{Name: "auth_token", Value: tokenString})
		rr := httptest.NewRecorder()

		nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		})

		middleware := handler.AuthMiddleware(nextHandler)
		middleware.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("expected status OK, got %v", rr.Code)
		}

		// Check if a new cookie was set
		cookies := rr.Result().Cookies()
		found := false
		for _, c := range cookies {
			if c.Name == "auth_token" {
				found = true
				if c.Value == tokenString {
					t.Errorf("expected new token value, but got the old one")
				}
				break
			}
		}
		if !found {
			t.Errorf("expected new auth_token cookie to be set")
		}
	})

	t.Run("TokenNotRenewed", func(t *testing.T) {
		// Create a token that expires in 13 hours (more than TokenDuration/2 = 12 hours)
		userID := uint(1)
		claims := jwt.MapClaims{
			"user_id": userID,
			"exp":     time.Now().Add(13 * time.Hour).Unix(),
		}
		token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
		tokenString, _ := token.SignedString([]byte(cfg.JWTSecret))

		req, _ := http.NewRequest("GET", "/", nil)
		req.AddCookie(&http.Cookie{Name: "auth_token", Value: tokenString})
		rr := httptest.NewRecorder()

		nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		})

		middleware := handler.AuthMiddleware(nextHandler)
		middleware.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("expected status OK, got %v", rr.Code)
		}

		// Check that no NEW auth_token cookie was set
		cookies := rr.Result().Cookies()
		for _, c := range cookies {
			if c.Name == "auth_token" {
				t.Errorf("did not expect a new auth_token cookie to be set")
			}
		}
	})
}
