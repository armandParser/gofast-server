package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"runtime"
	"strings"
	"syscall"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	version = "1.0.0" // Set during build with -ldflags
	config  *Config
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "gofast-server",
	Short: "GoFast - High-performance in-memory cache server",
	Long: `GoFast is a high-performance, distributed, in-memory cache system 
built in Go that rivals Redis in speed and functionality.

Features:
- High Performance: 100k+ operations/second
- Redis-Compatible: Familiar commands and data structures  
- Multiple Data Types: Strings, Lists, Sets, Hashes
- Pipeline Support: Batch operations for maximum throughput
- Pattern Matching: KEYS and SCAN operations
- TTL Support: Automatic expiration of keys`,
	Version: version,
	RunE:    runServer,
}

// runServer starts the GoFast server
func runServer(cmd *cobra.Command, args []string) error {
	// Load configuration
	var err error
	config, err = LoadConfig()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Validate configuration
	if err := config.Validate(); err != nil {
		return fmt.Errorf("invalid configuration: %w", err)
	}

	// Print startup info
	fmt.Printf("🚀 Starting GoFast Server v%s\n", version)
	fmt.Printf("📡 Listening on %s:%d\n", config.Host, config.Port)
	fmt.Printf("💾 Max Memory: %s\n", config.MaxMemory)
	fmt.Printf("📊 Log Level: %s\n", config.LogLevel)
	if config.EnablePersist {
		fmt.Printf("💽 Persistence: Enabled (save every %v)\n", config.SaveInterval)
		fmt.Printf("📁 Data Directory: %s\n", config.DataDir)
	}

	fmt.Println(strings.Repeat("=", 51))

	// Create and start server
	server := NewGoFastServer(config.Port)
	server.SetConfig(config) // We'll add this method

	// Handle graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Start server in goroutine
	go func() {
		if err := server.Start(); err != nil {
			log.Fatalf("Server failed to start: %v", err)
		}
	}()

	// Wait for shutdown signal
	<-sigChan
	fmt.Println("\n🛑 Shutting down GoFast server...")

	// Graceful shutdown
	server.Stop()
	fmt.Println("✅ GoFast server stopped")

	return nil
}

// configCmd shows current configuration
var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Show current configuration",
	RunE: func(cmd *cobra.Command, args []string) error {
		config, err := LoadConfig()
		if err != nil {
			return err
		}
		fmt.Println("GoFast Configuration:")
		fmt.Println(strings.Repeat("=", 31))
		fmt.Printf("Host: %s\n", config.Host)
		fmt.Printf("Host: %s\n", config.Host)
		fmt.Printf("Port: %d\n", config.Port)
		fmt.Printf("Max Memory: %s\n", config.MaxMemory)
		fmt.Printf("Max Clients: %d\n", config.MaxClients)
		fmt.Printf("Timeout: %v\n", config.Timeout)
		fmt.Printf("Log Level: %s\n", config.LogLevel)
		fmt.Printf("Log Format: %s\n", config.LogFormat)
		fmt.Printf("Save Interval: %v\n", config.SaveInterval)
		fmt.Printf("Data Directory: %s\n", config.DataDir)
		fmt.Printf("Persistence Enabled: %t\n", config.EnablePersist)
		fmt.Printf("Authentication Required: %t\n", config.RequireAuth)
		fmt.Printf("TCP Keep-Alive: %t\n", config.TCPKeepAlive)
		fmt.Printf("Read Timeout: %v\n", config.ReadTimeout)
		fmt.Printf("Write Timeout: %v\n", config.WriteTimeout)

		return nil
	},
}

// versionCmd shows version information
var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Show version information",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("GoFast Server v%s\n", version)
		fmt.Printf("Built with Go %s\n", runtime.Version())
		fmt.Printf("OS/Arch: %s/%s\n", runtime.GOOS, runtime.GOARCH)
	},
}

func init() {
	// Global flags
	rootCmd.PersistentFlags().StringP("host", "H", "localhost", "Host to bind to")
	rootCmd.PersistentFlags().IntP("port", "p", 6379, "Port to listen on")
	rootCmd.PersistentFlags().String("max-memory", "1GB", "Maximum memory to use (e.g., 512MB, 2GB)")
	rootCmd.PersistentFlags().Int("max-clients", 10000, "Maximum number of clients")
	rootCmd.PersistentFlags().Duration("timeout", 30*time.Second, "Client timeout")
	rootCmd.PersistentFlags().String("log-level", "info", "Log level (trace, debug, info, warn, error, fatal)")
	rootCmd.PersistentFlags().String("log-format", "text", "Log format (text, json)")
	rootCmd.PersistentFlags().Duration("save-interval", 300*time.Second, "Persistence save interval")
	rootCmd.PersistentFlags().String("data-dir", "./data", "Data directory for persistence")
	rootCmd.PersistentFlags().Bool("enable-persist", false, "Enable persistence to disk")
	rootCmd.PersistentFlags().Bool("require-auth", false, "Require authentication")
	rootCmd.PersistentFlags().String("password", "", "Authentication password")
	rootCmd.PersistentFlags().Bool("tcp-keepalive", true, "Enable TCP keep-alive")
	rootCmd.PersistentFlags().Duration("read-timeout", 30*time.Second, "Read timeout")
	rootCmd.PersistentFlags().Duration("write-timeout", 30*time.Second, "Write timeout")

	// Bind flags to viper
	viper.BindPFlag("host", rootCmd.PersistentFlags().Lookup("host"))
	viper.BindPFlag("port", rootCmd.PersistentFlags().Lookup("port"))
	viper.BindPFlag("max_memory", rootCmd.PersistentFlags().Lookup("max-memory"))
	viper.BindPFlag("max_clients", rootCmd.PersistentFlags().Lookup("max-clients"))
	viper.BindPFlag("timeout", rootCmd.PersistentFlags().Lookup("timeout"))
	viper.BindPFlag("log_level", rootCmd.PersistentFlags().Lookup("log-level"))
	viper.BindPFlag("log_format", rootCmd.PersistentFlags().Lookup("log-format"))
	viper.BindPFlag("save_interval", rootCmd.PersistentFlags().Lookup("save-interval"))
	viper.BindPFlag("data_dir", rootCmd.PersistentFlags().Lookup("data-dir"))
	viper.BindPFlag("enable_persist", rootCmd.PersistentFlags().Lookup("enable-persist"))
	viper.BindPFlag("require_auth", rootCmd.PersistentFlags().Lookup("require-auth"))
	viper.BindPFlag("password", rootCmd.PersistentFlags().Lookup("password"))
	viper.BindPFlag("tcp_keepalive", rootCmd.PersistentFlags().Lookup("tcp-keepalive"))
	viper.BindPFlag("read_timeout", rootCmd.PersistentFlags().Lookup("read-timeout"))
	viper.BindPFlag("write_timeout", rootCmd.PersistentFlags().Lookup("write-timeout"))

	// Add subcommands
	rootCmd.AddCommand(configCmd)
	rootCmd.AddCommand(versionCmd)
}

// Execute is the main entry point for the CLI
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
