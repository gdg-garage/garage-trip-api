package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

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
	Status   int    `header:"-" status:"307"`
	Location string `header:"Location"`
}

func (h *AuthHandler) HandleLogin(ctx context.Context, input *struct{}) (*LoginResponse, error) {
	url := h.oauthConfig.AuthCodeURL("state", oauth2.AccessTypeOnline)
	fmt.Printf("Login URL: %s\n", url)
	return &LoginResponse{
		Status:   307,
		Location: url,
	}, nil
}

type CallbackInput struct {
	Code string `query:"code" doc:"OAuth2 callback code"`
}

type MeResponse struct {
	Body struct {
		Username string `json:"username"`
		Email    string `json:"email"`
	}
}

type AuthInput struct {
	Cookie string `header:"Cookie" doc:"Authentication cookie containing the auth_token JWT" example:"auth_token=..."`
}

func (h *AuthHandler) HandleMe(ctx context.Context, input *AuthInput) (*MeResponse, error) {
	userID, err := h.Authorize(input.Cookie)
	if err != nil {
		return nil, err
	}

	var user models.User
	if err := h.db.First(&user, userID).Error; err != nil {
		return nil, huma.Error404NotFound("User not found")
	}

	res := &MeResponse{}
	res.Body.Username = user.Username
	res.Body.Email = user.Email

	return res, nil
}

// Authorize parses the auth_token from a Cookie header string
func (h *AuthHandler) Authorize(cookieHeader string) (uint, error) {
	if cookieHeader == "" {
		return 0, huma.Error401Unauthorized("Unauthorized: No cookies found")
	}

	cookieValue := ""
	parts := strings.Split(cookieHeader, ";")
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if strings.HasPrefix(p, "auth_token=") {
			cookieValue = strings.TrimPrefix(p, "auth_token=")
			break
		}
	}

	if cookieValue == "" {
		return 0, huma.Error401Unauthorized("Unauthorized: No token found")
	}

	token, err := jwt.Parse(cookieValue, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(h.cfg.JWTSecret), nil
	})

	if err != nil || !token.Valid {
		return 0, huma.Error401Unauthorized("Unauthorized: Invalid token")
	}

	if claims, ok := token.Claims.(jwt.MapClaims); ok {
		if userIDFloat, ok := claims["user_id"].(float64); ok {
			return uint(userIDFloat), nil
		}
	}

	return 0, huma.Error401Unauthorized("Unauthorized")
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

	fmt.Printf("User logged in: %s (Discord ID: %s)\n", user.Username, user.DiscordID)

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
