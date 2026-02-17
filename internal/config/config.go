package config

import (
	"fmt"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Server  ServerConfig  `yaml:"server"`
	Storage StorageConfig `yaml:"storage"`
	AOF     AOFConfig     `yaml:"aof"`
	RDB     RDBConfig     `yaml:"rdb"`
	Log     LogConfig     `yaml:"log"`
}

type ServerConfig struct {
	Port            int           `yaml:"port"`
	ReadTimeout     time.Duration `yaml:"read_timeout"`
	WriteTimeout    time.Duration `yaml:"write_timeout"`
	ShutdownTimeout time.Duration `yaml:"shutdown_timeout"`
}

type StorageConfig struct {
	ShardCount   int   `yaml:"shard_count"`
	MaxKeySize   int   `yaml:"max_key_size"`
	MaxValueSize int   `yaml:"max_value_size"`
	MaxMemory    int64 `yaml:"max_memory"`
}

type AOFConfig struct {
	Enabled          bool   `yaml:"enabled"`
	FilePath         string `yaml:"file_path"`
	RewriteThreshold int64  `yaml:"rewrite_threshold"`
}

type RDBConfig struct {
	Enabled   bool       `yaml:"enabled"`
	FilePath  string     `yaml:"file_path"`
	SaveRules []SaveRule `yaml:"save_rules"`
}

type SaveRule struct {
	Seconds int `yaml:"seconds"`
	Changes int `yaml:"changes"`
}

type LogConfig struct {
	Level string `yaml:"level"`
}

func DefaultConfig() *Config {
	return &Config{
		Server: ServerConfig{
			Port:            6380,
			ReadTimeout:     5 * time.Second,
			WriteTimeout:    5 * time.Second,
			ShutdownTimeout: 30 * time.Second,
		},
		Storage: StorageConfig{
			ShardCount:   256,
			MaxKeySize:   256,
			MaxValueSize: 1048576,
			MaxMemory:    268435456,
		},
		AOF: AOFConfig{
			Enabled:          true,
			FilePath:         "./data/appendonly.aof",
			RewriteThreshold: 67108864,
		},
		RDB: RDBConfig{
			Enabled:  true,
			FilePath: "./data/dump.rdb",
			SaveRules: []SaveRule{
				{Seconds: 900, Changes: 1},
				{Seconds: 300, Changes: 10},
				{Seconds: 60, Changes: 10000},
			},
		},
		Log: LogConfig{
			Level: "info",
		},
	}
}

func LoadConfig(path string) (*Config, error) {
	cfg := DefaultConfig()

	if path == "" {
		return cfg, nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, nil
		}
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	return cfg, nil
}
