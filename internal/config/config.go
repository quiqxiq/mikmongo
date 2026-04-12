package config

import (
	"log"
	"os"
	"strconv"
	"strings"

	"github.com/joho/godotenv"
)

// Config holds all configuration
type Config struct {
	App struct {
		Name           string
		Env            string
		Port           string
		AllowedOrigins []string // CORS and WebSocket allowed origins
	}
	DB struct {
		Host     string
		Port     string
		User     string
		Password string
		Name     string
		SSLMode  string
	}
	Redis struct {
		Host     string
		Port     string
		Password string
		DB       int
	}
	RabbitMQ struct {
		Host     string
		Port     int
		User     string
		Password string
	}
	JWT struct {
		Secret        string
		Expiry        string
		RefreshExpiry string
	}
	Mikrotik struct {
		Host     string
		Port     int
		User     string
		Password string
	}
	Midtrans struct {
		ServerKey   string
		ClientKey   string
		Environment string
	}
	Xendit struct {
		SecretKey    string
		WebhookToken string
	}
	GoWA struct {
		BaseURL  string
		Username string
		Password string
		DeviceID string
		GroupID  string
		Timeout  int
	}
	InfluxDB struct {
		URL    string
		Token  string
		Org    string
		Bucket string
	}
	InternalKey string
	Seed        struct {
		AutoMigrate    bool
		AdminEmail     string
		AdminPassword  string
		AdminName      string
		AdminPhone     string
		RouterName     string
		RouterAddress  string
		RouterAPIPort  int
		RouterUsername string
		RouterPassword string
	}
}

// Load loads configuration from .env file and environment variables
func Load() *Config {
	if err := godotenv.Load(); err != nil {
		log.Printf("Warning: .env file not found: %v", err)
	}

	config := &Config{}

	// App
	config.App.Name = env("APP_NAME", "MikMongo")
	config.App.Env = env("APP_ENV", "development")
	config.App.Port = env("APP_PORT", "8080")
	config.App.AllowedOrigins = parseOrigins(env("ALLOWED_ORIGINS", "*"))

	// DB
	config.DB.Host = env("DB_HOST", "localhost")
	config.DB.Port = env("DB_PORT", "5432")
	config.DB.User = env("DB_USER", "mikmongo")
	config.DB.Password = env("DB_PASSWORD", "mikmongo")
	config.DB.Name = env("DB_NAME", "mikmongo")
	config.DB.SSLMode = env("DB_SSLMODE", "disable")

	// Redis
	config.Redis.Host = env("REDIS_HOST", "localhost")
	config.Redis.Port = env("REDIS_PORT", "6379")
	config.Redis.Password = os.Getenv("REDIS_PASSWORD")
	config.Redis.DB = envInt("REDIS_DB", 0)

	// RabbitMQ
	config.RabbitMQ.Host = env("RABBITMQ_HOST", "localhost")
	config.RabbitMQ.Port = envInt("RABBITMQ_PORT", 5672)
	config.RabbitMQ.User = env("RABBITMQ_USER", "mikmongo")
	config.RabbitMQ.Password = env("RABBITMQ_PASSWORD", "mikmongo")

	// JWT
	config.JWT.Secret = env("JWT_SECRET", "your-secret-key-here")
	config.JWT.Expiry = env("JWT_EXPIRY", "1h")
	config.JWT.RefreshExpiry = env("JWT_REFRESH_EXPIRY", "168h")

	// Mikrotik
	config.Mikrotik.Host = os.Getenv("MIKROTIK_HOST")
	config.Mikrotik.Port = envInt("MIKROTIK_PORT", 8728)
	config.Mikrotik.User = os.Getenv("MIKROTIK_USER")
	config.Mikrotik.Password = os.Getenv("MIKROTIK_PASSWORD")

	// Midtrans
	config.Midtrans.ServerKey = os.Getenv("MIDTRANS_SERVER_KEY")
	config.Midtrans.ClientKey = os.Getenv("MIDTRANS_CLIENT_KEY")
	config.Midtrans.Environment = env("MIDTRANS_ENVIRONMENT", "sandbox")

	// Xendit
	config.Xendit.SecretKey = os.Getenv("XENDIT_SECRET_KEY")
	config.Xendit.WebhookToken = os.Getenv("XENDIT_WEBHOOK_SECRET")

	// GoWA
	config.GoWA.BaseURL = env("GOWA_BASE_URL", "http://localhost:3000")
	config.GoWA.Username = os.Getenv("GOWA_USERNAME")
	config.GoWA.Password = os.Getenv("GOWA_PASSWORD")
	config.GoWA.DeviceID = os.Getenv("GOWA_DEVICE_ID")
	config.GoWA.GroupID = os.Getenv("GOWA_GROUP_ID")
	config.GoWA.Timeout = envInt("GOWA_TIMEOUT", 30)

	// InfluxDB
	config.InfluxDB.URL = env("INFLUXDB_URL", "http://localhost:8086")
	config.InfluxDB.Token = os.Getenv("INFLUXDB_TOKEN")
	config.InfluxDB.Org = env("INFLUXDB_ORG", "mikmongo")
	config.InfluxDB.Bucket = env("INFLUXDB_BUCKET", "mikrotik")

	// Internal key for WebSocket auth bypass
	config.InternalKey = env("INTERNAL_KEY", "")

	// Seed
	config.Seed.AutoMigrate = envBool("AUTO_MIGRATE")
	config.Seed.AdminEmail = os.Getenv("SEED_ADMIN_EMAIL")
	config.Seed.AdminPassword = os.Getenv("SEED_ADMIN_PASSWORD")
	config.Seed.AdminName = os.Getenv("SEED_ADMIN_NAME")
	config.Seed.AdminPhone = os.Getenv("SEED_ADMIN_PHONE")
	config.Seed.RouterName = os.Getenv("SEED_ROUTER_NAME")
	config.Seed.RouterAddress = os.Getenv("SEED_ROUTER_ADDRESS")
	config.Seed.RouterAPIPort = envInt("SEED_ROUTER_API_PORT", 0)
	config.Seed.RouterUsername = os.Getenv("SEED_ROUTER_USERNAME")
	config.Seed.RouterPassword = os.Getenv("SEED_ROUTER_PASSWORD")

	return config
}

func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func envInt(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		if i, err := strconv.Atoi(v); err == nil {
			return i
		}
	}
	return fallback
}

func envBool(key string) bool {
	v := os.Getenv(key)
	return v == "true" || v == "1" || v == "yes"
}

// parseOrigins splits a comma-separated string of origins.
func parseOrigins(s string) []string {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	origins := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			origins = append(origins, p)
		}
	}
	return origins
}

// GetDSN returns database connection string
func (c *Config) GetDSN() string {
	return "postgres://" + c.DB.User + ":" + c.DB.Password +
		"@" + c.DB.Host + ":" + c.DB.Port +
		"/" + c.DB.Name + "?sslmode=" + c.DB.SSLMode
}
