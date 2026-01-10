package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"time"

	"github.com/danielgtaylor/huma/v2"
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

type LoginResponse struct {
	Location string `header:"Location"`
}

func (h *AuthHandler) HandleLogin(ctx context.Context, input *struct{}) (*LoginResponse, error) {
	url := h.oauthConfig.AuthCodeURL("state", oauth2.AccessTypeOnline)
	return &LoginResponse{Location: url}, nil
}

type CallbackInput struct {
	Code string `query:"code" doc:"OAuth2 callback code"`
}

type CallbackResponse struct {
	SetCookie string `header:"Set-Cookie"`
	Body      struct {
		Message string `json:"message"`
	}
}

func (h *AuthHandler) HandleCallback(ctx context.Context, input *CallbackInput) (*CallbackResponse, error) {
	if input.Code == "" {
		return nil, huma.Error400BadRequest("Code not found")
	}

	token, err := h.oauthConfig.Exchange(ctx, input.Code)
	if err != nil {
		return nil, huma.Error500InternalServerError("Failed to exchange token")
	}

	client := h.oauthConfig.Client(ctx, token)

	// Check Guild Membership
	if h.cfg.DiscordGuildID != "" {
		guildsResp, err := client.Get(DiscordUserGuildsAPI)
		if err != nil {
			return nil, huma.Error500InternalServerError("Failed to get user guilds")
		}
		defer guildsResp.Body.Close()

		var guilds []struct {
			ID string `json:"id"`
		}
		if err := json.NewDecoder(guildsResp.Body).Decode(&guilds); err != nil {
			return nil, huma.Error500InternalServerError("Failed to decode user guilds")
		}

		isMember := false
		for _, g := range guilds {
			if g.ID == h.cfg.DiscordGuildID {
				isMember = true
				break
			}
		}

		if !isMember {
			return nil, huma.Error403Forbidden(fmt.Sprintf("Access denied: You are not a member of the required Discord guild: %s", h.cfg.DiscordGuildID))
		}
	}

	// Get User Info
	resp, err := client.Get(DiscordUserAPI)
	if err != nil {
		return nil, huma.Error500InternalServerError("Failed to get user info")
	}
	defer resp.Body.Close()

	var discordUser struct {
		ID       string `json:"id"`
		Username string `json:"username"`
		Email    string `json:"email"`
		Avatar   string `json:"avatar"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&discordUser); err != nil {
		return nil, huma.Error500InternalServerError("Failed to decode user info")
	}

	var user models.User
	if err := h.db.FirstOrInit(&user, models.User{DiscordID: discordUser.ID}).Error; err != nil {
		return nil, huma.Error500InternalServerError("Database error")
	}
	user.Username = discordUser.Username
	user.Email = discordUser.Email
	user.Avatar = discordUser.Avatar

	if err := h.db.Save(&user).Error; err != nil {
		return nil, huma.Error500InternalServerError("Failed to save user")
	}

	// Generate JWT
	jwtToken, err := h.GenerateToken(user.ID)
	if err != nil {
		return nil, huma.Error500InternalServerError("Failed to generate token")
	}

	cookie := &http.Cookie{
		Name:     "auth_token",
		Value:    jwtToken,
		Expires:  time.Now().Add(24 * time.Hour),
		HttpOnly: true,
		Path:     "/",
	}

	res := &CallbackResponse{}
	res.Body.Message = fmt.Sprintf("Welcome %s! You are logged in.", user.Username)
	res.SetCookie = cookie.String()

	return res, nil
}

func (h *AuthHandler) GenerateToken(userID uint) (string, error) {
	claims := jwt.MapClaims{
		"user_id": userID,
		"exp":     time.Now().Add(24 * time.Hour).Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(h.cfg.JWTSecret))
}
