package handlers

import (
	"context"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/gdg-garage/garage-trip-api/internal/auth"
	"github.com/gdg-garage/garage-trip-api/internal/models"
	"github.com/gdg-garage/garage-trip-api/internal/notifier"
	"gorm.io/gorm"
)

type RegistrationHandler struct {
	db          *gorm.DB
	notifier    notifier.Notifier
	authHandler *auth.AuthHandler
}

func NewRegistrationHandler(db *gorm.DB, notifier notifier.Notifier, authHandler *auth.AuthHandler) *RegistrationHandler {
	return &RegistrationHandler{db: db, notifier: notifier, authHandler: authHandler}
}

type RegistrationRequest struct {
	auth.AuthInput
	Body struct {
		ArrivalDate      time.Time `json:"arrival_date" doc:"Date of arrival"`
		DepartureDate    time.Time `json:"departure_date" doc:"Date of departure"`
		FoodRestrictions string    `json:"food_restrictions" doc:"Food restrictions or allergies"`
		ChildrenCount    int       `json:"children_count" doc:"Number of children joining"`
		Cancelled        bool      `json:"cancelled" doc:"Whether the registration is cancelled"`
		Note             string    `json:"note" doc:"Additional notes"`
		Event            string    `json:"event" doc:"Event ID"`
	}
}

type RegistrationResponse struct {
	Body struct {
		Message string `json:"message"`
	}
}

func (h *RegistrationHandler) HandleRegister(ctx context.Context, input *RegistrationRequest) (*RegistrationResponse, error) {
	// Get UserID
	userID, err := h.authHandler.Authorize(input.Cookie)
	if err != nil {
		return nil, err
	}

	// Validate dates
	if input.Body.ArrivalDate.After(input.Body.DepartureDate) {
		return nil, huma.Error400BadRequest("Arrival date cannot be after departure date")
	}

	if userID == 0 {
		return nil, huma.Error401Unauthorized("Unauthorized: Invalid user ID")
	}

	var registration models.Registration
	err = h.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("user_id = ?", userID).FirstOrInit(&registration).Error; err != nil {
			return err
		}
		registration.UserID = userID

		registration.RegistrationFields = models.RegistrationFields{
			ArrivalDate:      input.Body.ArrivalDate,
			DepartureDate:    input.Body.DepartureDate,
			FoodRestrictions: input.Body.FoodRestrictions,
			ChildrenCount:    input.Body.ChildrenCount,
			Cancelled:        input.Body.Cancelled,
			Note:             input.Body.Note,
			Event:            input.Body.Event,
		}

		if err := tx.Save(&registration).Error; err != nil {
			return err
		}

		// Save history snapshot
		history := models.RegistrationHistory{
			RegistrationID:     registration.ID,
			UserID:             registration.UserID,
			RegistrationFields: registration.RegistrationFields,
		}

		if err := tx.Create(&history).Error; err != nil {
			return err
		}

		return nil
	})

	if err != nil {
		return nil, huma.Error500InternalServerError("Failed to process registration: " + err.Error())
	}

	// Fetch user for notification
	var user models.User
	if err := h.db.First(&user, userID).Error; err == nil {
		if h.notifier != nil {
			_ = h.notifier.NotifyRegistration(user, registration)
		}
	}

	res := &RegistrationResponse{}
	res.Body.Message = "Registration processed successfully"
	return res, nil
}

type HistoryRequest struct {
	auth.AuthInput
}

type HistoryResponse struct {
	Body struct {
		History []models.RegistrationHistory `json:"history" doc:"List of registration changes, newest first"`
	}
}

func (h *RegistrationHandler) HandleHistory(ctx context.Context, input *HistoryRequest) (*HistoryResponse, error) {
	// Get UserID
	userID, err := h.authHandler.Authorize(input.Cookie)
	if err != nil {
		return nil, err
	}

	var history []models.RegistrationHistory
	if err := h.db.Where("user_id = ?", userID).Order("created_at DESC").Find(&history).Error; err != nil {
		return nil, huma.Error500InternalServerError("Failed to fetch history: " + err.Error())
	}

	res := &HistoryResponse{}
	res.Body.History = history
	return res, nil
}

type ListRegistrationsRequest struct {
	auth.AuthInput `doc:"Restricted to users with the 'g::t::orgs' role"`
}

type ListRegistrationsResponse struct {
	Body struct {
		Registrations []models.Registration `json:"registrations" doc:"List of all registrations"`
	}
}

func (h *RegistrationHandler) HandleListRegistrations(ctx context.Context, input *ListRegistrationsRequest) (*ListRegistrationsResponse, error) {
	// 1. Authorize
	userID, err := h.authHandler.Authorize(input.Cookie)
	if err != nil {
		return nil, err
	}

	// 2. Get User to get DiscordID
	var user models.User
	if err := h.db.First(&user, userID).Error; err != nil {
		return nil, huma.Error404NotFound("User not found")
	}

	// 3. Check Role
	hasRole, err := h.authHandler.CheckRole(user.DiscordID, "g::t::orgs")
	if err != nil {
		return nil, err
	}
	if !hasRole {
		return nil, huma.Error403Forbidden("Access denied: missing g::t::orgs role")
	}

	// 4. Fetch all registrations
	var registrations []models.Registration
	if err := h.db.Preload("User").Find(&registrations).Error; err != nil {
		return nil, huma.Error500InternalServerError("Failed to fetch registrations: " + err.Error())
	}

	res := &ListRegistrationsResponse{}
	res.Body.Registrations = registrations
	return res, nil
}
