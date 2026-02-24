package cfg

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"log"
	"os"
	"strconv"
	"time"

	"github.com/google/uuid"
	"gopkg.in/yaml.v3"
)

const configPath = ".heimly/config/"
const configName = "cfg.yml"

type Config struct {
	ID        uuid.UUID `yaml:"id"`
	ShareData bool      `yaml:"share_data"`
	JWTSecret string    `yaml:"jwt_secret"`

	Port            int           `yaml:"-"`
	DatabaseURL     string        `yaml:"-"`
	CacheURL        string        `yaml:"-"`
	StorageURL      string        `yaml:"-"`
	AccessTokenTTL  time.Duration `yaml:"-"`
	RefreshTokenTTL time.Duration `yaml:"-"`
}

func Load() *Config {
	cfg := loadOrInitConfig()
	cfg.Port = loadPort()
	cfg.DatabaseURL = loadDatabaseURL()
	cfg.CacheURL = loadCacheURL()
	cfg.StorageURL = loadStorageURL()
	cfg.AccessTokenTTL = loadDurationEnv("ACCESS_TOKEN_TTL", 15*time.Minute)
	cfg.RefreshTokenTTL = loadDurationEnv("REFRESH_TOKEN_TTL", 30*24*time.Hour)
	return cfg
}

func (c *Config) PrintSummary() {
	fmt.Println("==========================")
	fmt.Printf("Instance ID:\t%s\n", c.ID)
	fmt.Printf("Share data:\t%t\n", c.ShareData)
	fmt.Printf("JWT secret:\t%s\n", sanitize(c.JWTSecret))
	fmt.Printf("Database:\t%s\n", sanitize(c.DatabaseURL))
	fmt.Printf("Cache:\t%s\n", sanitize(c.CacheURL))
	fmt.Printf("Storage:\t%s\n", sanitize(c.StorageURL))
	fmt.Println("==========================")
}

func sanitize(v string) string {
	if v == "" {
		return "(missing)"
	}
	return "[set]"
}

func loadPort() int {
	if p := os.Getenv("PORT"); p != "" {
		if port, err := strconv.Atoi(p); err == nil {
			return port
		}
		log.Fatalf("invalid PORT: %q", p)
	}
	return 8080
}

func loadDatabaseURL() string {
	if v := os.Getenv("DATABASE_URL"); v != "" {
		return v
	}
	log.Fatal("DATABASE_URL is required")
	return ""
}

func loadCacheURL() string {
	if v := os.Getenv("CACHE_URL"); v != "" {
		return v
	}
	log.Fatal("CACHE_URL is required")
	return ""
}

func loadStorageURL() string {
	if v := os.Getenv("STORAGE_URL"); v != "" {
		return v
	}
	log.Fatal("STORAGE_URL is required")
	return ""
}

func loadDurationEnv(key string, fallback time.Duration) time.Duration {
	if v := os.Getenv(key); v != "" {
		d, err := time.ParseDuration(v)
		if err != nil {
			log.Fatalf("invalid %s: %q", key, v)
		}
		if d <= 0 {
			log.Fatalf("%s must be greater than zero", key)
		}
		return d
	}
	return fallback
}

func loadShareData() bool {
	if v := os.Getenv("SHARE_DATA"); v != "" {
		switch v {
		case "1", "true", "TRUE", "yes", "YES", "on", "ON":
			return true
		case "0", "false", "FALSE", "no", "NO", "off", "OFF":
			return false
		default:
			log.Fatalf("invalid SHARE_DATA: %q", v)
		}
	}
	return true
}

func loadOrGenerateJWTSecret() string {
	if v := os.Getenv("JWT_SECRET"); v != "" {
		return v
	}

	secretBytes := make([]byte, 32)
	if _, err := rand.Read(secretBytes); err != nil {
		log.Fatalf("failed to generate JWT secret: %v", err)
	}

	return base64.StdEncoding.EncodeToString(secretBytes)
}

func loadOrInitConfig() *Config {
	data, err := os.ReadFile(configPath + configName)
	if err != nil {
		if os.IsNotExist(err) {
			cfg := &Config{
				ID:        uuid.New(),
				ShareData: loadShareData(),
				JWTSecret: loadOrGenerateJWTSecret(),
			}
			if err := saveConfig(cfg); err != nil {
				panic(err)
			}
			return cfg
		}
		panic(err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		panic(err)
	}

	if cfg.JWTSecret == "" {
		cfg.JWTSecret = loadOrGenerateJWTSecret()
		if err := saveConfig(&cfg); err != nil {
			panic(err)
		}
	}

	return &cfg
}

func saveConfig(cfg *Config) error {
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(configPath, 0700); err != nil {
		return err
	}
	return os.WriteFile(configPath+configName, data, 0600)
}
