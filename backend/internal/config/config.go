package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/andiq123/FindVibeFiber/internal/core/constants"
	"github.com/andiq123/FindVibeFiber/internal/utils"
)

type DatabaseConfig struct {
	DSN             string
	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxLifetime time.Duration
	ConnMaxIdleTime time.Duration
}

type HTTPConfig struct {
	Timeout        time.Duration
	MaxIdleConns   int
	MaxIdlePerHost int
	IdleTimeout    time.Duration
	// ProviderProxies — comma/newline list for Mp3pm/Mp3mn scrape rotation.
	// Also honors HTTP_PROXY / HTTPS_PROXY / ALL_PROXY via the scrape transport.
	ProviderProxies string
}

type ServerConfig struct {
	Port         string
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
	IdleTimeout  time.Duration
}

type SearchConfig struct {
	Timeout    time.Duration
	MaxResults int
}

type AppConfig struct {
	Database DatabaseConfig
	HTTP     HTTPConfig
	Server   ServerConfig
	Search   SearchConfig
}

func LoadConfig() *AppConfig {
	return &AppConfig{
		Database: loadDatabaseConfig(),
		HTTP:     loadHTTPConfig(),
		Server:   loadServerConfig(),
		Search:   loadSearchConfig(),
	}
}

func loadDatabaseConfig() DatabaseConfig {
	return DatabaseConfig{
		DSN:             databaseDSN(),
		MaxOpenConns:    parseIntEnv("DB_MAX_OPEN_CONNS", constants.DefaultDBMaxOpenConns),
		MaxIdleConns:    parseIntEnv("DB_MAX_IDLE_CONNS", constants.DefaultDBMaxIdleConns),
		ConnMaxLifetime: time.Duration(parseIntEnv("DB_CONN_MAX_LIFETIME_MIN", constants.DefaultDBConnMaxLifetime)) * time.Minute,
		ConnMaxIdleTime: time.Duration(parseIntEnv("DB_CONN_MAX_IDLE_TIME_MIN", constants.DefaultDBConnMaxIdleTime)) * time.Minute,
	}
}

// ponytail: DATABASE_URL first (Render/Heroku), DB_* for local AlwaysData-style setups
func databaseDSN() string {
	if dsn := os.Getenv("DATABASE_URL"); dsn != "" {
		return dsn
	}
	port, err := strconv.Atoi(utils.GetEnvOrDef("DB_PORT", fmt.Sprintf("%d", constants.DefaultDBPort)))
	if err != nil {
		port = constants.DefaultDBPort
	}
	return fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		utils.GetEnvOrDef("DB_HOST", "localhost"),
		port,
		utils.GetEnvOrDef("DB_USER", "postgres"),
		utils.GetEnvOrDef("DB_PASSWORD", "postgres"),
		utils.GetEnvOrDef("DB_NAME", "findvibe"),
		utils.GetEnvOrDef("DB_SSLMODE", "disable"),
	)
}

func loadHTTPConfig() HTTPConfig {
	return HTTPConfig{
		Timeout:         time.Duration(parseIntEnv("HTTP_TIMEOUT_SEC", constants.DefaultHTTPTimeout)) * time.Second,
		MaxIdleConns:    parseIntEnv("HTTP_MAX_IDLE_CONNS", constants.DefaultHTTPMaxIdleConns),
		MaxIdlePerHost:  parseIntEnv("HTTP_MAX_IDLE_PER_HOST", constants.DefaultHTTPMaxIdlePerHost),
		IdleTimeout:     time.Duration(parseIntEnv("HTTP_IDLE_TIMEOUT_SEC", constants.DefaultHTTPIdleTimeout)) * time.Second,
		ProviderProxies: firstNonEmpty(os.Getenv("PROVIDER_PROXIES"), os.Getenv("MUZJAM_PROXIES")),
	}
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

func loadServerConfig() ServerConfig {
	return ServerConfig{
		Port:         utils.GetEnvOrDef("PORT", constants.DefaultServerPort),
		ReadTimeout:  time.Duration(parseIntEnv("SERVER_READ_TIMEOUT_SEC", constants.DefaultReadTimeout)) * time.Second,
		WriteTimeout: time.Duration(parseIntEnv("SERVER_WRITE_TIMEOUT_SEC", constants.DefaultWriteTimeout)) * time.Second,
		IdleTimeout:  time.Duration(parseIntEnv("SERVER_IDLE_TIMEOUT_SEC", constants.DefaultIdleTimeout)) * time.Second,
	}
}

func loadSearchConfig() SearchConfig {
	return SearchConfig{
		Timeout:    time.Duration(parseIntEnv("SEARCH_TIMEOUT_SEC", constants.DefaultSearchTimeout)) * time.Second,
		MaxResults: parseIntEnv("SEARCH_MAX_RESULTS", constants.DefaultMaxSearchResults),
	}
}

func parseIntEnv(key string, defaultValue int) int {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	if parsed, err := strconv.Atoi(value); err == nil {
		return parsed
	}
	return defaultValue
}
