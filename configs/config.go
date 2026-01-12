package configs

import (
	"fmt"
	"os"
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

	return &cfg, nil
}

func (c *Config) DatabaseURL() string {
	return fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=disable",
		c.DB.User, c.DB.Password, c.DB.Host, c.DB.Port, c.DB.Database)
}
