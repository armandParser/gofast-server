package main

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/viper"
)

// Config holds all configuration for GoFast server
type Config struct {
	// Server settings
	Host string `mapstructure:"host"`
	Port int    `mapstructure:"port"`

	// Performance settings
	MaxMemory  string        `mapstructure:"max_memory"`
	MaxClients int           `mapstructure:"max_clients"`
	Timeout    time.Duration `mapstructure:"timeout"`

	// Logging
	LogLevel  string `mapstructure:"log_level"`
	LogFormat string `mapstructure:"log_format"`

	// Persistence
	SaveInterval  time.Duration `mapstructure:"save_interval"`
	DataDir       string        `mapstructure:"data_dir"`
	EnablePersist bool          `mapstructure:"enable_persist"`

	// Security
	RequireAuth bool   `mapstructure:"require_auth"`
	Password    string `mapstructure:"password"`

	// Advanced
	TCPKeepAlive bool          `mapstructure:"tcp_keepalive"`
	ReadTimeout  time.Duration `mapstructure:"read_timeout"`
	WriteTimeout time.Duration `mapstructure:"write_timeout"`
}

// DefaultConfig returns a Config with default values
func DefaultConfig() *Config {
	return &Config{
		Host:          "localhost",
		Port:          6379,
		MaxMemory:     "1GB",
		MaxClients:    10000,
		Timeout:       30 * time.Second,
		LogLevel:      "info",
		LogFormat:     "text",
		SaveInterval:  300 * time.Second, // 5 minutes
		DataDir:       "./data",
		EnablePersist: false,
		RequireAuth:   false,
		Password:      "",
		TCPKeepAlive:  true,
		ReadTimeout:   30 * time.Second,
		WriteTimeout:  30 * time.Second,
	}
}

// LoadConfig loads configuration from environment variables, config file, and command line flags
func LoadConfig() (*Config, error) {
	config := DefaultConfig()

	// Set up Viper
	viper.SetConfigName("gofast")
	viper.SetConfigType("yaml")
	viper.AddConfigPath(".")
	viper.AddConfigPath("/etc/gofast/")
	viper.AddConfigPath("$HOME/.gofast")

	// Environment variables
	viper.SetEnvPrefix("GOFAST")
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_", "-", "_"))
	viper.AutomaticEnv()

	// Set defaults
	viper.SetDefault("host", config.Host)
	viper.SetDefault("port", config.Port)
	viper.SetDefault("max_memory", config.MaxMemory)
	viper.SetDefault("max_clients", config.MaxClients)
	viper.SetDefault("timeout", config.Timeout)
	viper.SetDefault("log_level", config.LogLevel)
	viper.SetDefault("log_format", config.LogFormat)
	viper.SetDefault("save_interval", config.SaveInterval)
	viper.SetDefault("data_dir", config.DataDir)
	viper.SetDefault("enable_persist", config.EnablePersist)
	viper.SetDefault("require_auth", config.RequireAuth)
	viper.SetDefault("password", config.Password)
	viper.SetDefault("tcp_keepalive", config.TCPKeepAlive)
	viper.SetDefault("read_timeout", config.ReadTimeout)
	viper.SetDefault("write_timeout", config.WriteTimeout)

	// Read config file (optional)
	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, fmt.Errorf("error reading config file: %w", err)
		}
		// Config file not found is OK
	}

	// Unmarshal into struct
	if err := viper.Unmarshal(config); err != nil {
		return nil, fmt.Errorf("error unmarshaling config: %w", err)
	}

	return config, nil
}

// Validate validates the configuration
func (c *Config) Validate() error {
	if c.Port < 1 || c.Port > 65535 {
		return fmt.Errorf("invalid port: %d (must be 1-65535)", c.Port)
	}

	if c.MaxClients < 1 {
		return fmt.Errorf("max_clients must be at least 1")
	}

	validLogLevels := []string{"trace", "debug", "info", "warn", "error", "fatal"}
	validLevel := false
	for _, level := range validLogLevels {
		if c.LogLevel == level {
			validLevel = true
			break
		}
	}
	if !validLevel {
		return fmt.Errorf("invalid log_level: %s (must be one of: %s)",
			c.LogLevel, strings.Join(validLogLevels, ", "))
	}

	return nil
}

// ParseMemorySize converts human-readable memory size to bytes
func (c *Config) ParseMemorySize() (int64, error) {
	size := strings.ToUpper(c.MaxMemory)

	if size == "" {
		return 0, nil // No limit
	}

	multiplier := int64(1)
	if strings.HasSuffix(size, "KB") {
		multiplier = 1024
		size = strings.TrimSuffix(size, "KB")
	} else if strings.HasSuffix(size, "MB") {
		multiplier = 1024 * 1024
		size = strings.TrimSuffix(size, "MB")
	} else if strings.HasSuffix(size, "GB") {
		multiplier = 1024 * 1024 * 1024
		size = strings.TrimSuffix(size, "GB")
	}

	value, err := strconv.ParseInt(size, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid memory size: %s", c.MaxMemory)
	}

	return value * multiplier, nil
}

// String returns a string representation of the config
func (c *Config) String() string {
	return fmt.Sprintf("GoFast Config: %s:%d, MaxMemory: %s, LogLevel: %s",
		c.Host, c.Port, c.MaxMemory, c.LogLevel)
}
