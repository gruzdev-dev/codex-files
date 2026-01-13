package configs

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

type Config struct {
	HTTP struct {
		Port string
	}
	GRPC struct {
		Port string
	}
	Auth struct {
		JWTSecret      string
		InternalSecret string
	}
	DB struct {
		Host     string
		Port     string
		User     string
		Password string
		Database string
	}
	S3 struct {
		Endpoint      string
		AccessKey     string
		SecretKey     string
		Bucket        string
		ExternalHost  string
		UseSSL        bool
		WebhookSecret string
	}
	Upload struct {
		MaxSize int64
		TTL     time.Duration
	}
	Download struct {
		TTL time.Duration
	}
}

func NewConfig() (*Config, error) {
	var cfg Config

	if envPort := os.Getenv("HTTP_PORT"); envPort != "" {
		cfg.HTTP.Port = envPort
	}

	if envGRPCPort := os.Getenv("GRPC_PORT"); envGRPCPort != "" {
		cfg.GRPC.Port = envGRPCPort
	}

	if envJWTSecret := os.Getenv("JWT_SECRET"); envJWTSecret != "" {
		cfg.Auth.JWTSecret = envJWTSecret
	}

	if envInternalSecret := os.Getenv("INTERNAL_SERVICE_SECRET"); envInternalSecret != "" {
		cfg.Auth.InternalSecret = envInternalSecret
	}

	if envDBHost := os.Getenv("POSTGRES_HOST"); envDBHost != "" {
		cfg.DB.Host = envDBHost
	}

	if envDBPort := os.Getenv("POSTGRES_PORT"); envDBPort != "" {
		cfg.DB.Port = envDBPort
	} else {
		cfg.DB.Port = "5432"
	}

	if envDBUser := os.Getenv("POSTGRES_USER"); envDBUser != "" {
		cfg.DB.User = envDBUser
	}

	if envDBPassword := os.Getenv("POSTGRES_PASSWORD"); envDBPassword != "" {
		cfg.DB.Password = envDBPassword
	}

	if envDBDatabase := os.Getenv("POSTGRES_DB"); envDBDatabase != "" {
		cfg.DB.Database = envDBDatabase
	}

	if envS3Endpoint := os.Getenv("S3_ENDPOINT"); envS3Endpoint != "" {
		cfg.S3.Endpoint = envS3Endpoint
	}

	if envS3AccessKey := os.Getenv("S3_ACCESS_KEY"); envS3AccessKey != "" {
		cfg.S3.AccessKey = envS3AccessKey
	}

	if envS3SecretKey := os.Getenv("S3_SECRET_KEY"); envS3SecretKey != "" {
		cfg.S3.SecretKey = envS3SecretKey
	}

	if envS3Bucket := os.Getenv("S3_BUCKET"); envS3Bucket != "" {
		cfg.S3.Bucket = envS3Bucket
	}

	if envS3ExternalHost := os.Getenv("S3_EXTERNAL_HOST"); envS3ExternalHost != "" {
		cfg.S3.ExternalHost = envS3ExternalHost
	}

	if envS3UseSSL := os.Getenv("S3_USE_SSL"); envS3UseSSL != "" {
		cfg.S3.UseSSL = envS3UseSSL == "true"
	}

	if envS3WebhookSecret := os.Getenv("S3_WEBHOOK_SECRET"); envS3WebhookSecret != "" {
		cfg.S3.WebhookSecret = envS3WebhookSecret
	}

	if envUploadMaxSize := os.Getenv("UPLOAD_MAX_SIZE"); envUploadMaxSize != "" {
		if size, err := strconv.ParseInt(envUploadMaxSize, 10, 64); err == nil {
			cfg.Upload.MaxSize = size
		}
	} else {
		cfg.Upload.MaxSize = 100 * 1024 * 1024
	}

	if envUploadTTL := os.Getenv("UPLOAD_TTL"); envUploadTTL != "" {
		if ttl, err := time.ParseDuration(envUploadTTL); err == nil {
			cfg.Upload.TTL = ttl
		}
	} else {
		cfg.Upload.TTL = 5 * time.Minute
	}

	if envDownloadTTL := os.Getenv("DOWNLOAD_TTL"); envDownloadTTL != "" {
		if ttl, err := time.ParseDuration(envDownloadTTL); err == nil {
			cfg.Download.TTL = ttl
		}
	} else {
		cfg.Download.TTL = 15 * time.Minute
	}

	return &cfg, nil
}

func (c *Config) DatabaseURL() string {
	return fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=disable",
		c.DB.User, c.DB.Password, c.DB.Host, c.DB.Port, c.DB.Database)
}
