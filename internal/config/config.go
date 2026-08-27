package config

import (
	"log"
	"os"
	"strconv"

	"github.com/joho/godotenv"
)

type Config struct {
	Port              string
	DatabaseURL       string
	RedisURL          string
	CaddyAdminURL     string
	BaseDomain        string
	MasterKey         string
	JWTSecret         string
	JWTExpiresIn      int64
	DockerNetwork     string
	BuildStorageDir   string
	Environment       string
	FrontendURL       string
}

func Load() *Config {
	// Attempt to load .env from current dir or parent dir
	_ = godotenv.Load()
	_ = godotenv.Load("../.env")

	port := getEnv("PORT", "8080")
	dbURL := getEnv("DATABASE_URL", "postgres://shipr:shipr_secret_password@localhost:5432/shipr?sslmode=disable")
	redisURL := getEnv("REDIS_URL", "redis://localhost:6379")
	caddyAdminURL := getEnv("CADDY_ADMIN_URL", "http://localhost:2019")
	baseDomain := getEnv("BASE_DOMAIN", "shipr.local")
	masterKey := getEnv("CLOUDBOX_MASTER_KEY", "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef")
	jwtSecret := getEnv("JWT_SECRET", "super_secret_jwt_access_key_change_in_production_32bytes")
	dockerNetwork := getEnv("DOCKER_NETWORK", "shipr-network")
	buildStorageDir := getEnv("BUILD_STORAGE_DIR", "./tmp/shipr-builds")
	environment := getEnv("ENVIRONMENT", "development")
	frontendURL := getEnv("FRONTEND_URL", "http://localhost:3000")

	jwtExpiresInStr := getEnv("JWT_EXPIRES_IN", "86400")
	jwtExpiresIn, err := strconv.ParseInt(jwtExpiresInStr, 10, 64)
	if err != nil {
		jwtExpiresIn = 86400
	}

	if len(masterKey) != 64 {
		log.Printf("[CONFIG WARNING] CLOUDBOX_MASTER_KEY should ideally be 64 hex characters (32 bytes). Using key provided.")
	}

	return &Config{
		Port:            port,
		DatabaseURL:     dbURL,
		RedisURL:        redisURL,
		CaddyAdminURL:   caddyAdminURL,
		BaseDomain:      baseDomain,
		MasterKey:       masterKey,
		JWTSecret:       jwtSecret,
		JWTExpiresIn:    jwtExpiresIn,
		DockerNetwork:   dockerNetwork,
		BuildStorageDir: buildStorageDir,
		Environment:     environment,
		FrontendURL:     frontendURL,
	}
}

func getEnv(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok && value != "" {
		return value
	}
	return fallback
}
