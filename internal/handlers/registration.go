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
		if err := tx.Where("user_id = ? AND event = ?", userID, input.Body.Event).FirstOrInit(&registration).Error; err != nil {
			return err
		}
		registration.UserID = userID
		registration.Event = input.Body.Event

		registration.RegistrationFields = models.RegistrationFields{
			ArrivalDate:      input.Body.ArrivalDate,
			DepartureDate:    input.Body.DepartureDate,
			FoodRestrictions: input.Body.FoodRestrictions,
			ChildrenCount:    input.Body.ChildrenCount,
			Cancelled:        input.Body.Cancelled,
			Note:             input.Body.Note,
		}

		if err := tx.Save(&registration).Error; err != nil {
			return err
		}

		// Save history snapshot
		history := models.RegistrationHistory{
			RegistrationID:     registration.ID,
			UserID:             registration.UserID,
			Event:              registration.Event,
			RegistrationFields: registration.RegistrationFields,
			Model:              gorm.Model{CreatedAt: time.Now()}, // Ensure CreatedAt is set
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
	Diff *bool `query:"diff" default:"true" doc:"If true, returns only changed fields compared to the previous history entry"`
}

type RegistrationFieldsResponse struct {
	ArrivalDate      *time.Time `json:"arrival_date,omitempty"`
	DepartureDate    *time.Time `json:"departure_date,omitempty"`
	FoodRestrictions *string    `json:"food_restrictions,omitempty"`
	ChildrenCount    *int       `json:"children_count,omitempty"`
	Cancelled        *bool      `json:"cancelled,omitempty"`
	Note             *string    `json:"note,omitempty"`
}

type RegistrationHistoryResponseItem struct {
	ID                 uint                       `json:"id"`
	CreatedAt          time.Time                  `json:"created_at"`
	DeletedAt          *time.Time                 `json:"deleted_at,omitempty"`
	RegistrationID     uint                       `json:"registration_id"`
	UserID             uint                       `json:"user_id"`
	Event              string                     `json:"event"`
	RegistrationFields RegistrationFieldsResponse `json:"fields"`
}

type HistoryResponse struct {
	Body struct {
		History []RegistrationHistoryResponseItem `json:"history" doc:"List of registration changes, newest first"`
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

	responseItems := make([]RegistrationHistoryResponseItem, 0, len(history))
	diff := true
	if input.Diff != nil {
		diff = *input.Diff
	}

	for i, item := range history {
		respItem := RegistrationHistoryResponseItem{
			ID:             item.ID,
			CreatedAt:      item.CreatedAt,
			RegistrationID: item.RegistrationID,
			UserID:         item.UserID,
			Event:          item.Event,
		}
		if item.DeletedAt.Valid {
			respItem.DeletedAt = &item.DeletedAt.Time
		}

		if !diff || i == len(history)-1 {
			// Return full object if diff is disabled or it's the oldest item (no previous to compare)
			respItem.RegistrationFields = RegistrationFieldsResponse{
				ArrivalDate:      &item.ArrivalDate,
				DepartureDate:    &item.DepartureDate,
				FoodRestrictions: &item.FoodRestrictions,
				ChildrenCount:    &item.ChildrenCount,
				Cancelled:        &item.Cancelled,
				Note:             &item.Note,
			}
		} else {
			// Compare with previous item (which is next in the list since we ordered DESC)
			prev := history[i+1]
			fields := RegistrationFieldsResponse{}

			if !item.ArrivalDate.Equal(prev.ArrivalDate) {
				fields.ArrivalDate = &item.ArrivalDate
			}
			if !item.DepartureDate.Equal(prev.DepartureDate) {
				fields.DepartureDate = &item.DepartureDate
			}
			if item.FoodRestrictions != prev.FoodRestrictions {
				fields.FoodRestrictions = &item.FoodRestrictions
			}
			if item.ChildrenCount != prev.ChildrenCount {
				fields.ChildrenCount = &item.ChildrenCount
			}
			if item.Cancelled != prev.Cancelled {
				fields.Cancelled = &item.Cancelled
			}
			if item.Note != prev.Note {
				fields.Note = &item.Note
			}
			respItem.RegistrationFields = fields
		}
		responseItems = append(responseItems, respItem)
	}

	res := &HistoryResponse{}
	res.Body.History = responseItems
	return res, nil
}

type ListRegistrationsRequest struct {
	auth.AuthInput `doc:"Restricted to users with the 'g::t::orgs' role"`
	Event          string `query:"event" doc:"Optional event ID to filter by"`
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
	query := h.db.Preload("User")

	if input.Event != "" {
		query = query.Where("event = ?", input.Event)
	}

	if err := query.Find(&registrations).Error; err != nil {
		return nil, huma.Error500InternalServerError("Failed to fetch registrations: " + err.Error())
	}

	res := &ListRegistrationsResponse{}
	res.Body.Registrations = registrations
	return res, nil
}
