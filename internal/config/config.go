package config

import (
	"os"
	"strings"
)

type Config struct {
	APIKey            string
	GRPCAddr          string
	WebhookURL        string
	TuyaAccessID      string
	TuyaAccessKey     string
	EventMatchDevices []string
	EventMatchDPs     []string
}

func FromEnv() Config {
	return Config{
		APIKey:            strings.TrimSpace(os.Getenv("API_KEY")),
		GRPCAddr:          envOr("GRPC_ADDR", ":50051"),
		WebhookURL:        strings.TrimSpace(os.Getenv("WEBHOOK_URL")),
		TuyaAccessID:      strings.TrimSpace(os.Getenv("TUYA_ACCESSID")),
		TuyaAccessKey:     strings.TrimSpace(os.Getenv("TUYA_ACCESSKEY")),
		EventMatchDevices: splitCSV(os.Getenv("EVENT_MATCH_DEVICE_IDS")),
		EventMatchDPs:     splitCSV(envOr("EVENT_MATCH_DP_CODES", "unlock_temporary,unlock_password")),
	}
}

func envOr(key, fallback string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return fallback
}

func splitCSV(s string) []string {
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

func (c Config) MatchesDevice(deviceID string) bool {
	if len(c.EventMatchDevices) == 0 {
		return true
	}
	for _, id := range c.EventMatchDevices {
		if id == deviceID {
			return true
		}
	}
	return false
}

func (c Config) MatchesDP(code string) bool {
	if len(c.EventMatchDPs) == 0 {
		return false
	}
	for _, dp := range c.EventMatchDPs {
		if dp == code {
			return true
		}
	}
	return false
}
