package notifier

import (
	"fmt"
	"log"

	"github.com/bwmarrin/discordgo"
	"github.com/gdg-garage/garage-trip-api/internal/models"
)

type Notifier interface {
	NotifyRegistration(user models.User, registration models.Registration) error
}

type DiscordNotifier struct {
	session   *discordgo.Session
	channelID string
	guildID   string
}

func NewDiscordNotifier(session *discordgo.Session, channelID string, guildID string) *DiscordNotifier {
	return &DiscordNotifier{
		session:   session,
		channelID: channelID,
		guildID:   guildID,
	}
}

func (n *DiscordNotifier) NotifyRegistration(user models.User, registration models.Registration) error {
	if n.session == nil {
		return fmt.Errorf("discord session is nil")
	}
	if n.channelID == "" {
		return fmt.Errorf("discord channel ID is empty")
	}

	// Role Management
	if n.guildID != "" {
		roleName := "g::t::7.0.0"
		roles, err := n.session.GuildRoles(n.guildID)
		if err != nil {
			log.Printf("Failed to fetch guild roles: %v", err)
		} else {
			var roleID string
			for _, r := range roles {
				if r.Name == roleName {
					roleID = r.ID
					break
				}
			}

			if roleID != "" {
				if registration.Cancelled {
					err = n.session.GuildMemberRoleRemove(n.guildID, user.DiscordID, roleID)
					if err != nil {
						log.Printf("Failed to remove role: %v", err)
					}
				} else {
					err = n.session.GuildMemberRoleAdd(n.guildID, user.DiscordID, roleID)
					if err != nil {
						log.Printf("Failed to add role: %v", err)
					}
				}
			} else {
				log.Printf("Role %s not found in guild %s", roleName, n.guildID)
			}
		}
	}

	status := "registered/updated registration"
	if registration.Cancelled {
		status = "cancelled registration 😢 👎"
	}

	noteStr := ""
	if registration.Note != "" {
		noteStr = fmt.Sprintf("\n**Note:** %s", registration.Note)
	}

	message := fmt.Sprintf("🎉 **Registration Update**\n**User:** %s (<@%s>)\n**Status:** %s\n**Dates:** %s - %s\n**Children:** %d\n**Food Restrictions:** %s%s",
		user.Username,
		user.DiscordID,
		status,
		registration.ArrivalDate.Format("2006-01-02"),
		registration.DepartureDate.Format("2006-01-02"),
		registration.ChildrenCount,
		registration.FoodRestrictions,
		noteStr,
	)

	_, err := n.session.ChannelMessageSend(n.channelID, message)
	if err != nil {
		log.Printf("Failed to send discord message: %v", err)
		return err
	}

	return nil
}
