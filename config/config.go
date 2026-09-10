package config

import (
	"errors"
	"log"
	"os"
	"time"

	"github.com/spf13/viper"
)

type Config struct {
	Server   ServerConfig
	Postgres PostgresConfig
	Logger   LoggerConfig
	JWT      JWTConfig
	Secrets  SecretsConfig
}

type ServerConfig struct {
	InternalPort string
	ExternalPort string
	RunMode      string
	Domain       string
	AppName      string
}

type LoggerConfig struct {
	FileFolderPath string
	Encoding       string
	Level          string
	Logger         string
	MaxLogAge      time.Duration
	MaxLogSize     int
}

type PostgresConfig struct {
	Host            string
	Port            string
	User            string
	Password        string
	DbName          string
	SSLMode         string
	MaxIdleConns    int
	MaxOpenConns    int
	ConnMaxLifetime time.Duration
}

type JWTConfig struct {
	AccessTokenExpireDuration  time.Duration
	RefreshTokenExpireDuration time.Duration
	Secret                     string
	RefreshSecret              string
}

// SecretsConfig holds keys for encrypting sensitive data at rest (SSH
// credentials, and anything similar later) — distinct from JWT, which
// signs tokens rather than encrypting stored data.
type SecretsConfig struct {
	// EncryptionKey is a base64-encoded 32-byte AES-256 key, consumed by
	// pkg/crypto.NewBox. The dev default in config-development.yml is not
	// safe for any real deployment.
	EncryptionKey string
}

func GetConfig() *Config {
	cfgPath := getConfigPath(os.Getenv("APP_ENV"))
	v, err := LoadConfig(cfgPath, "yml")
	if err != nil {
		log.Fatalf("Error in load config %v", err)
	}

	cfg, err := ParseConfig(v)
	if err != nil {
		log.Fatalf("Error in parse config %v", err)
	}

	envPort := os.Getenv("PORT")
	if envPort != "" {
		cfg.Server.ExternalPort = envPort
		log.Printf("Set external port from environment -> %s", cfg.Server.ExternalPort)
	} else {
		cfg.Server.ExternalPort = cfg.Server.InternalPort
		log.Printf("Environment variable PORT not set; using internal port value -> %s", cfg.Server.ExternalPort)
	}

	return cfg
}

func ParseConfig(v *viper.Viper) (*Config, error) {
	var cfg Config
	err := v.Unmarshal(&cfg)
	if err != nil {
		log.Printf("Unable to parse config: %v", err)
		return nil, err
	}
	return &cfg, nil
}
func LoadConfig(filename string, fileType string) (*viper.Viper, error) {
	v := viper.New()
	v.SetConfigType(fileType)
	v.SetConfigName(filename)
	v.AddConfigPath(".")
	v.AutomaticEnv()

	err := v.ReadInConfig()
	if err != nil {
		log.Printf("Unable to read config: %v", err)
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			return nil, errors.New("config file not found")
		}
		return nil, err
	}
	return v, nil
}

// getConfigPath is relative to the repo root, since cmd/server is meant to
// be run as `go run ./cmd/server` from there.
func getConfigPath(env string) string {
	switch env {
case "docker":
		return "/app/config/config-docker"
	case "production":
		return "/config/config-production"
	default:
		return "config/config-development"
	}
}
