package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/korotovsky/slack-mcp-server/pkg/provider"
	"github.com/korotovsky/slack-mcp-server/pkg/server"
	"github.com/mattn/go-isatty"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var defaultSseHost = "127.0.0.1"
var defaultSsePort = 13080

// ServerConfig holds all server configuration from environment variables
type ServerConfig struct {
	// Network configuration
	Host    string
	Port    string
	BaseURL string

	// Railway-specific configuration
	RailwayEnvironment string
	RailwayPort        string

	// Security configuration
	CORSOrigins     []string
	RateLimit       time.Duration
	SecurityHeaders bool
	HealthEnabled   bool
	PrivateNetwork  bool

	// Logging configuration
	LogLevel  string
	LogFormat string
	LogColor  bool
}

// loadServerConfig loads and validates server configuration from environment variables
func loadServerConfig() (*ServerConfig, error) {
	config := &ServerConfig{}

	// Railway-specific environment variables (automatically set by Railway)
	config.RailwayPort = os.Getenv("PORT")
	config.RailwayEnvironment = os.Getenv("RAILWAY_ENVIRONMENT")

	// Network configuration
	config.Host = os.Getenv("SLACK_MCP_HOST")
	config.Port = os.Getenv("SLACK_MCP_PORT")
	config.BaseURL = os.Getenv("SLACK_MCP_BASE_URL")

	// Apply Railway port precedence
	if config.RailwayPort != "" {
		config.Port = config.RailwayPort
	}

	// Set default port if none specified
	if config.Port == "" {
		config.Port = strconv.Itoa(defaultSsePort)
	}

	// Handle dual-stack binding for Railway deployment
	if config.Host == "" {
		if config.RailwayPort != "" || config.RailwayEnvironment != "" {
			// Empty host for dual-stack IPv4/IPv6 binding on Railway
			config.Host = ""
		} else {
			// Default to localhost for local development
			config.Host = defaultSseHost
		}
	}

	// Security configuration with validation
	corsOriginsStr := os.Getenv("SLACK_MCP_CORS_ORIGINS")
	if corsOriginsStr == "" {
		config.CORSOrigins = []string{"*"} // Default to allow all origins
	} else {
		config.CORSOrigins = strings.Split(corsOriginsStr, ",")
		for i, origin := range config.CORSOrigins {
			config.CORSOrigins[i] = strings.TrimSpace(origin)
		}
	}

	// Rate limiting configuration
	rateLimitStr := os.Getenv("SLACK_MCP_RATE_LIMIT")
	if rateLimitStr == "" {
		config.RateLimit = time.Minute // Default: 60 requests per minute
	} else {
		rateLimitInt, err := strconv.Atoi(rateLimitStr)
		if err != nil || rateLimitInt < 0 {
			return nil, fmt.Errorf("invalid SLACK_MCP_RATE_LIMIT value '%s': must be a non-negative integer", rateLimitStr)
		}

		// Handle special case: 0 means no rate limiting
		if rateLimitInt == 0 {
			config.RateLimit = 0 // Disabled
		} else {
			config.RateLimit = time.Minute / time.Duration(rateLimitInt)
		}
	}

	// Security headers configuration
	securityHeadersStr := os.Getenv("SLACK_MCP_SECURITY_HEADERS")
	config.SecurityHeaders = securityHeadersStr == "" || securityHeadersStr == "true" || securityHeadersStr == "1"

	// Health check configuration
	healthEnabledStr := os.Getenv("SLACK_MCP_HEALTH_ENABLED")
	config.HealthEnabled = healthEnabledStr == "" || healthEnabledStr == "true" || healthEnabledStr == "1"

	// Private network deployment detection
	privateNetworkStr := os.Getenv("SLACK_MCP_PRIVATE_NETWORK")
	config.PrivateNetwork = privateNetworkStr == "true" || privateNetworkStr == "1" ||
		config.RailwayEnvironment != "" || os.Getenv("SLACK_MCP_SSE_API_KEY") == ""

	// Logging configuration
	config.LogLevel = os.Getenv("SLACK_MCP_LOG_LEVEL")
	config.LogFormat = os.Getenv("SLACK_MCP_LOG_FORMAT")
	logColorStr := os.Getenv("SLACK_MCP_LOG_COLOR")
	config.LogColor = logColorStr == "true" || logColorStr == "1"

	return config, nil
}

