package handlers

import (
	"context"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/gdg-garage/garage-trip-api/internal/auth"
	"github.com/gdg-garage/garage-trip-api/internal/config"
	"github.com/gdg-garage/garage-trip-api/internal/models"
	"gorm.io/gorm"
)

type EventHandler struct {
	db          *gorm.DB
	authHandler *auth.AuthHandler
	cfg         *config.Config
}

func NewEventHandler(db *gorm.DB, authHandler *auth.AuthHandler, cfg *config.Config) *EventHandler {
	return &EventHandler{db: db, authHandler: authHandler, cfg: cfg}
}

func (h *EventHandler) checkOrg(ctx context.Context, cookie string) error {
	userID, err := h.authHandler.Authorize(ctx, cookie)
	if err != nil {
		return err
	}

	var user models.User
	if err := h.db.First(&user, userID).Error; err != nil {
		return huma.Error404NotFound("User not found")
	}

	hasRole, err := h.authHandler.CheckRole(user.DiscordID, h.cfg.OrgRole)
	if err != nil {
		return err
	}
	if !hasRole {
		return huma.Error403Forbidden("Access denied: missing " + h.cfg.OrgRole + " role")
	}

	return nil
}

type EventItem struct {
	ID          uint       `json:"id"`
	Code        string     `json:"code"`
	Name        string     `json:"name"`
	StartDate   *time.Time `json:"start_date,omitempty"`
	EndDate     *time.Time `json:"end_date,omitempty"`
	Location    string     `json:"location,omitempty"`
	Description string     `json:"description,omitempty"`
	Enabled     bool       `json:"enabled"`
	Status      string     `json:"status"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

func toEventItem(e models.Event) EventItem {
	status := e.Status
	if status == "active" || status == "" {
		if e.EndDate != nil && e.EndDate.Before(time.Now()) {
			status = "past"
		} else {
			status = "future"
		}
	}
	return EventItem{
		ID:          e.ID,
		Code:        e.Code,
		Name:        e.Name,
		StartDate:   e.StartDate,
		EndDate:     e.EndDate,
		Location:    e.Location,
		Description: e.Description,
		Enabled:     e.Enabled,
		Status:      status,
		CreatedAt:   e.CreatedAt,
		UpdatedAt:   e.UpdatedAt,
	}
}

type ListEventsRequest struct {
	auth.AuthInput
	All bool `query:"all" doc:"Return all events including locked ones (org only)"`
}

type ListEventsResponse struct {
	Body struct {
		Events []EventItem `json:"events" doc:"List of events"`
	}
}

func isPastEvent(e models.Event) bool {
	if e.Status == "past" {
		return true
	}
	if e.EndDate != nil && e.EndDate.Before(time.Now()) {
		return true
	}
	return false
}

func (h *EventHandler) HandleList(ctx context.Context, input *ListEventsRequest) (*ListEventsResponse, error) {
	var events []models.Event
	if err := h.db.Order("start_date DESC, id DESC").Find(&events).Error; err != nil {
		return nil, huma.Error500InternalServerError("Failed to fetch events: " + err.Error())
	}

	if input != nil && input.All {
		if err := h.checkOrg(ctx, input.Cookie); err != nil {
			return nil, err
		}
		items := make([]EventItem, len(events))
		for i, e := range events {
			items[i] = toEventItem(e)
		}
		res := &ListEventsResponse{}
		res.Body.Events = items
		return res, nil
	}

	// For regular users / guests:
	// If authenticated, find user's registered events
	userRegistered := make(map[string]bool)
	if input != nil && input.Cookie != "" {
		if userID, err := h.authHandler.Authorize(ctx, input.Cookie); err == nil {
			var regs []models.Registration
			if err := h.db.Where("user_id = ?", userID).Find(&regs).Error; err == nil {
				for _, r := range regs {
					userRegistered[r.Event] = true
				}
			}
		}
	}

	items := make([]EventItem, 0, len(events))
	for _, e := range events {
		if e.Enabled || (isPastEvent(e) && userRegistered[e.Code]) {
			items = append(items, toEventItem(e))
		}
	}

	res := &ListEventsResponse{}
	res.Body.Events = items
	return res, nil
}

type CreateEventRequest struct {
	auth.AuthInput `doc:"Restricted to org users"`
	Body           struct {
		Code        string     `json:"code" doc:"Unique event code (e.g. g::t::8.0.0)"`
		Name        string     `json:"name" doc:"Event name"`
		StartDate   *time.Time `json:"start_date,omitempty" doc:"Event start date"`
		EndDate     *time.Time `json:"end_date,omitempty" doc:"Event end date"`
		Location    string     `json:"location,omitempty" doc:"Event location"`
		Description string     `json:"description,omitempty" doc:"Event description"`
		Enabled     bool       `json:"enabled" doc:"Whether event registration is open"`
		Status      string     `json:"status,omitempty" doc:"Event status: future or past"`
	}
}

type EventResponse struct {
	Body EventItem
}

func (h *EventHandler) HandleCreate(ctx context.Context, input *CreateEventRequest) (*EventResponse, error) {
	if err := h.checkOrg(ctx, input.Cookie); err != nil {
		return nil, err
	}

	if input.Body.Code == "" {
		return nil, huma.Error400BadRequest("Event code is required")
	}
	if input.Body.Name == "" {
		return nil, huma.Error400BadRequest("Event name is required")
	}

	var existing models.Event
	if err := h.db.Where("code = ?", input.Body.Code).First(&existing).Error; err == nil {
		return nil, huma.Error400BadRequest("Event with code " + input.Body.Code + " already exists")
	}

	status := input.Body.Status
	if status == "" {
		status = "future"
	}

	event := models.Event{
		Code:        input.Body.Code,
		Name:        input.Body.Name,
		StartDate:   input.Body.StartDate,
		EndDate:     input.Body.EndDate,
		Location:    input.Body.Location,
		Description: input.Body.Description,
		Enabled:     input.Body.Enabled,
		Status:      status,
	}

	if err := h.db.Create(&event).Error; err != nil {
		return nil, huma.Error500InternalServerError("Failed to create event: " + err.Error())
	}

	return &EventResponse{Body: toEventItem(event)}, nil
}

type UpdateEventRequest struct {
	auth.AuthInput `doc:"Restricted to org users"`
	ID             uint `path:"id" doc:"Event ID"`
	Body           struct {
		Code        *string    `json:"code,omitempty" doc:"Event code"`
		Name        *string    `json:"name,omitempty" doc:"Event name"`
		StartDate   *time.Time `json:"start_date,omitempty" doc:"Event start date"`
		EndDate     *time.Time `json:"end_date,omitempty" doc:"Event end date"`
		Location    *string    `json:"location,omitempty" doc:"Event location"`
		Description *string    `json:"description,omitempty" doc:"Event description"`
		Enabled     *bool      `json:"enabled,omitempty" doc:"Whether event registration is open"`
		Status      *string    `json:"status,omitempty" doc:"Event status: future or past"`
	}
}

func (h *EventHandler) HandleUpdate(ctx context.Context, input *UpdateEventRequest) (*EventResponse, error) {
	if err := h.checkOrg(ctx, input.Cookie); err != nil {
		return nil, err
	}

	var event models.Event
	if err := h.db.First(&event, input.ID).Error; err != nil {
		return nil, huma.Error404NotFound("Event not found")
	}

	if input.Body.Code != nil && *input.Body.Code != "" {
		// If changing code, verify uniqueness
		if *input.Body.Code != event.Code {
			var check models.Event
			if err := h.db.Where("code = ? AND id != ?", *input.Body.Code, event.ID).First(&check).Error; err == nil {
				return nil, huma.Error400BadRequest("Event code " + *input.Body.Code + " is already taken")
			}
		}
		event.Code = *input.Body.Code
	}
	if input.Body.Name != nil && *input.Body.Name != "" {
		event.Name = *input.Body.Name
	}
	if input.Body.StartDate != nil {
		event.StartDate = input.Body.StartDate
	}
	if input.Body.EndDate != nil {
		event.EndDate = input.Body.EndDate
	}
	if input.Body.Location != nil {
		event.Location = *input.Body.Location
	}
	if input.Body.Description != nil {
		event.Description = *input.Body.Description
	}
	if input.Body.Enabled != nil {
		event.Enabled = *input.Body.Enabled
	}
	if input.Body.Status != nil && *input.Body.Status != "" {
		event.Status = *input.Body.Status
	}

	if err := h.db.Save(&event).Error; err != nil {
		return nil, huma.Error500InternalServerError("Failed to update event: " + err.Error())
	}

	return &EventResponse{Body: toEventItem(event)}, nil
}

type ToggleEventRequest struct {
	auth.AuthInput `doc:"Restricted to org users"`
	ID             uint `path:"id" doc:"Event ID"`
}

func (h *EventHandler) HandleToggle(ctx context.Context, input *ToggleEventRequest) (*EventResponse, error) {
	if err := h.checkOrg(ctx, input.Cookie); err != nil {
		return nil, err
	}

	var event models.Event
	if err := h.db.First(&event, input.ID).Error; err != nil {
		return nil, huma.Error404NotFound("Event not found")
	}

	event.Enabled = !event.Enabled

	if err := h.db.Save(&event).Error; err != nil {
		return nil, huma.Error500InternalServerError("Failed to toggle event: " + err.Error())
	}

	return &EventResponse{Body: toEventItem(event)}, nil
}
