package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

type Config struct {
	HTTPAddr                string
	LogLevel                string
	ShutdownTimeout         time.Duration
	DatabaseURL             string
	DatabaseMaxConns        int32
	DatabaseMinConns        int32
	DatabaseMaxConnLifetime time.Duration
	DatabaseConnectTimeout  time.Duration
	DatabaseQueryTimeout    time.Duration
}

func Load() (Config, error) {
	httpAddress, err := requiredString("HTTP_ADDR")
	if err != nil {
		return Config{}, err
	}

	logLevel, err := requiredString("LOG_LEVEL")
	if err != nil {
		return Config{}, err
	}

	shutdownTimeout, err := requiredTime("SHUTDOWN_TIMEOUT")
	if err != nil {
		return Config{}, err
	}

	databaseURL, err := requiredString("DATABASE_URL")
	if err != nil {
		return Config{}, err
	}

	databaseMaxConns, err := requiredInt("DATABASE_MAX_CONNS")
	if err != nil {
		return Config{}, err
	}

	databaseMinConns, err := requiredInt("DATABASE_MIN_CONNS")
	if err != nil {
		return Config{}, err
	}

	databaseMaxConnLifetime, err := requiredTime("DATABASE_MAX_CONN_LIFETIME")
	if err != nil {
		return Config{}, err
	}

	databaseConnectTimeout, err := requiredTime("DATABASE_CONNECT_TIMEOUT")
	if err != nil {
		return Config{}, err
	}

	databaseQueryTimeout, err := requiredTime("DATABASE_QUERY_TIMEOUT")
	if err != nil {
		return Config{}, err
	}

	return Config{
		HTTPAddr:                httpAddress,
		LogLevel:                logLevel,
		ShutdownTimeout:         shutdownTimeout,
		DatabaseURL:             databaseURL,
		DatabaseMaxConns:        databaseMaxConns,
		DatabaseMinConns:        databaseMinConns,
		DatabaseMaxConnLifetime: databaseMaxConnLifetime,
		DatabaseConnectTimeout:  databaseConnectTimeout,
		DatabaseQueryTimeout:    databaseQueryTimeout,
	}, nil
}

func requiredString(param string) (string, error) {
	value := os.Getenv(param)
	if value == "" {
		return "", fmt.Errorf("%s is required", param)
	}

	return value, nil
}

func requiredInt(param string) (int32, error) {
	value, err := requiredString(param)
	if err != nil {
		return 0, err
	}

	result, err := strconv.ParseInt(value, 10, 32)

	if err != nil {
		return 0, fmt.Errorf("%s must be int: %w", param, err)
	}

	return int32(result), nil
}

func requiredTime(param string) (time.Duration, error) {
	value, err := requiredString(param)
	if err != nil {
		return 0, err
	}

	result, err := time.ParseDuration(value)
	if err != nil {
		return 0, fmt.Errorf("%s must be time: %w", param, err)
	}

	return result, nil
}
