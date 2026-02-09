package config

import (
	"log"

	"github.com/spf13/viper"
)

type Config struct {
	Port                          string   `mapstructure:"PORT"`
	DatabasePath                  string   `mapstructure:"DATABASE_PATH"`
	DiscordClientID               string   `mapstructure:"DISCORD_CLIENT_ID"`
	DiscordClientSecret           string   `mapstructure:"DISCORD_CLIENT_SECRET"`
	DiscordRedirectURL            string   `mapstructure:"DISCORD_REDIRECT_URL"`
	DiscordGuildID                string   `mapstructure:"DISCORD_GUILD_ID"`
	DiscordBotToken               string   `mapstructure:"DISCORD_BOT_TOKEN"`
	DiscordNotificationsChannelID string   `mapstructure:"DISCORD_NOTIFICATIONS_CHANNEL_ID"`
	JWTSecret                     string   `mapstructure:"JWT_SECRET"`
	FrontendURL                   string   `mapstructure:"FRONTEND_URL"`
	AchievementPrefix             string   `mapstructure:"ACHIEVEMENT_PREFIX"`
	EnableCORS                    bool     `mapstructure:"ENABLE_CORS"`
	EnabledEvents                 []string `mapstructure:"ENABLED_EVENTS"`
}

func LoadConfig() *Config {
	viper.SetDefault("PORT", "8080")
	viper.SetDefault("DATABASE_PATH", "garage.db")
	viper.SetDefault("DISCORD_REDIRECT_URL", "http://127.0.0.1:8080/auth/discord/callback")
	viper.SetDefault("DISCORD_GUILD_ID", "750810991897608293")
	viper.SetDefault("FRONTEND_URL", "http://127.0.0.1:4000/register")
	viper.SetDefault("ACHIEVEMENT_PREFIX", "achievement::")
	viper.SetDefault("ENABLED_EVENTS", []string{"g::t::7.0.0"})

	viper.BindEnv("DISCORD_CLIENT_ID")
	viper.BindEnv("DISCORD_CLIENT_SECRET")
	viper.BindEnv("DISCORD_GUILD_ID")
	viper.BindEnv("DISCORD_BOT_TOKEN")
	viper.BindEnv("DISCORD_NOTIFICATIONS_CHANNEL_ID")
	viper.BindEnv("JWT_SECRET")
	viper.BindEnv("FRONTEND_URL")
	viper.BindEnv("ACHIEVEMENT_PREFIX")
	viper.BindEnv("ENABLE_CORS")
	viper.BindEnv("ENABLED_EVENTS")

	viper.AutomaticEnv()

	var config Config
	if err := viper.Unmarshal(&config); err != nil {
		log.Fatalf("Unable to decode into struct, %v", err)
	}

	return &config
}
