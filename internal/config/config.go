package config

import (
	"log"
	"os"
	"strconv"

	"github.com/joho/godotenv"
)

type Config struct {
	Port               string
	DatabaseURL        string
	RedisURL           string
	CaddyAdminURL      string
	BaseDomain         string
	MasterKey          string
	JWTSecret          string
	JWTExpiresIn       int64
	DockerNetwork      string
	BuildStorageDir    string
	Environment        string
	FrontendURL        string
	APIBaseURL         string

	// PayDunya Billing
	PaydunyaMasterKey  string
	PaydunyaPrivateKey string
	PaydunyaToken      string
	PaydunyaMode       string

	// OAuth2 Providers
	GithubClientID     string
	GithubClientSecret string
	GoogleClientID     string
	GoogleClientSecret string
	GitlabClientID     string
	GitlabClientSecret string
}

func Load() *Config {
	// Attempt to load .env from current dir or parent dir
	_ = godotenv.Load()
	_ = godotenv.Load("../.env")

	port := getEnv("PORT", "8080")
	dbURL := getEnv("DATABASE_URL", "postgres://shipr:shipr_secret_password@127.0.0.1:5432/shipr?sslmode=disable")
	redisURL := getEnv("REDIS_URL", "redis://127.0.0.1:6379")
	caddyAdminURL := getEnv("CADDY_ADMIN_URL", "http://localhost:2019")
	baseDomain := getEnv("BASE_DOMAIN", "shipr.local")
	masterKey := getEnv("CLOUDBOX_MASTER_KEY", "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef")
	jwtSecret := getEnv("JWT_SECRET", "super_secret_jwt_access_key_change_in_production_32bytes")
	dockerNetwork := getEnv("DOCKER_NETWORK", "shipr-network")
	buildStorageDir := getEnv("BUILD_STORAGE_DIR", "./tmp/shipr-builds")
	environment := getEnv("ENVIRONMENT", "development")
	frontendURL := getEnv("FRONTEND_URL", "http://localhost:3000")
	apiBaseURL := getEnv("API_BASE_URL", "http://localhost:8080")

	// PayDunya
	paydunyaMasterKey := getEnv("PAYDUNYA_MASTER_KEY", "")
	paydunyaPrivateKey := getEnv("PAYDUNYA_PRIVATE_KEY", "")
	paydunyaToken := getEnv("PAYDUNYA_TOKEN", "")
	paydunyaMode := getEnv("PAYDUNYA_MODE", "test")

	// OAuth2
	githubClientID := getEnv("GITHUB_CLIENT_ID", "")
	githubClientSecret := getEnv("GITHUB_CLIENT_SECRET", "")
	googleClientID := getEnv("GOOGLE_CLIENT_ID", "")
	googleClientSecret := getEnv("GOOGLE_CLIENT_SECRET", "")
	gitlabClientID := getEnv("GITLAB_CLIENT_ID", "")
	gitlabClientSecret := getEnv("GITLAB_CLIENT_SECRET", "")

	jwtExpiresInStr := getEnv("JWT_EXPIRES_IN", "86400")
	jwtExpiresIn, err := strconv.ParseInt(jwtExpiresInStr, 10, 64)
	if err != nil {
		jwtExpiresIn = 86400
	}

	if len(masterKey) != 64 {
		log.Printf("[CONFIG WARNING] CLOUDBOX_MASTER_KEY should ideally be 64 hex characters (32 bytes). Using key provided.")
	}

	return &Config{
		Port:               port,
		DatabaseURL:        dbURL,
		RedisURL:           redisURL,
		CaddyAdminURL:      caddyAdminURL,
		BaseDomain:         baseDomain,
		MasterKey:          masterKey,
		JWTSecret:          jwtSecret,
		JWTExpiresIn:       jwtExpiresIn,
		DockerNetwork:      dockerNetwork,
		BuildStorageDir:    buildStorageDir,
		Environment:        environment,
		FrontendURL:        frontendURL,
		APIBaseURL:         apiBaseURL,
		PaydunyaMasterKey:  paydunyaMasterKey,
		PaydunyaPrivateKey: paydunyaPrivateKey,
		PaydunyaToken:      paydunyaToken,
		PaydunyaMode:       paydunyaMode,
		GithubClientID:     githubClientID,
		GithubClientSecret: githubClientSecret,
		GoogleClientID:     googleClientID,
		GoogleClientSecret: googleClientSecret,
		GitlabClientID:     gitlabClientID,
		GitlabClientSecret: gitlabClientSecret,
	}
}

func getEnv(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok && value != "" {
		return value
	}
	return fallback
}
