package main

import (
	"testing"

	"github.com/disgoorg/snowflake/v2"
)

func TestParseSnowflakes(t *testing.T) {
	ids, err := parseSnowflakes(" 123, 456 ,789")
	if err != nil {
		t.Fatalf("parseSnowflakes returned error: %v", err)
	}

	if len(ids) != 3 {
		t.Fatalf("expected 3 ids, got %d", len(ids))
	}

	if ids[0] != snowflake.ID(123) || ids[1] != snowflake.ID(456) || ids[2] != snowflake.ID(789) {
		t.Fatalf("unexpected ids: %v", ids)
	}

	if _, err := parseSnowflakes("abc"); err == nil {
		t.Fatal("expected error for invalid snowflake")
	}
}

func TestApplyEnvOverridesConfig(t *testing.T) {
	t.Setenv("TOKEN", "env-token")
	t.Setenv("SPOTIFY_CLIENT_ID", "env-id")
	t.Setenv("WHITELIST", "true")
	t.Setenv("OWNERS", "111,222")
	t.Setenv("LOG_LEVEL", "debug")

	cfg := Config{
		Token:    "yaml-token",
		ClientID: "yaml-id",
		LogLevel: "informational",
		Owner:    []snowflake.ID{1},
	}

	if err := applyEnv(&cfg); err != nil {
		t.Fatalf("applyEnv returned error: %v", err)
	}

	if cfg.Token != "env-token" {
		t.Errorf("expected env token, got %q", cfg.Token)
	}
	if cfg.ClientID != "env-id" {
		t.Errorf("expected env client id, got %q", cfg.ClientID)
	}
	if !cfg.WhiteList {
		t.Error("expected whitelist to be true")
	}
	if cfg.LogLevel != "debug" {
		t.Errorf("expected debug log level, got %q", cfg.LogLevel)
	}
	if len(cfg.Owner) != 2 || cfg.Owner[0] != snowflake.ID(111) || cfg.Owner[1] != snowflake.ID(222) {
		t.Errorf("unexpected owners: %v", cfg.Owner)
	}
}

func TestApplyEnvIgnoresEmpty(t *testing.T) {
	t.Setenv("TOKEN", "")
	t.Setenv("ORIGIN", "")

	cfg := Config{Token: "yaml-token"}

	if err := applyEnv(&cfg); err != nil {
		t.Fatalf("applyEnv returned error: %v", err)
	}

	if cfg.Token != "yaml-token" {
		t.Errorf("expected yaml token to remain, got %q", cfg.Token)
	}
}

func TestApplyEnvAPITokens(t *testing.T) {
	t.Setenv("API_TOKENS", `[{"userid":"123","token":"correct-horse-battery-staple","textchannelid":"456","guildid":"789"}]`)

	cfg := Config{}

	if err := applyEnv(&cfg); err != nil {
		t.Fatalf("applyEnv returned error: %v", err)
	}

	if len(cfg.ApiTokens) != 1 {
		t.Fatalf("expected 1 api token, got %d", len(cfg.ApiTokens))
	}

	tok := cfg.ApiTokens[0]
	if tok.UserID != snowflake.ID(123) || tok.Token != "correct-horse-battery-staple" ||
		tok.TextChannel != snowflake.ID(456) || tok.Guild != snowflake.ID(789) {
		t.Errorf("unexpected api token: %+v", tok)
	}
}

func TestApplyEnvInvalidAPITokens(t *testing.T) {
	t.Setenv("API_TOKENS", "not-json")

	cfg := Config{}

	if err := applyEnv(&cfg); err == nil {
		t.Fatal("expected error for invalid API_TOKENS")
	}
}
