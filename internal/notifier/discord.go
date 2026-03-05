package notifier

import (
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/bwmarrin/discordgo"
	"github.com/gdg-garage/garage-trip-api/internal/models"
)

type Notifier interface {

	// CreateRole Create a new role in guild
	CreateRole(name string) (string, error)
	// GrantRole Grant a role to a user
	GrantRole(userID string, roleID string) error
	// NotifyAchievement Send a message about the achievement
	NotifyAchievement(user models.User, achievement models.Achievement, grantor models.User, showGrantor bool) error
	// NotifyRegistration Notify about registration changes
	NotifyRegistration(user models.User, registration models.Registration) error
	// HasRole Check if a user has a role
	HasRole(userID string, roleID string) (bool, error)
}

type DiscordNotifier struct {
	session                *discordgo.Session
	achievementsChannelID  string
	registrationsChannelID string
	guildID                string
	achievementPrefix      string
}

func NewDiscordNotifier(session *discordgo.Session, achievementsChannelID string, registrationsChannelID string, guildID string, achievementPrefix string) *DiscordNotifier {
	return &DiscordNotifier{
		session:                session,
		achievementsChannelID:  achievementsChannelID,
		registrationsChannelID: registrationsChannelID,
		guildID:                guildID,
		achievementPrefix:      achievementPrefix,
	}
}

func (n *DiscordNotifier) CreateRole(name string) (string, error) {
	if n.session == nil || n.guildID == "" {
		return "", fmt.Errorf("discord session is nil or guildID is empty")
	}

	role, err := n.session.GuildRoleCreate(n.guildID, &discordgo.RoleParams{
		Name: n.achievementPrefix + name,
	})
	if err != nil {
		return "", fmt.Errorf("failed to create role: %w", err)
	}
	return role.ID, nil
}

func (n *DiscordNotifier) GrantRole(userID string, roleID string) error {
	if n.session == nil || n.guildID == "" {
		return fmt.Errorf("discord session is nil or guildID is empty")
	}

	err := n.session.GuildMemberRoleAdd(n.guildID, userID, roleID)
	if err != nil {
		return fmt.Errorf("failed to grant role: %w", err)
	}
	return nil
}

func (n *DiscordNotifier) HasRole(userID string, roleID string) (bool, error) {
	if n.session == nil || n.guildID == "" {
		return false, fmt.Errorf("discord session is nil or guildID is empty")
	}

	member, err := n.session.GuildMember(n.guildID, userID)
	if err != nil {
		return false, fmt.Errorf("failed to get guild member: %w", err)
	}

	for _, id := range member.Roles {
		if id == roleID {
			return true, nil
		}
	}

	return false, nil
}

func (n *DiscordNotifier) NotifyAchievement(user models.User, achievement models.Achievement, grantor models.User, showGrantor bool) error {
	if n.session == nil || n.achievementsChannelID == "" {
		return fmt.Errorf("discord session is nil or achievements channel ID is empty")
	}

	embed := &discordgo.MessageEmbed{
		Title:       "Achievement Unlocked! 🏆",
		Description: fmt.Sprintf("<@%s> has unlocked the **%s** achievement!", user.DiscordID, achievement.Name),
		Color:       0xFFD700, // Gold color
	}

	if showGrantor {
		embed.Fields = []*discordgo.MessageEmbedField{
			{
				Name:   "Granted By",
				Value:  grantor.Username,
				Inline: true,
			},
		}
	}

	var files []*discordgo.File
	if achievement.Image != "" {
		// Check if it's a URL or a local file
		if len(achievement.Image) > 4 && achievement.Image[:4] == "http" {
			embed.Thumbnail = &discordgo.MessageEmbedThumbnail{
				URL: achievement.Image,
			}
		} else {
			// Local file
			f, err := os.Open(achievement.Image)
			if err != nil {
				log.Printf("Failed to open achievement image: %v", err)
			} else {
				defer f.Close()
				filename := filepath.Base(achievement.Image)
				embed.Thumbnail = &discordgo.MessageEmbedThumbnail{
					URL: "attachment://" + filename,
				}
				files = append(files, &discordgo.File{
					Name:   filename,
					Reader: f,
				})
			}
		}
	}

	var err error
	if len(files) > 0 {
		_, err = n.session.ChannelMessageSendComplex(n.achievementsChannelID, &discordgo.MessageSend{
			Embeds: []*discordgo.MessageEmbed{embed},
			Files:  files,
		})
	} else {
		_, err = n.session.ChannelMessageSendEmbed(n.achievementsChannelID, embed)
	}

	if err != nil {
		log.Printf("Failed to send discord message: %v", err)
		return err
	}

	return nil
}

func (n *DiscordNotifier) NotifyRegistration(user models.User, registration models.Registration) error {
	if n.session == nil {
		return fmt.Errorf("discord session is nil")
	}
	if n.registrationsChannelID == "" {
		return fmt.Errorf("discord registrations channel ID is empty")
	}

	// Role Management
	if n.guildID != "" {
		roleName := registration.Event
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

	message := fmt.Sprintf("🎉 **Registration Update: %s**\n**User:** %s (<@%s>)\n**Status:** %s\n**Dates:** %s - %s\n**Children:** %d\n**Food Restrictions:** %s%s",
		registration.Event,
		user.Username,
		user.DiscordID,
		status,
		registration.ArrivalDate.Format("2006-01-02"),
		registration.DepartureDate.Format("2006-01-02"),
		registration.ChildrenCount,
		registration.FoodRestrictions,
		noteStr,
	)

	_, err := n.session.ChannelMessageSend(n.registrationsChannelID, message)
	if err != nil {
		log.Printf("Failed to send discord message: %v", err)
		return err
	}

	return nil
}
