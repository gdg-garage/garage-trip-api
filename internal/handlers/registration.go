package handlers

import (
	"context"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/gdg-garage/garage-trip-api/internal/auth"
	"github.com/gdg-garage/garage-trip-api/internal/models"
	"gorm.io/gorm"
)

type RegistrationHandler struct {
	db *gorm.DB
}

func NewRegistrationHandler(db *gorm.DB) *RegistrationHandler {
	return &RegistrationHandler{db: db}
}

type RegistrationRequest struct {
	Body struct {
		ArrivalDate      time.Time `json:"arrival_date" doc:"Date of arrival"`
		DepartureDate    time.Time `json:"departure_date" doc:"Date of departure"`
		FoodRestrictions string    `json:"food_restrictions" doc:"Food restrictions or allergies"`
		ChildrenCount    int       `json:"children_count" doc:"Number of children joining"`
	}
}

type RegistrationResponse struct {
	Body struct {
		Message string `json:"message"`
	}
}

func (h *RegistrationHandler) HandleRegister(ctx context.Context, input *RegistrationRequest) (*RegistrationResponse, error) {
	// Get UserID from context
	userID, ok := ctx.Value(auth.UserIDKey).(uint)
	if !ok {
		return nil, huma.Error401Unauthorized("Unauthorized")
	}

	// Validate dates
	if input.Body.ArrivalDate.After(input.Body.DepartureDate) {
		return nil, huma.Error400BadRequest("Arrival date cannot be after departure date")
	}

	registration := models.Registration{
		UserID:           userID,
		ArrivalDate:      input.Body.ArrivalDate,
		DepartureDate:    input.Body.DepartureDate,
		FoodRestrictions: input.Body.FoodRestrictions,
		ChildrenCount:    input.Body.ChildrenCount,
	}

	if err := h.db.Create(&registration).Error; err != nil {
		return nil, huma.Error500InternalServerError("Failed to create registration")
	}

	res := &RegistrationResponse{}
	res.Body.Message = "Registration created successfully"
	return res, nil
}
