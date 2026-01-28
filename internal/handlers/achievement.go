package handlers

import (
	"context"
	"fmt"
	"log"

	"github.com/danielgtaylor/huma/v2"
	"github.com/gdg-garage/garage-trip-api/internal/auth"
	"github.com/gdg-garage/garage-trip-api/internal/models"
	"github.com/gdg-garage/garage-trip-api/internal/notifier"
	"gorm.io/gorm"
)

type AchievementHandler struct {
	db          *gorm.DB
	notifier    notifier.Notifier
	authHandler *auth.AuthHandler
}

func NewAchievementHandler(db *gorm.DB, notifier notifier.Notifier, authHandler *auth.AuthHandler) *AchievementHandler {
	return &AchievementHandler{db: db, notifier: notifier, authHandler: authHandler}
}

type CreateAchievementRequest struct {
	auth.AuthInput
	Body struct {
		Name  string `json:"name" doc:"Name of the achievement" required:"true"`
		Image string `json:"image" doc:"URL to image of the achievement"`
		Code  string `json:"code" doc:"Unique secret code for the achievement" required:"true"`
	}
}

type CreateAchievementResponse struct {
	Body struct {
		ID            uint   `json:"id"`
		DiscordRoleID string `json:"discord_role_id"`
	}
}

func (h *AchievementHandler) HandleCreateAchievement(ctx context.Context, input *CreateAchievementRequest) (*CreateAchievementResponse, error) {
	// 1. Authorize
	userID, err := h.authHandler.Authorize(ctx, input.Cookie)
	if err != nil {
		return nil, err
	}

	// 2. Check Org Role
	var user models.User
	if err := h.db.First(&user, userID).Error; err != nil {
		return nil, huma.Error404NotFound("User not found")
	}

	hasRole, err := h.authHandler.CheckRole(user.DiscordID, "g::t::orgs")
	if err != nil {
		return nil, huma.Error500InternalServerError("Failed to check role: " + err.Error())
	}
	if !hasRole {
		return nil, huma.Error403Forbidden("Access denied: missing g::t::orgs role")
	}

	// 3. Create Discord Role
	roleID, err := h.notifier.CreateRole(input.Body.Name)
	if err != nil {
		return nil, huma.Error500InternalServerError("Failed to create discord role: " + err.Error())
	}

	// 4. Create Achievement
	achievement := models.Achievement{
		Name:          input.Body.Name,
		Image:         input.Body.Image,
		Code:          input.Body.Code,
		DiscordRoleID: roleID,
	}

	if err := h.db.Create(&achievement).Error; err != nil {
		return nil, huma.Error500InternalServerError("Failed to create achievement in DB: " + err.Error())
	}

	res := &CreateAchievementResponse{}
	res.Body.ID = achievement.ID
	res.Body.DiscordRoleID = achievement.DiscordRoleID

	return res, nil
}

type GrantAchievementRequest struct {
	auth.AuthInput
	Body struct {
		Code   string `json:"code" doc:"Unique secret code of the achievement to grant" required:"true"`
		UserID uint   `json:"user_id,omitempty" doc:"Optional user ID to grant to (only for orgs)"`
	}
}

type GrantAchievementResponse struct {
	Body struct {
		Message string `json:"message"`
	}
}

func (h *AchievementHandler) HandleGrantAchievement(ctx context.Context, input *GrantAchievementRequest) (*GrantAchievementResponse, error) {
	// 1. Authorize
	grantorID, err := h.authHandler.Authorize(ctx, input.Cookie)
	if err != nil {
		return nil, err
	}

	var grantor models.User
	if err := h.db.First(&grantor, grantorID).Error; err != nil {
		return nil, huma.Error404NotFound("Grantor not found")
	}

	targetUserID := grantorID
	// Check if granting to another user
	if input.Body.UserID != 0 && input.Body.UserID != grantorID {
		// Must be org
		hasRole, err := h.authHandler.CheckRole(grantor.DiscordID, "g::t::orgs")
		if err != nil {
			return nil, huma.Error500InternalServerError("Failed to check role: " + err.Error())
		}
		if !hasRole {
			return nil, huma.Error403Forbidden("Access denied: only orgs can grant to others")
		}
		targetUserID = input.Body.UserID
	}

	// 2. Find Achievement by secret code
	var achievement models.Achievement
	if err := h.db.Where("code = ?", input.Body.Code).First(&achievement).Error; err != nil {
		return nil, huma.Error404NotFound("Achievement not found or invalid code")
	}

	// 3. Check if already granted
	var existingGrant models.AchievementGrant
	if err := h.db.Where("achievement_id = ? AND user_id = ?", achievement.ID, targetUserID).First(&existingGrant).Error; err == nil {
		return nil, huma.Error409Conflict("Achievement already granted to this user")
	} else if err != gorm.ErrRecordNotFound {
		return nil, huma.Error500InternalServerError("Database error checking grant: " + err.Error())
	}

	// 4. Grant Role on Discord
	var targetUser models.User
	if err := h.db.First(&targetUser, targetUserID).Error; err != nil {
		return nil, huma.Error404NotFound("Target user not found")
	}

	if err := h.notifier.GrantRole(targetUser.DiscordID, achievement.DiscordRoleID); err != nil {
		// Log but don't fail complete flow if discord fails? Or fail?
		// Requirement says "cannot be granted again", implying strong consistency.
		// However, discord might be down or temporary issue.
		// Let's log error and try to proceed but maybe return error?
		// User requirement "grants them the role".
		log.Printf("Failed to grant discord role: %v", err)
		return nil, huma.Error500InternalServerError("Failed to grant discord role: " + err.Error())
	}

	// 5. Create AchievementGrant in DB
	grant := models.AchievementGrant{
		AchievementID: achievement.ID,
		UserID:        targetUserID,
		GrantedByID:   grantorID,
	}

	if err := h.db.Create(&grant).Error; err != nil {
		return nil, huma.Error500InternalServerError("Failed to record grant: " + err.Error())
	}

	// 6. Send Discord Notification
	showGrantor := targetUserID != grantorID
	if err := h.notifier.NotifyAchievement(targetUser, achievement, grantor, showGrantor); err != nil {
		log.Printf("Failed to send notification: %v", err)
		// Don't fail the request here as role and DB are done
	}

	res := &GrantAchievementResponse{}
	res.Body.Message = fmt.Sprintf("Achievement '%s' granted to %s", achievement.Name, targetUser.Username)

	return res, nil
}
