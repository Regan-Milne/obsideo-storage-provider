package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

const (
	DefaultPort         = 3334
	DefaultReadTimeout  = 30
	DefaultWriteTimeout = 300
)

type Config struct {
	ProviderID string       `yaml:"provider_id"`
	Server     ServerConfig `yaml:"server"`
	Data       DataConfig   `yaml:"data"`
	Tokens     TokensConfig `yaml:"tokens"`
}

type ServerConfig struct {
	Host         string `yaml:"host"`
	Port         int    `yaml:"port"`
	ReadTimeout  int    `yaml:"read_timeout"`
	WriteTimeout int    `yaml:"write_timeout"`
}

type DataConfig struct {
	Path string `yaml:"path"`
}

type TokensConfig struct {
	PublicKeyPath string `yaml:"public_key_path"`
}

func Load(path string) (*Config, error) {
	cfg := &Config{
		Server: ServerConfig{
			Host:         "0.0.0.0",
			Port:         DefaultPort,
			ReadTimeout:  DefaultReadTimeout,
			WriteTimeout: DefaultWriteTimeout,
		},
		Data:   DataConfig{Path: "./data"},
		Tokens: TokensConfig{PublicKeyPath: "coordinator_pub.pem"},
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, nil
		}
		return nil, fmt.Errorf("read config %s: %w", path, err)
	}
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("parse config %s: %w", path, err)
	}
	return cfg, nil
}
