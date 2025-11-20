package configs

import (
	"gopkg.in/yaml.v3"
	"os"
)

type Config struct {
	Server struct {
		Port string `yaml:"port"`
	} `yaml:"server"`
}

func NewConfig() (*Config, error) {
	configFile, err := os.ReadFile("config.yaml")
	if err != nil {
		return nil, err
	}

	var cfg Config
	if err := yaml.Unmarshal(configFile, &cfg); err != nil {
		return nil, err
	}

	if envPort := os.Getenv("SERVER_PORT"); envPort != "" {
		cfg.Server.Port = envPort
	}

	return &cfg, nil
}
