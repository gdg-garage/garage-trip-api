package notifier

import (
	"fmt"
	"log"

	"github.com/bwmarrin/discordgo"
	"github.com/gdg-garage/garage-trip-api/internal/config"
	"github.com/gdg-garage/garage-trip-api/internal/models"
)

type Notifier interface {
	NotifyRegistration(user models.User, registration models.Registration) error
}

type DiscordNotifier struct {
	session   *discordgo.Session
	channelID string
}

func NewDiscordNotifier(cfg *config.Config) (*DiscordNotifier, error) {
	if cfg.DiscordBotToken == "" || cfg.DiscordNotificationsChannelID == "" {
		return nil, fmt.Errorf("discord bot token or channel ID not configured")
	}

	session, err := discordgo.New("Bot " + cfg.DiscordBotToken)
	if err != nil {
		return nil, fmt.Errorf("failed to create discord session: %w", err)
	}

	return &DiscordNotifier{
		session:   session,
		channelID: cfg.DiscordNotificationsChannelID,
	}, nil
}

func (n *DiscordNotifier) NotifyRegistration(user models.User, registration models.Registration) error {
	status := "registered/updated registration"
	if registration.Cancelled {
		status = "cancelled registration"
	}

	message := fmt.Sprintf("🔔 **Registration Update**\n**User:** %s (<@%s>)\n**Status:** %s\n**Dates:** %s - %s\n**Children:** %d\n**Food Restrictions:** %s",
		user.Username,
		user.DiscordID,
		status,
		registration.ArrivalDate.Format("2006-01-02"),
		registration.DepartureDate.Format("2006-01-02"),
		registration.ChildrenCount,
		registration.FoodRestrictions,
	)

	_, err := n.session.ChannelMessageSend(n.channelID, message)
	if err != nil {
		log.Printf("Failed to send discord message: %v", err)
		return err
	}

	return nil
}
