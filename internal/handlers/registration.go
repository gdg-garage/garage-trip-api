package handlers

import (
	"encoding/json"
	"net/http"
	"time"

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
	ArrivalDate      time.Time `json:"arrival_date"`
	DepartureDate    time.Time `json:"departure_date"`
	FoodRestrictions string    `json:"food_restrictions"`
	ChildrenCount    int       `json:"children_count"`
}

func (h *RegistrationHandler) HandleRegister(w http.ResponseWriter, r *http.Request) {
	// Get UserID from context
	userID, ok := r.Context().Value(auth.UserIDKey).(uint)
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	var req RegistrationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Validate dates (basic check)
	if req.ArrivalDate.After(req.DepartureDate) {
		http.Error(w, "Arrival date cannot be after departure date", http.StatusBadRequest)
		return
	}

	registration := models.Registration{
		UserID:           userID,
		ArrivalDate:      req.ArrivalDate,
		DepartureDate:    req.DepartureDate,
		FoodRestrictions: req.FoodRestrictions,
		ChildrenCount:    req.ChildrenCount,
	}

	if err := h.db.Create(&registration).Error; err != nil {
		http.Error(w, "Failed to create registration", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusCreated)
	w.Write([]byte("Registration created successfully"))
}
