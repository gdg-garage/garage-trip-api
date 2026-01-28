package handlers

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/gdg-garage/garage-trip-api/internal/auth"
	"github.com/gdg-garage/garage-trip-api/internal/models"
	"gorm.io/gorm"
)

type APIKeyHandler struct {
	db          *gorm.DB
	authHandler *auth.AuthHandler
}

func NewAPIKeyHandler(db *gorm.DB, authHandler *auth.AuthHandler) *APIKeyHandler {
	return &APIKeyHandler{db: db, authHandler: authHandler}
}

type CreateAPIKeyInput struct {
	auth.AuthInput
	Body struct {
		Name      string     `json:"name"`
		ExpiresAt *time.Time `json:"expires_at"`
	}
}

type APIKeyResponse struct {
	ID         uint       `json:"id"`
	Name       string     `json:"name"`
	Key        string     `json:"key"`
	CreatedAt  time.Time  `json:"created_at"`
	ExpiresAt  *time.Time `json:"expires_at"`
	LastUsedAt *time.Time `json:"last_used_at"`
}

type CreateAPIKeyOutput struct {
	Body APIKeyResponse
}

func (h *APIKeyHandler) HandleCreate(ctx context.Context, input *CreateAPIKeyInput) (*CreateAPIKeyOutput, error) {
	userID, err := h.authHandler.Authorize(ctx, input.Cookie)
	if err != nil {
		return nil, err
	}

	keyBytes := make([]byte, 32)
	if _, err := rand.Read(keyBytes); err != nil {
		return nil, huma.Error500InternalServerError("Failed to generate key")
	}
	key := hex.EncodeToString(keyBytes)

	apiKey := models.APIKey{
		UserID:    userID,
		Key:       key,
		Name:      input.Body.Name,
		ExpiresAt: input.Body.ExpiresAt,
	}

	if err := h.db.Create(&apiKey).Error; err != nil {
		return nil, huma.Error500InternalServerError("Failed to create API key")
	}

	return &CreateAPIKeyOutput{
		Body: APIKeyResponse{
			ID:         apiKey.ID,
			Name:       apiKey.Name,
			Key:        apiKey.Key,
			CreatedAt:  apiKey.CreatedAt,
			ExpiresAt:  apiKey.ExpiresAt,
			LastUsedAt: apiKey.LastUsedAt,
		},
	}, nil
}

type ListAPIKeysInput struct {
	auth.AuthInput
}

type ListAPIKeysOutput struct {
	Body []APIKeyResponse
}

func (h *APIKeyHandler) HandleList(ctx context.Context, input *ListAPIKeysInput) (*ListAPIKeysOutput, error) {
	userID, err := h.authHandler.Authorize(ctx, input.Cookie)
	if err != nil {
		return nil, err
	}

	var apiKeys []models.APIKey
	if err := h.db.Where("user_id = ?", userID).Find(&apiKeys).Error; err != nil {
		return nil, huma.Error500InternalServerError("Failed to list API keys")
	}

	var response []APIKeyResponse
	for _, k := range apiKeys {
		maskedKey := k.Key
		if len(k.Key) > 4 {
			maskedKey = "..." + k.Key[len(k.Key)-4:]
		}
		response = append(response, APIKeyResponse{
			ID:         k.ID,
			Name:       k.Name,
			Key:        maskedKey,
			CreatedAt:  k.CreatedAt,
			ExpiresAt:  k.ExpiresAt,
			LastUsedAt: k.LastUsedAt,
		})
	}

	return &ListAPIKeysOutput{Body: response}, nil
}

type DeleteAPIKeyInput struct {
	auth.AuthInput
	ID uint `path:"id"`
}

func (h *APIKeyHandler) HandleDelete(ctx context.Context, input *DeleteAPIKeyInput) (*struct{}, error) {
	userID, err := h.authHandler.Authorize(ctx, input.Cookie)
	if err != nil {
		return nil, err
	}

	if err := h.db.Where("id = ? AND user_id = ?", input.ID, userID).Delete(&models.APIKey{}).Error; err != nil {
		return nil, huma.Error500InternalServerError("Failed to delete API key")
	}

	return nil, nil
}
