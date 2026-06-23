package config

import (
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/go-sql-driver/mysql"
)

const defaultPort = 8080

type Config struct {
	Environment       string
	Address           string
	MySQLDSN          string
	DataKeyVersion    string
	DataKeyBase64     string
	DBMaxOpenConns    int
	DBMaxIdleConns    int
	DBConnLifetime    time.Duration
	DBPingTimeout     time.Duration
	DBQueryTimeout    time.Duration
	ReadHeaderTimeout time.Duration
	ReadTimeout       time.Duration
	WriteTimeout      time.Duration
	IdleTimeout       time.Duration
	ShutdownTimeout   time.Duration
}

func Load() (Config, error) {
	environment := os.Getenv("APP_ENV")
	if environment == "" {
		return Config{}, fmt.Errorf("APP_ENV is required")
	}
	if environment != "local" && environment != "test" && environment != "production" {
		return Config{}, fmt.Errorf("APP_ENV must be local, test, or production")
	}
	mysqlDSN := os.Getenv("MYSQL_DSN")
	if mysqlDSN == "" {
		return Config{}, fmt.Errorf("MYSQL_DSN is required")
	}
	mysqlDSN, err := normalizeMySQLDSN(mysqlDSN)
	if err != nil {
		return Config{}, err
	}
	port, err := intValueOrDefault("PORT", defaultPort)
	if err != nil {
		return Config{}, err
	}
	if port < 1 || port > 65535 {
		return Config{}, fmt.Errorf("PORT must be between 1 and 65535")
	}
	dataKeyBase64 := os.Getenv("DATA_ENCRYPTION_KEY_B64")
	if dataKeyBase64 == "" {
		return Config{}, fmt.Errorf("DATA_ENCRYPTION_KEY_B64 is required")
	}
	dataKeyVersion := os.Getenv("DATA_ENCRYPTION_KEY_VERSION")
	if dataKeyVersion == "" {
		dataKeyVersion = "v1"
	}

	return Config{
		Environment:       environment,
		Address:           fmt.Sprintf(":%d", port),
		MySQLDSN:          mysqlDSN,
		DataKeyVersion:    dataKeyVersion,
		DataKeyBase64:     dataKeyBase64,
		DBMaxOpenConns:    10,
		DBMaxIdleConns:    5,
		DBConnLifetime:    3 * time.Minute,
		DBPingTimeout:     10 * time.Second,
		DBQueryTimeout:    3 * time.Second,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       5 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       60 * time.Second,
		ShutdownTimeout:   10 * time.Second,
	}, nil
}

func normalizeMySQLDSN(value string) (string, error) {
	parsed, err := mysql.ParseDSN(value)
	if err != nil {
		return "", fmt.Errorf("parse MYSQL_DSN: %w", err)
	}
	parsed.ParseTime = true
	parsed.Loc = time.UTC
	if parsed.Params == nil {
		parsed.Params = make(map[string]string)
	}
	parsed.Params["time_zone"] = "'+00:00'"
	return parsed.FormatDSN(), nil
}

func intValueOrDefault(name string, fallback int) (int, error) {
	value := os.Getenv(name)
	if value == "" {
		return fallback, nil
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return 0, fmt.Errorf("parse %s: %w", name, err)
	}
	return parsed, nil
}