// validateServerConfig validates the server configuration
func validateServerConfig(config *ServerConfig) error {
	// Validate port
	if _, err := strconv.Atoi(config.Port); err != nil {
		return fmt.Errorf("invalid port '%s': must be a valid integer", config.Port)
	}

	// Validate CORS origins
	for _, origin := range config.CORSOrigins {
		if origin == "" {
			return fmt.Errorf("invalid CORS origin: empty origin not allowed")
		}
	}

	// Validate rate limit (0 means disabled, which is allowed)
	if config.RateLimit < 0 {
		return fmt.Errorf("invalid rate limit: must be non-negative")
	}

	return nil
}

func main() {
	var transport string
	flag.StringVar(&transport, "t", "stdio", "Transport type (stdio or sse)")
	flag.StringVar(&transport, "transport", "stdio", "Transport type (stdio or sse)")
	flag.Parse()

	// Load and validate server configuration
	config, err := loadServerConfig()
	if err != nil {
		fmt.Printf("Configuration error: %v\n", err)
		os.Exit(1)
	}

	err = validateServerConfig(config)
	if err != nil {
		fmt.Printf("Configuration validation error: %v\n", err)
		os.Exit(1)
	}

	logger, err := newLogger(transport, config)
	if err != nil {
		panic(err)
	}
	defer logger.Sync()

	// Log configuration information for debugging
	logger.Info("Server configuration loaded",
		zap.String("context", "console"),
		zap.String("host", config.Host),
		zap.String("port", config.Port),
		zap.String("railway_environment", config.RailwayEnvironment),
		zap.Strings("cors_origins", config.CORSOrigins),
		zap.Duration("rate_limit_interval", config.RateLimit),
		zap.Bool("security_headers", config.SecurityHeaders),
		zap.Bool("health_enabled", config.HealthEnabled),
		zap.Bool("private_network", config.PrivateNetwork),
	)

	err = validateToolConfig(os.Getenv("SLACK_MCP_ADD_MESSAGE_TOOL"))
	if err != nil {
		logger.Fatal("error in SLACK_MCP_ADD_MESSAGE_TOOL",
			zap.String("context", "console"),
			zap.Error(err),
		)
	}

	p := provider.New(transport, logger)
	s := server.NewMCPServer(p, logger)

	go func() {
		var once sync.Once

		newUsersWatcher(p, &once, logger)()
		newChannelsWatcher(p, &once, logger)()
	}()

	switch transport {
	case "stdio":
		if err := s.ServeStdio(); err != nil {
			logger.Fatal("Server error",
				zap.String("context", "console"),
				zap.Error(err),
			)
		}
	case "sse":
		// Determine bind address for dual-stack or IPv4-only
		var bindAddr string
		if config.Host == "" {
			bindAddr = ":" + config.Port // Dual-stack binding
		} else {
			bindAddr = config.Host + ":" + config.Port // Specific host binding
		}

		sseServer := s.ServeSSEWithHealthChecks(bindAddr)

		// Log appropriate address information with enhanced IPv6 support
		if config.Host == "" {
			logger.Info("SSE server starting with dual-stack IPv4/IPv6 binding",
				zap.String("context", "console"),
				zap.String("port", config.Port),
				zap.String("bind_address", bindAddr),
				zap.Bool("railway_deployment", config.RailwayEnvironment != ""),
				zap.String("railway_environment", config.RailwayEnvironment),
			)
		} else {
			// Format IPv6 addresses properly in logs
			displayHost := config.Host
			if strings.Contains(config.Host, ":") && !strings.HasPrefix(config.Host, "[") {
				displayHost = "[" + config.Host + "]"
			}

			logger.Info("SSE server starting with specific host binding",
				zap.String("context", "console"),
				zap.String("host", displayHost),
				zap.String("port", config.Port),
				zap.String("bind_address", bindAddr),
				zap.String("server_url", fmt.Sprintf("http://%s:%s/sse", displayHost, config.Port)),
			)
		}

		// Log security and deployment configuration
		logger.Info("Security configuration active",
			zap.String("context", "console"),
			zap.Strings("cors_origins", config.CORSOrigins),
			zap.Duration("rate_limit_interval", config.RateLimit),
			zap.Bool("security_headers_enabled", config.SecurityHeaders),
			zap.Bool("health_checks_enabled", config.HealthEnabled),
			zap.Bool("private_network_mode", config.PrivateNetwork),
		)

		if ready, _ := p.IsReady(); !ready {
			logger.Info("Slack MCP Server is still warming up caches",
				zap.String("context", "console"),
			)
		}

		if err := sseServer.Start(bindAddr); err != nil {
			logger.Fatal("Server error",
				zap.String("context", "console"),
				zap.Error(err),
			)
		}
	default:
		logger.Fatal("Invalid transport type",
			zap.String("context", "console"),
			zap.String("transport", transport),
			zap.String("allowed", "stdio,sse"),
		)
	}
}

