package cfg

import (
	"fmt"
	"log"
	"os"
	"strconv"

	"github.com/google/uuid"
	"gopkg.in/yaml.v3"
)

const configPath = ".heimly/config/"
const configName = "cfg.yml"

type Config struct {
	ID        uuid.UUID `yaml:"id"`
	ShareData bool      `yaml:"share_data"`

	Port        int    `yaml:"-"`
	DatabaseURL string `yaml:"-"`
	CacheURL    string `yaml:"-"`
	StorageURL  string `yaml:"-"`
}

func Load() *Config {
	cfg := loadOrInitConfig()
	cfg.Port = loadPort()
	cfg.DatabaseURL = loadDatabaseURL()
	cfg.CacheURL = loadCacheURL()
	cfg.StorageURL = loadStorageURL()
	return cfg
}

func (c *Config) PrintSummary() {
	fmt.Println("==========================")
	fmt.Printf("Instance ID:\t%s\n", c.ID)
	fmt.Printf("Share data:\t%t\n", c.ShareData)
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

func loadOrInitConfig() *Config {
	data, err := os.ReadFile(configPath + configName)
	if err != nil {
		if os.IsNotExist(err) {
			cfg := &Config{
				ID:        uuid.New(),
				ShareData: loadShareData(),
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
