package config

import (
	"os"
	"strings"
)

type Config struct {
	Port            string
	DatabaseURL     string
	ClerkSecretKey  string
	ClerkWebhookSec string
	R2Endpoint      string
	R2AccessKey     string
	R2SecretKey     string
	R2Bucket        string
	R2PublicURL     string
	ClientURL       string
	AllowedOrigins  []string
	PeerJSPort      string
}

func Load() *Config {
	return &Config{
		Port:            getEnv("PORT", "5000"),
		DatabaseURL:     getEnv("DATABASE_URL", "postgres://localhost:5432/chat"),
		ClerkSecretKey:  getEnv("CLERK_SECRET_KEY", ""),
		ClerkWebhookSec: getEnv("CLERK_WEBHOOK_SIGNING_SECRET", ""),
		R2Endpoint:      getEnv("R2_ENDPOINT", ""),
		R2AccessKey:     getEnv("R2_ACCESS_KEY_ID", ""),
		R2SecretKey:     getEnv("R2_SECRET_ACCESS_KEY", ""),
		R2Bucket:        getEnv("R2_BUCKET_NAME", ""),
		R2PublicURL:     getEnv("R2_PUBLIC_URL", ""),
		ClientURL:       getEnv("CLIENT_URL", "http://localhost:5173"),
		AllowedOrigins:  parseOrigins(getEnv("ALLOWED_ORIGINS", "")),
		PeerJSPort:      getEnv("PEERJS_PORT", "5001"),
	}
}

func parseOrigins(s string) []string {
	if s == "" {
		return nil
	}
	var origins []string
	for _, o := range strings.Split(s, ",") {
		o = strings.TrimSpace(o)
		if o != "" {
			origins = append(origins, o)
		}
	}
	return origins
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
