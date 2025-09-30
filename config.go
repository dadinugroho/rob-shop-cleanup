package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"time"
)

type Config struct {
	DBHost     string
	DBPort     string
	DBUser     string
	DBPassword string
	DBName     string
	CutoffDate string
	DryRun     bool
	LogFile    string
}

// LoadConfig loads configuration from .env file and environment variables
func LoadConfig() (*Config, error) {
	// Load environment variables from .env file
	err := loadEnv(".env")
	if err != nil {
		return nil, fmt.Errorf("error loading .env file: %v", err)
	}

	// Generate log filename with current datetime
	currentTime := time.Now()
	defaultLogFile := fmt.Sprintf("cleanup_%s.log", currentTime.Format("20060102_150405"))

	config := &Config{
		DBHost:     getEnv("DB_HOST", "localhost"),
		DBPort:     getEnv("DB_PORT", "3306"),
		DBUser:     getEnv("DB_USER", ""),
		DBPassword: getEnv("DB_PASSWORD", ""),
		DBName:     getEnv("DB_NAME", ""),
		CutoffDate: getEnv("CUTOFF_DATE", "2024-01-01"),
		DryRun:     getEnv("DRY_RUN", "false") == "true",
		LogFile:    getEnv("LOG_FILE", defaultLogFile),
	}

	// Validate required configuration
	if config.DBUser == "" || config.DBPassword == "" || config.DBName == "" {
		return nil, fmt.Errorf("missing required database configuration. Please check your .env file")
	}

	return config, nil
}

// GetDSN returns the database connection string
func (c *Config) GetDSN() string {
	return fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?parseTime=true",
		c.DBUser, c.DBPassword, c.DBHost, c.DBPort, c.DBName)
}

// loadEnv loads environment variables from a file
func loadEnv(filename string) error {
	file, err := os.Open(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		
		// Skip empty lines and comments
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Split key=value
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}

		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])

		// Remove quotes if present
		if (strings.HasPrefix(value, "\"") && strings.HasSuffix(value, "\"")) ||
			(strings.HasPrefix(value, "'") && strings.HasSuffix(value, "'")) {
			value = value[1 : len(value)-1]
		}

		os.Setenv(key, value)
	}

	return scanner.Err()
}

// getEnv gets environment variable with fallback
func getEnv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