func newUsersWatcher(p *provider.ApiProvider, once *sync.Once, logger *zap.Logger) func() {
	return func() {
		logger.Info("Caching users collection...",
			zap.String("context", "console"),
		)

		if os.Getenv("SLACK_MCP_XOXP_TOKEN") == "demo" || (os.Getenv("SLACK_MCP_XOXC_TOKEN") == "demo" && os.Getenv("SLACK_MCP_XOXD_TOKEN") == "demo") {
			logger.Info("Demo credentials are set, skip",
				zap.String("context", "console"),
			)
			return
		}

		err := p.RefreshUsers(context.Background())
		if err != nil {
			logger.Fatal("Error booting provider",
				zap.String("context", "console"),
				zap.Error(err),
			)
		}

		ready, _ := p.IsReady()
		if ready {
			once.Do(func() {
				logger.Info("Slack MCP Server is fully ready",
					zap.String("context", "console"),
				)
			})
		}
	}
}

func newChannelsWatcher(p *provider.ApiProvider, once *sync.Once, logger *zap.Logger) func() {
	return func() {
		logger.Info("Caching channels collection...",
			zap.String("context", "console"),
		)

		if os.Getenv("SLACK_MCP_XOXP_TOKEN") == "demo" || (os.Getenv("SLACK_MCP_XOXC_TOKEN") == "demo" && os.Getenv("SLACK_MCP_XOXD_TOKEN") == "demo") {
			logger.Info("Demo credentials are set, skip.",
				zap.String("context", "console"),
			)
			return
		}

		err := p.RefreshChannels(context.Background())
		if err != nil {
			logger.Fatal("Error booting provider",
				zap.String("context", "console"),
				zap.Error(err),
			)
		}

		ready, _ := p.IsReady()
		if ready {
			once.Do(func() {
				logger.Info("Slack MCP Server is fully ready.",
					zap.String("context", "console"),
				)
			})
		}
	}
}

func validateToolConfig(config string) error {
	if config == "" || config == "true" || config == "1" {
		return nil
	}

	items := strings.Split(config, ",")
	hasNegated := false
	hasPositive := false

	for _, item := range items {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		if strings.HasPrefix(item, "!") {
			hasNegated = true
		} else {
			hasPositive = true
		}
	}

	if hasNegated && hasPositive {
		return fmt.Errorf("cannot mix allowed and disallowed (! prefixed) channels")
	}

	return nil
}

