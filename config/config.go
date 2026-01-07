package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
)

const defaultEnvPath = ".env"

// Config 聚合服务运行所需的配置。
type Config struct {
	Server  ServerConfig  `json:"server"`
	DB      DBConfig      `json:"db"`
	Redis   RedisConfig   `json:"redis"`
	Session SessionConfig `json:"session"`
	Storage StorageConfig `json:"storage"`
}

type ServerConfig struct {
	Addr string `json:"addr"`
}

type DBConfig struct {
	Host         string `json:"host"`
	Port         int    `json:"port"`
	User         string `json:"user"`
	Password     string `json:"password"`
	Database     string `json:"database"`
	MaxOpenConns int    `json:"maxOpenConns"`
}

type RedisConfig struct {
	Addr     string `json:"addr"`
	Password string `json:"password"`
}

type SessionConfig struct {
	Name   string `json:"name"`
	Secret string `json:"secret"`
	MaxAge int    `json:"maxAge"`
	Secure bool   `json:"secure"`
}

type StorageConfig struct {
	TmpDir string `json:"tmpDir"`
}

var (
	loaded Config
	errCfg error
	once   sync.Once
)

// Load 读取 .env 与环境变量，叠加默认值。调用多次只会加载一次。
func Load() (Config, error) {
	once.Do(func() {
		loaded, errCfg = loadConfig()
	})
	return loaded, errCfg
}

// MustLoad 返回配置，若加载失败则 panic。
func MustLoad() Config {
	cfg, err := Load()
	if err != nil {
		panic(err)
	}
	return cfg
}

// DSN 按当前配置生成 MySQL 连接串。
func (c DBConfig) DSN() string {
	host := c.Host
	if c.Port > 0 {
		host = fmt.Sprintf("%s:%d", c.Host, c.Port)
	}
	return fmt.Sprintf("%s:%s@tcp(%s)/%s?parseTime=true", c.User, c.Password, host, c.Database)
}

func loadConfig() (Config, error) {
	if err := loadDotEnv(); err != nil {
		return Config{}, err
	}

	cfg := defaultConfig()

	if err := applyEnvOverrides(&cfg); err != nil {
		return Config{}, err
	}

	return cfg, nil
}

func defaultConfig() Config {
	return Config{
		Server: ServerConfig{
			Addr: ":8080",
		},
		DB: DBConfig{
			Host:         "127.0.0.1",
			Port:         3306,
			User:         "root",
			Password:     "master_root_password",
			Database:     "filestore",
			MaxOpenConns: 1000,
		},
		Redis: RedisConfig{
			Addr:     "127.0.0.1:6379",
			Password: "testupload",
		},
		Session: SessionConfig{
			Name:   "filestore_session",
			Secret: "filestore-session-secret",
			MaxAge: 86400 * 7,
			Secure: false,
		},
		Storage: StorageConfig{
			TmpDir: "./tmp",
		},
	}
}

func envPath() (string, bool) {
	if path := os.Getenv("ENV_FILE"); path != "" {
		return filepath.Clean(path), true
	}
	return defaultEnvPath, false
}

func loadDotEnv() error {
	path, explicit := envPath()
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) && !explicit {
			return nil
		}
		return fmt.Errorf("read env file %s: %w", path, err)
	}

	lines := bytesToLines(data)
	for _, line := range lines {
		line = trimSpaces(line)
		if line == "" || line[0] == '#' {
			continue
		}
		parts := splitOnce(line, '=')
		if len(parts) != 2 {
			continue
		}
		key := trimSpaces(parts[0])
		val := trimSpaces(parts[1])
		val = stripQuotes(val)

		if key == "" {
			continue
		}
		if _, exists := os.LookupEnv(key); exists {
			continue
		}
		_ = os.Setenv(key, val)
	}
	return nil
}

func bytesToLines(b []byte) []string {
	var lines []string
	start := 0
	for i, c := range b {
		if c == '\n' || c == '\r' {
			lines = append(lines, string(b[start:i]))
			if c == '\r' && i+1 < len(b) && b[i+1] == '\n' {
				start = i + 2
				i++
				continue
			}
			start = i + 1
		}
	}
	if start <= len(b)-1 {
		lines = append(lines, string(b[start:]))
	}
	return lines
}

func trimSpaces(s string) string {
	return strings.TrimSpace(s)
}

func splitOnce(s string, sep byte) []string {
	for i := 0; i < len(s); i++ {
		if s[i] == sep {
			return []string{s[:i], s[i+1:]}
		}
	}
	return []string{s}
}

func stripQuotes(s string) string {
	if len(s) >= 2 {
		if (s[0] == '"' && s[len(s)-1] == '"') || (s[0] == '\'' && s[len(s)-1] == '\'') {
			return s[1 : len(s)-1]
		}
	}
	return s
}

func applyEnvOverrides(cfg *Config) error {
	setString := func(env string, target *string) {
		if v := os.Getenv(env); v != "" {
			*target = v
		}
	}

	setString("SERVER_ADDR", &cfg.Server.Addr)

	setString("DB_HOST", &cfg.DB.Host)
	if v := os.Getenv("DB_PORT"); v != "" {
		port, err := strconv.Atoi(v)
		if err != nil {
			return fmt.Errorf("invalid DB_PORT: %w", err)
		}
		cfg.DB.Port = port
	}
	setString("DB_USER", &cfg.DB.User)
	setString("DB_PASSWORD", &cfg.DB.Password)
	setString("DB_NAME", &cfg.DB.Database)
	if v := os.Getenv("DB_MAX_OPEN_CONNS"); v != "" {
		max, err := strconv.Atoi(v)
		if err != nil {
			return fmt.Errorf("invalid DB_MAX_OPEN_CONNS: %w", err)
		}
		cfg.DB.MaxOpenConns = max
	}

	setString("REDIS_ADDR", &cfg.Redis.Addr)
	setString("REDIS_PASSWORD", &cfg.Redis.Password)

	setString("SESSION_NAME", &cfg.Session.Name)
	setString("SESSION_SECRET", &cfg.Session.Secret)
	if v := os.Getenv("SESSION_MAX_AGE"); v != "" {
		age, err := strconv.Atoi(v)
		if err != nil {
			return fmt.Errorf("invalid SESSION_MAX_AGE: %w", err)
		}
		cfg.Session.MaxAge = age
	}
	if v := os.Getenv("SESSION_SECURE"); v != "" {
		secure, err := strconv.ParseBool(v)
		if err != nil {
			return fmt.Errorf("invalid SESSION_SECURE: %w", err)
		}
		cfg.Session.Secure = secure
	}

	setString("STORAGE_TMP_DIR", &cfg.Storage.TmpDir)

	return nil
}
