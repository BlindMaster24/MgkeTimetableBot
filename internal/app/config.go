package app

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"

	"github.com/blindmaster24/MgkeTimetableBot/internal/config"
	telegrambot "github.com/blindmaster24/MgkeTimetableBot/internal/telegram"
)

func ResolveConfigPath(flagValue string) string {
	if flagValue != "" {
		return flagValue
	}
	if env := os.Getenv("CONFIG_PATH"); env != "" {
		return env
	}
	return "configs/config.yaml"
}

func loadConfig(path string) (*config.Config, error) {
	if _, err := os.Stat(path); err != nil {
		return nil, fmt.Errorf("config file not found: %s", path)
	}

	cfg, err := config.LoadWithEnv(path, nil)
	if err != nil {
		return nil, fmt.Errorf("load config %s: %v", path, err)
	}

	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("invalid config %s: %v", path, err)
	}

	if cfg.Telegram.Webhook.Enabled {
		if err := telegrambot.ValidateWebhook(cfg); err != nil {
			return nil, fmt.Errorf("invalid config %s: %v", path, err)
		}
	}

	return cfg, nil
}

func configStamp(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])[:8]
}