func newLogger(transport string, config *ServerConfig) (*zap.Logger, error) {
	atomicLevel := zap.NewAtomicLevelAt(zap.InfoLevel)
	if config.LogLevel != "" {
		if err := atomicLevel.UnmarshalText([]byte(config.LogLevel)); err != nil {
			fmt.Printf("Invalid log level '%s': %v, using 'info'\n", config.LogLevel, err)
		}
	}

	useJSON := shouldUseJSONFormat(config)
	useColors := shouldUseColors(config) && !useJSON

	outputPath := "stdout"
	if transport == "stdio" {
		outputPath = "stderr"
	}

	var zapConfig zap.Config

	if useJSON {
		zapConfig = zap.Config{
			Level:            atomicLevel,
			Development:      false,
			Encoding:         "json",
			OutputPaths:      []string{outputPath},
			ErrorOutputPaths: []string{"stderr"},
			EncoderConfig: zapcore.EncoderConfig{
				TimeKey:       "timestamp",
				LevelKey:      "level",
				NameKey:       "logger",
				MessageKey:    "message",
				StacktraceKey: "stacktrace",
				EncodeLevel:   zapcore.LowercaseLevelEncoder,
				EncodeTime:    zapcore.RFC3339TimeEncoder,
				EncodeCaller:  zapcore.ShortCallerEncoder,
			},
		}
	} else {
		zapConfig = zap.Config{
			Level:            atomicLevel,
			Development:      true,
			Encoding:         "console",
			OutputPaths:      []string{outputPath},
			ErrorOutputPaths: []string{"stderr"},
			EncoderConfig: zapcore.EncoderConfig{
				TimeKey:          "timestamp",
				LevelKey:         "level",
				NameKey:          "logger",
				MessageKey:       "msg",
				StacktraceKey:    "stacktrace",
				EncodeLevel:      getConsoleLevelEncoder(useColors),
				EncodeTime:       zapcore.ISO8601TimeEncoder,
				EncodeCaller:     zapcore.ShortCallerEncoder,
				ConsoleSeparator: " | ",
			},
		}
	}

	logger, err := zapConfig.Build(zap.AddCaller())
	if err != nil {
		return nil, err
	}

	logger = logger.With(zap.String("app", "slack-mcp-server"))

	return logger, err
}

// shouldUseJSONFormat determines if JSON format should be used
func shouldUseJSONFormat(config *ServerConfig) bool {
	if config.LogFormat != "" {
		return strings.ToLower(config.LogFormat) == "json"
	}

	// Railway deployment should use JSON format for better log aggregation
	if config.RailwayEnvironment != "" {
		return true
	}

	if env := os.Getenv("ENVIRONMENT"); env != "" {
		switch strings.ToLower(env) {
		case "production", "prod", "staging":
			return true
		case "development", "dev", "local":
			return false
		}
	}

	if os.Getenv("KUBERNETES_SERVICE_HOST") != "" ||
		os.Getenv("DOCKER_CONTAINER") != "" ||
		os.Getenv("container") != "" {
		return true
	}

	if !isatty.IsTerminal(os.Stdout.Fd()) {
		return true
	}

	return false
}

func shouldUseColors(config *ServerConfig) bool {
	if config.LogColor {
		return true
	}

	if os.Getenv("NO_COLOR") != "" {
		return false
	}

	if os.Getenv("FORCE_COLOR") != "" {
		return true
	}

	// Railway deployment should not use colors for better log readability
	if config.RailwayEnvironment != "" {
		return false
	}

	if env := os.Getenv("ENVIRONMENT"); env == "development" || env == "dev" {
		return isatty.IsTerminal(os.Stdout.Fd())
	}

	return isatty.IsTerminal(os.Stdout.Fd())
}

func getConsoleLevelEncoder(useColors bool) zapcore.LevelEncoder {
	if useColors {
		return zapcore.CapitalColorLevelEncoder
	}
	return zapcore.CapitalLevelEncoder
}
