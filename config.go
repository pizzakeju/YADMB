package main

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/disgoorg/snowflake/v2"
	"github.com/goccy/go-json"
	"github.com/joho/godotenv"
	"github.com/kkyr/fig"
)

// loadConfig loads the bot configuration from config.yml, if present, and
// then overrides it with environment variables. In container platforms like
// Dokploy only the environment is usually available.
func loadConfig() (Config, error) {
	var cfg Config

	if configFileExists() {
		if err := fig.Load(&cfg, fig.File("config.yml"), fig.Dirs(".", "./data")); err != nil {
			return cfg, fmt.Errorf("error loading config.yml: %w", err)
		}
	}

	if err := applyEnv(&cfg); err != nil {
		return cfg, err
	}

	return cfg, nil
}

func configFileExists() bool {
	for _, dir := range []string{".", "./data"} {
		if _, err := os.Stat(dir + "/config.yml"); err == nil {
			return true
		}
	}

	return false
}

// applyEnv overrides cfg with values from environment variables. A .env file
// next to the binary (or in ./data) is loaded first, if present. Empty
// variables are ignored, so config.yml values stay in effect.
func applyEnv(cfg *Config) error {
	_ = godotenv.Load("./.env", "./data/.env")

	set := func(key string, dst *string) {
		if v := os.Getenv(key); v != "" {
			*dst = v
		}
	}

	set("TOKEN", &cfg.Token)
	set("SPOTIFY_CLIENT_ID", &cfg.ClientID)
	set("SPOTIFY_CLIENT_SECRET", &cfg.ClientSecret)
	set("DATABASE_DSN", &cfg.DSN)
	set("DATABASE_DRIVER", &cfg.Driver)
	set("LOG_LEVEL", &cfg.LogLevel)
	set("YOUTUBE_API_KEY", &cfg.YouTubeAPI)
	set("HTTP_ADDRESS", &cfg.Address)
	set("ORIGIN", &cfg.Origin)

	if v := os.Getenv("OWNERS"); v != "" {
		ids, err := parseSnowflakes(v)
		if err != nil {
			return fmt.Errorf("invalid OWNERS: %w", err)
		}
		cfg.Owner = ids
	}

	if v := os.Getenv("GUILD_LIST"); v != "" {
		ids, err := parseSnowflakes(v)
		if err != nil {
			return fmt.Errorf("invalid GUILD_LIST: %w", err)
		}
		cfg.GuildList = ids
	}

	if v := os.Getenv("WHITELIST"); v != "" {
		b, err := strconv.ParseBool(v)
		if err != nil {
			return fmt.Errorf("invalid WHITELIST: %w", err)
		}
		cfg.WhiteList = b
	}

	if v := os.Getenv("API_TOKENS"); v != "" {
		var tokens []apiToken
		if err := json.Unmarshal([]byte(v), &tokens); err != nil {
			return fmt.Errorf("invalid API_TOKENS: %w", err)
		}
		cfg.ApiTokens = tokens
	}

	return nil
}

func parseSnowflakes(s string) ([]snowflake.ID, error) {
	parts := strings.Split(s, ",")
	ids := make([]snowflake.ID, 0, len(parts))

	for _, p := range parts {
		id, err := snowflake.Parse(strings.TrimSpace(p))
		if err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}

	return ids, nil
}
