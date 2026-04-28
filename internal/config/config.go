package config

import (
	"fmt"
	"os"

	"github.com/joho/godotenv"
)

type Config struct {
	Port       string
	BaseURL    string
	DBURL      string
	JWTSecret  []byte
	AdminToken string
}

func Load() (*Config, error) {
	_ = godotenv.Load()

	port := getenv("PORT", "8084")
	baseURL := os.Getenv("BASE_URL")
	if baseURL == "" {
		return nil, fmt.Errorf("BASE_URL is required")
	}
	dbURL := os.Getenv("DB_URL")
	if dbURL == "" {
		return nil, fmt.Errorf("DB_URL is required")
	}
	jwtSecret := os.Getenv("JWT_SECRET")
	if jwtSecret == "" {
		return nil, fmt.Errorf("JWT_SECRET is required")
	}
	adminToken := os.Getenv("ADMIN_TOKEN")
	if adminToken == "" {
		return nil, fmt.Errorf("ADMIN_TOKEN is required")
	}

	return &Config{
		Port:       port,
		BaseURL:    baseURL,
		DBURL:      dbURL,
		JWTSecret:  []byte(jwtSecret),
		AdminToken: adminToken,
	}, nil
}

func getenv(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}
