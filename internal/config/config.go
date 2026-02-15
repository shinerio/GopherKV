package config

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
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

type SaveRule struct {
	Seconds int `yaml:"seconds"`
	Changes int `yaml:"changes"`
}

type RDBConfig struct {
	Enabled   bool       `yaml:"enabled"`
	FilePath  string     `yaml:"file_path"`
	SaveRules []SaveRule `yaml:"save_rules"`
}

type LogConfig struct {
	Level string `yaml:"level"`
}

func Default() Config {
	return Config{
		Server:  ServerConfig{Port: 6380, ReadTimeout: 5 * time.Second, WriteTimeout: 5 * time.Second, ShutdownTimeout: 30 * time.Second},
		Storage: StorageConfig{ShardCount: 256, MaxKeySize: 256, MaxValueSize: 1024 * 1024, MaxMemory: 256 * 1024 * 1024},
		AOF:     AOFConfig{Enabled: true, FilePath: "./data/appendonly.aof", RewriteThreshold: 64 * 1024 * 1024},
		RDB:     RDBConfig{Enabled: true, FilePath: "./data/dump.rdb", SaveRules: []SaveRule{{Seconds: 900, Changes: 1}, {Seconds: 300, Changes: 10}, {Seconds: 60, Changes: 10000}}},
		Log:     LogConfig{Level: "info"},
	}
}

func Load(path string) (Config, error) {
	cfg := Default()
	if path == "" {
		return cfg, nil
	}
	f, err := os.Open(path)
	if err != nil {
		return cfg, fmt.Errorf("read config: %w", err)
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	section := ""
	inSaveRules := false
	var pendingRule *SaveRule

	for scanner.Scan() {
		line := scanner.Text()
		if idx := strings.Index(line, "#"); idx >= 0 {
			line = line[:idx]
		}
		if strings.TrimSpace(line) == "" {
			continue
		}
		trim := strings.TrimSpace(line)
		if !strings.HasPrefix(line, " ") && strings.HasSuffix(trim, ":") {
			section = strings.TrimSuffix(trim, ":")
			inSaveRules = false
			continue
		}

		if section == "rdb" && strings.HasPrefix(trim, "save_rules:") {
			inSaveRules = true
			cfg.RDB.SaveRules = nil
			continue
		}
		if inSaveRules {
			if strings.HasPrefix(trim, "-") {
				r := SaveRule{}
				cfg.RDB.SaveRules = append(cfg.RDB.SaveRules, r)
				pendingRule = &cfg.RDB.SaveRules[len(cfg.RDB.SaveRules)-1]
				trim = strings.TrimSpace(strings.TrimPrefix(trim, "-"))
				if trim != "" {
					k, v, ok := splitKV(trim)
					if ok {
						assignRuleKV(pendingRule, k, v)
					}
				}
				continue
			}
			if pendingRule != nil {
				k, v, ok := splitKV(trim)
				if ok {
					assignRuleKV(pendingRule, k, v)
					continue
				}
			}
		}

		k, v, ok := splitKV(trim)
		if !ok {
			continue
		}
		switch section {
		case "server":
			switch k {
			case "port":
				cfg.Server.Port = atoi(v, cfg.Server.Port)
			case "read_timeout":
				cfg.Server.ReadTimeout = parseDuration(v, cfg.Server.ReadTimeout)
			case "write_timeout":
				cfg.Server.WriteTimeout = parseDuration(v, cfg.Server.WriteTimeout)
			case "shutdown_timeout":
				cfg.Server.ShutdownTimeout = parseDuration(v, cfg.Server.ShutdownTimeout)
			}
		case "storage":
			switch k {
			case "shard_count":
				cfg.Storage.ShardCount = atoi(v, cfg.Storage.ShardCount)
			case "max_key_size":
				cfg.Storage.MaxKeySize = atoi(v, cfg.Storage.MaxKeySize)
			case "max_value_size":
				cfg.Storage.MaxValueSize = atoi(v, cfg.Storage.MaxValueSize)
			case "max_memory":
				cfg.Storage.MaxMemory = int64(atoi(v, int(cfg.Storage.MaxMemory)))
			}
		case "aof":
			switch k {
			case "enabled":
				cfg.AOF.Enabled = parseBool(v, cfg.AOF.Enabled)
			case "file_path":
				cfg.AOF.FilePath = unquote(v)
			case "rewrite_threshold":
				cfg.AOF.RewriteThreshold = int64(atoi(v, int(cfg.AOF.RewriteThreshold)))
			}
		case "rdb":
			switch k {
			case "enabled":
				cfg.RDB.Enabled = parseBool(v, cfg.RDB.Enabled)
			case "file_path":
				cfg.RDB.FilePath = unquote(v)
			}
		case "log":
			if k == "level" {
				cfg.Log.Level = unquote(v)
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return cfg, fmt.Errorf("scan config: %w", err)
	}
	return cfg, nil
}

func splitKV(s string) (string, string, bool) {
	parts := strings.SplitN(s, ":", 2)
	if len(parts) != 2 {
		return "", "", false
	}
	return strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1]), true
}

func assignRuleKV(rule *SaveRule, k, v string) {
	switch k {
	case "seconds":
		rule.Seconds = atoi(v, rule.Seconds)
	case "changes":
		rule.Changes = atoi(v, rule.Changes)
	}
}

func atoi(s string, fallback int) int {
	i, err := strconv.Atoi(unquote(s))
	if err != nil {
		return fallback
	}
	return i
}

func parseDuration(s string, fallback time.Duration) time.Duration {
	d, err := time.ParseDuration(unquote(s))
	if err != nil {
		return fallback
	}
	return d
}

func parseBool(s string, fallback bool) bool {
	b, err := strconv.ParseBool(unquote(s))
	if err != nil {
		return fallback
	}
	return b
}

func unquote(s string) string {
	s = strings.TrimSpace(s)
	s = strings.Trim(s, "\"")
	s = strings.Trim(s, "'")
	return s
}
