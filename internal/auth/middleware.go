package auth

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/gdg-garage/garage-trip-api/internal/models"
	"github.com/golang-jwt/jwt/v5"
)

type contextKey string

const UserIDKey contextKey = "user_id"

func (h *AuthHandler) AuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 1. Check for API Key Header
		apiKey := r.Header.Get("X-API-KEY")
		if apiKey != "" {
			var keyModel models.APIKey
			if err := h.db.Where("key = ?", apiKey).First(&keyModel).Error; err == nil {
				if keyModel.ExpiresAt != nil && time.Now().After(*keyModel.ExpiresAt) {
					http.Error(w, "Unauthorized: API Key expired", http.StatusUnauthorized)
					return
				}

				h.db.Model(&keyModel).Update("last_used_at", time.Now())

				ctx := context.WithValue(r.Context(), UserIDKey, keyModel.UserID)
				next.ServeHTTP(w, r.WithContext(ctx))
				return
			}
		}

		// 2. Fallback to JWT Cookie
		cookie, err := r.Cookie("auth_token")
		if err != nil {
			if err == http.ErrNoCookie {
				http.Error(w, "Unauthorized: No token found", http.StatusUnauthorized)
				return
			}
			http.Error(w, "Bad Request", http.StatusBadRequest)
			return
		}

		tokenString := cookie.Value
		token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
			if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
			}
			return []byte(h.cfg.JWTSecret), nil
		})

		if err != nil || !token.Valid {
			http.Error(w, "Unauthorized: Invalid token", http.StatusUnauthorized)
			return
		}

		if claims, ok := token.Claims.(jwt.MapClaims); ok {
			userIDFloat, ok := claims["user_id"].(float64)
			if !ok {
				http.Error(w, "Unauthorized: Invalid token claims", http.StatusUnauthorized)
				return
			}
			userID := uint(userIDFloat)

			// Sliding session: refresh token if it's more than halfway through its duration
			if exp, ok := claims["exp"].(float64); ok {
				remaining := time.Until(time.Unix(int64(exp), 0))
				if remaining < TokenDuration/2 {
					newToken, err := h.GenerateToken(userID)
					if err == nil {
						cookie := &http.Cookie{
							Name:     "auth_token",
							Value:    newToken,
							Expires:  time.Now().Add(TokenDuration),
							HttpOnly: true,
							Path:     "/",
						}
						http.SetCookie(w, cookie)
					}
				}
			}

			ctx := context.WithValue(r.Context(), UserIDKey, userID)
			next.ServeHTTP(w, r.WithContext(ctx))
		} else {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
		}
	})
}
