package config

import (
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/joho/godotenv"
)

type Config struct {
	Port           string
	BaseURL        string
	DBURL          string
	JWTSecret      []byte
	AdminToken     string
	GoMetaBaseURL  string
	GoMetaTimeout  time.Duration
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

	gometaBaseURL := getenv("GOMETA_BASE_URL", "https://apis.gometa.org")
	gometaTimeoutSec, _ := strconv.Atoi(getenv("GOMETA_TIMEOUT_SECONDS", "10"))
	if gometaTimeoutSec <= 0 {
		gometaTimeoutSec = 10
	}

	return &Config{
		Port:          port,
		BaseURL:       baseURL,
		DBURL:         dbURL,
		JWTSecret:     []byte(jwtSecret),
		AdminToken:    adminToken,
		GoMetaBaseURL: gometaBaseURL,
		GoMetaTimeout: time.Duration(gometaTimeoutSec) * time.Second,
	}, nil
}

func getenv(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}
