package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"time"

	"github.com/gdg-garage/garage-trip-api/internal/config"
	"github.com/gdg-garage/garage-trip-api/internal/models"
	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/oauth2"
	"gorm.io/gorm"
)

const (
	DiscordAuthorizeEndpoint = "https://discord.com/api/oauth2/authorize"
	DiscordTokenEndpoint     = "https://discord.com/api/oauth2/token"
	DiscordUserAPI           = "https://discord.com/api/users/@me"
	DiscordUserGuildsAPI     = "https://discord.com/api/users/@me/guilds"
)

type AuthHandler struct {
	oauthConfig *oauth2.Config
	db          *gorm.DB
	cfg         *config.Config
}

func NewAuthHandler(cfg *config.Config, db *gorm.DB) *AuthHandler {
	return &AuthHandler{
		oauthConfig: &oauth2.Config{
			ClientID:     cfg.DiscordClientID,
			ClientSecret: cfg.DiscordClientSecret,
			RedirectURL:  cfg.DiscordRedirectURL,
			Scopes:       []string{"identify", "email", "guilds"},
			Endpoint: oauth2.Endpoint{
				AuthURL:  DiscordAuthorizeEndpoint,
				TokenURL: DiscordTokenEndpoint,
			},
		},
		db:  db,
		cfg: cfg,
	}
}

func (h *AuthHandler) HandleLogin(w http.ResponseWriter, r *http.Request) {
	url := h.oauthConfig.AuthCodeURL("state", oauth2.AccessTypeOnline)
	http.Redirect(w, r, url, http.StatusTemporaryRedirect)
}

func (h *AuthHandler) HandleCallback(w http.ResponseWriter, r *http.Request) {
	code := r.URL.Query().Get("code")
	if code == "" {
		http.Error(w, "Code not found", http.StatusBadRequest)
		return
	}

	token, err := h.oauthConfig.Exchange(context.Background(), code)
	if err != nil {
		http.Error(w, "Failed to exchange token", http.StatusInternalServerError)
		return
	}

	client := h.oauthConfig.Client(context.Background(), token)

	// Check Guild Membership
	if h.cfg.DiscordGuildID != "" {
		guildsResp, err := client.Get(DiscordUserGuildsAPI)
		if err != nil {
			http.Error(w, "Failed to get user guilds", http.StatusInternalServerError)
			return
		}
		defer guildsResp.Body.Close()

		var guilds []struct {
			ID string `json:"id"`
		}
		if err := json.NewDecoder(guildsResp.Body).Decode(&guilds); err != nil {
			http.Error(w, "Failed to decode user guilds", http.StatusInternalServerError)
			return
		}

		isMember := false
		for _, g := range guilds {
			if g.ID == h.cfg.DiscordGuildID {
				isMember = true
				break
			}
		}

		if !isMember {
			http.Error(w, "Access denied: You are not a member of the required guild.", http.StatusForbidden)
			return
		}
	}

	// Get User Info
	resp, err := client.Get(DiscordUserAPI)
	if err != nil {
		http.Error(w, "Failed to get user info", http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	var discordUser struct {
		ID       string `json:"id"`
		Username string `json:"username"`
		Email    string `json:"email"`
		Avatar   string `json:"avatar"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&discordUser); err != nil {
		http.Error(w, "Failed to decode user info", http.StatusInternalServerError)
		return
	}

	var user models.User
	if err := h.db.FirstOrInit(&user, models.User{DiscordID: discordUser.ID}).Error; err != nil {
		http.Error(w, "Database error", http.StatusInternalServerError)
		return
	}
	user.Username = discordUser.Username
	user.Email = discordUser.Email
	user.Avatar = discordUser.Avatar

	if err := h.db.Save(&user).Error; err != nil {
		http.Error(w, "Failed to save user", http.StatusInternalServerError)
		return
	}

	// Generate JWT
	jwtToken, err := h.GenerateToken(user.ID)
	if err != nil {
		http.Error(w, "Failed to generate token", http.StatusInternalServerError)
		return
	}

	// Set Cookie
	http.SetCookie(w, &http.Cookie{
		Name:     "auth_token",
		Value:    jwtToken,
		Expires:  time.Now().Add(24 * time.Hour),
		HttpOnly: true,
		Path:     "/",
		// Secure:   true, // Uncomment in production wi
		// th HTTPS
	})

	w.Write([]byte(fmt.Sprintf("Welcome %s! You are logged in.", user.Username)))
}

func (h *AuthHandler) GenerateToken(userID uint) (string, error) {
	claims := jwt.MapClaims{
		"user_id": userID,
		"exp":     time.Now().Add(24 * time.Hour).Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(h.cfg.JWTSecret))
}
