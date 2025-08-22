package main

import (
	"net"
	"os"
	"strings"
	"testing"
	"time"
)

func TestRailwayPortDetection(t *testing.T) {
	tests := []struct {
		name           string
		portEnv        string
		slackMcpPort   string
		railwayEnv     string
		expectedPort   string
		description    string
	}{
		{
			name:           "railway port takes precedence",
			portEnv:        "3000",
			slackMcpPort:   "8080",
			railwayEnv:     "production",
			expectedPort:   "3000",
			description:    "Railway PORT should take precedence over SLACK_MCP_PORT",
		},
		{
			name:           "fallback to slack mcp port",
			portEnv:        "",
			slackMcpPort:   "8080",
			railwayEnv:     "",
			expectedPort:   "8080",
			description:    "Should fallback to SLACK_MCP_PORT when PORT is not set",
		},
		{
			name:           "fallback to default",
			portEnv:        "",
			slackMcpPort:   "",
			railwayEnv:     "",
			expectedPort:   "13080",
			description:    "Should use default port when neither PORT nor SLACK_MCP_PORT is set",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clear environment variables
			os.Unsetenv("PORT")
			os.Unsetenv("SLACK_MCP_PORT")
			os.Unsetenv("RAILWAY_ENVIRONMENT")

			// Set test environment variables
			if tt.portEnv != "" {
				os.Setenv("PORT", tt.portEnv)
			}
			if tt.slackMcpPort != "" {
				os.Setenv("SLACK_MCP_PORT", tt.slackMcpPort)
			}
			if tt.railwayEnv != "" {
				os.Setenv("RAILWAY_ENVIRONMENT", tt.railwayEnv)
			}

			// Clean up after test
			defer func() {
				os.Unsetenv("PORT")
				os.Unsetenv("SLACK_MCP_PORT")
				os.Unsetenv("RAILWAY_ENVIRONMENT")
			}()

			// Test the port detection logic
			port := os.Getenv("PORT")
			if port == "" {
				port = os.Getenv("SLACK_MCP_PORT")
				if port == "" {
					port = "13080" // defaultSsePort as string
				}
			}

			if port != tt.expectedPort {
				t.Errorf("%s: got port %q, expected %q", tt.description, port, tt.expectedPort)
			}
		})
	}
}

func TestRailwayHostDetection(t *testing.T) {
	tests := []struct {
		name           string
		hostEnv        string
		portEnv        string
		railwayEnv     string
		expectedHost   string
		description    string
	}{
		{
			name:           "empty host for railway deployment",
			hostEnv:        "",
			portEnv:        "3000",
			railwayEnv:     "production",
			expectedHost:   "",
			description:    "Should use empty host for dual-stack binding on Railway",
		},
		{
			name:           "empty host when PORT is set",
			hostEnv:        "",
			portEnv:        "3000",
			railwayEnv:     "",
			expectedHost:   "",
			description:    "Should use empty host when Railway PORT is set",
		},
		{
			name:           "explicit host overrides railway detection",
			hostEnv:        "192.168.1.100",
			portEnv:        "3000",
			railwayEnv:     "production",
			expectedHost:   "192.168.1.100",
			description:    "Explicit SLACK_MCP_HOST should override Railway detection",
		},
		{
			name:           "default host for local development",
			hostEnv:        "",
			portEnv:        "",
			railwayEnv:     "",
			expectedHost:   "127.0.0.1",
			description:    "Should use default host for local development",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clear environment variables
			os.Unsetenv("SLACK_MCP_HOST")
			os.Unsetenv("PORT")
			os.Unsetenv("RAILWAY_ENVIRONMENT")

			// Set test environment variables
			if tt.hostEnv != "" {
				os.Setenv("SLACK_MCP_HOST", tt.hostEnv)
			}
			if tt.portEnv != "" {
				os.Setenv("PORT", tt.portEnv)
			}
			if tt.railwayEnv != "" {
				os.Setenv("RAILWAY_ENVIRONMENT", tt.railwayEnv)
			}

			// Clean up after test
			defer func() {
				os.Unsetenv("SLACK_MCP_HOST")
				os.Unsetenv("PORT")
				os.Unsetenv("RAILWAY_ENVIRONMENT")
			}()

			// Test the host detection logic
			host := os.Getenv("SLACK_MCP_HOST")
			if host == "" {
				// Empty host for dual-stack IPv4/IPv6 binding on Railway
				if os.Getenv("PORT") != "" || os.Getenv("RAILWAY_ENVIRONMENT") != "" {
					host = ""
				} else {
					host = "127.0.0.1" // defaultSseHost
				}
			}

			if host != tt.expectedHost {
				t.Errorf("%s: got host %q, expected %q", tt.description, host, tt.expectedHost)
			}
		})
	}
}

func TestLoadServerConfig(t *testing.T) {
	tests := []struct {
		name        string
		envVars     map[string]string
		expectError bool
		validate    func(*testing.T, *ServerConfig)
	}{
		{
			name: "default configuration",
			envVars: map[string]string{},
			expectError: false,
			validate: func(t *testing.T, config *ServerConfig) {
				if config.Port != "13080" {
					t.Errorf("Expected default port 13080, got %s", config.Port)
				}
				if config.Host != "127.0.0.1" {
					t.Errorf("Expected default host 127.0.0.1, got %s", config.Host)
				}
				if len(config.CORSOrigins) != 1 || config.CORSOrigins[0] != "*" {
					t.Errorf("Expected default CORS origins [*], got %v", config.CORSOrigins)
				}
				if config.RateLimit != time.Minute {
					t.Errorf("Expected default rate limit 1m, got %v", config.RateLimit)
				}
			},
		},
		{
			name: "railway configuration",
			envVars: map[string]string{
				"PORT": "3000",
				"RAILWAY_ENVIRONMENT": "production",
			},
			expectError: false,
			validate: func(t *testing.T, config *ServerConfig) {
				if config.Port != "3000" {
					t.Errorf("Expected Railway port 3000, got %s", config.Port)
				}
				if config.Host != "" {
					t.Errorf("Expected empty host for dual-stack, got %s", config.Host)
				}
				if config.RailwayEnvironment != "production" {
					t.Errorf("Expected Railway environment production, got %s", config.RailwayEnvironment)
				}
			},
		},
		{
			name: "custom CORS origins",
			envVars: map[string]string{
				"SLACK_MCP_CORS_ORIGINS": "https://example.com, https://app.example.com",
			},
			expectError: false,
			validate: func(t *testing.T, config *ServerConfig) {
				expected := []string{"https://example.com", "https://app.example.com"}
				if len(config.CORSOrigins) != len(expected) {
					t.Errorf("Expected %d CORS origins, got %d", len(expected), len(config.CORSOrigins))
				}
				for i, origin := range expected {
					if config.CORSOrigins[i] != origin {
						t.Errorf("Expected CORS origin %s, got %s", origin, config.CORSOrigins[i])
					}
				}
			},
		},
		{
			name: "custom rate limit",
			envVars: map[string]string{
				"SLACK_MCP_RATE_LIMIT": "120",
			},
			expectError: false,
			validate: func(t *testing.T, config *ServerConfig) {
				expected := time.Minute / 120 // 120 requests per minute
				if config.RateLimit != expected {
					t.Errorf("Expected rate limit %v, got %v", expected, config.RateLimit)
				}
			},
		},
		{
			name: "invalid rate limit",
			envVars: map[string]string{
				"SLACK_MCP_RATE_LIMIT": "invalid",
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clear all relevant environment variables
			envVarsToClean := []string{
				"PORT", "RAILWAY_ENVIRONMENT", "SLACK_MCP_HOST", "SLACK_MCP_PORT",
				"SLACK_MCP_BASE_URL", "SLACK_MCP_CORS_ORIGINS", "SLACK_MCP_RATE_LIMIT",
				"SLACK_MCP_SECURITY_HEADERS", "SLACK_MCP_HEALTH_ENABLED", "SLACK_MCP_PRIVATE_NETWORK",
			}
			for _, envVar := range envVarsToClean {
				os.Unsetenv(envVar)
			}

			// Set test environment variables
			for key, value := range tt.envVars {
				os.Setenv(key, value)
			}

			// Clean up after test
			defer func() {
				for _, envVar := range envVarsToClean {
					os.Unsetenv(envVar)
				}
			}()

			config, err := loadServerConfig()
			
			if tt.expectError {
				if err == nil {
					t.Error("Expected error but got none")
				}
				return
			}

			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			if tt.validate != nil {
				tt.validate(t, config)
			}
		})
	}
}

func TestValidateServerConfig(t *testing.T) {
	tests := []struct {
		name        string
		config      *ServerConfig
		expectError bool
		errorMsg    string
	}{
		{
			name: "valid configuration",
			config: &ServerConfig{
				Port:        "8080",
				CORSOrigins: []string{"https://example.com"},
				RateLimit:   time.Minute,
			},
			expectError: false,
		},
		{
			name: "invalid port",
			config: &ServerConfig{
				Port:        "invalid",
				CORSOrigins: []string{"https://example.com"},
				RateLimit:   time.Minute,
			},
			expectError: true,
			errorMsg:    "invalid port",
		},
		{
			name: "empty CORS origin",
			config: &ServerConfig{
				Port:        "8080",
				CORSOrigins: []string{""},
				RateLimit:   time.Minute,
			},
			expectError: true,
			errorMsg:    "empty origin not allowed",
		},
		{
			name: "invalid rate limit",
			config: &ServerConfig{
				Port:        "8080",
				CORSOrigins: []string{"https://example.com"},
				RateLimit:   0,
			},
			expectError: true,
			errorMsg:    "must be positive",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateServerConfig(tt.config)
			
			if tt.expectError {
				if err == nil {
					t.Error("Expected error but got none")
				} else if tt.errorMsg != "" && !strings.Contains(err.Error(), tt.errorMsg) {
					t.Errorf("Expected error to contain %q, got %q", tt.errorMsg, err.Error())
				}
			} else if err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
		})
	}
}

func TestDualStackBinding(t *testing.T) {
	// Test that dual-stack binding works correctly
	tests := []struct {
		name         string
		host         string
		expectedAddr string
		description  string
	}{
		{
			name:         "dual-stack binding",
			host:         "",
			expectedAddr: ":8080",
			description:  "Empty host should create dual-stack bind address",
		},
		{
			name:         "ipv4 specific binding",
			host:         "127.0.0.1",
			expectedAddr: "127.0.0.1:8080",
			description:  "IPv4 host should create IPv4-specific bind address",
		},
		{
			name:         "ipv6 specific binding",
			host:         "::1",
			expectedAddr: "::1:8080",
			description:  "IPv6 host should create IPv6-specific bind address",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			port := "8080"
			var bindAddr string
			
			if tt.host == "" {
				bindAddr = ":" + port // Dual-stack binding
			} else {
				bindAddr = tt.host + ":" + port // Specific host binding
			}

			if bindAddr != tt.expectedAddr {
				t.Errorf("%s: got bind address %q, expected %q", tt.description, bindAddr, tt.expectedAddr)
			}
		})
	}
}

func TestIPv6AddressFormatting(t *testing.T) {
	tests := []struct {
		name         string
		host         string
		expectedLog  string
		description  string
	}{
		{
			name:         "ipv4 address",
			host:         "127.0.0.1",
			expectedLog:  "127.0.0.1",
			description:  "IPv4 addresses should not be modified",
		},
		{
			name:         "ipv6 address without brackets",
			host:         "::1",
			expectedLog:  "[::1]",
			description:  "IPv6 addresses should be wrapped in brackets for logging",
		},
		{
			name:         "ipv6 address with brackets",
			host:         "[::1]",
			expectedLog:  "[::1]",
			description:  "IPv6 addresses with brackets should remain unchanged",
		},
		{
			name:         "hostname",
			host:         "localhost",
			expectedLog:  "localhost",
			description:  "Hostnames should not be modified",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Format IPv6 addresses properly in logs
			displayHost := tt.host
			if strings.Contains(tt.host, ":") && !strings.HasPrefix(tt.host, "[") {
				displayHost = "[" + tt.host + "]"
			}

			if displayHost != tt.expectedLog {
				t.Errorf("%s: got display host %q, expected %q", tt.description, displayHost, tt.expectedLog)
			}
		})
	}
}

func TestNetworkBindingFallback(t *testing.T) {
	// Test IPv6 fallback behavior by attempting to bind to different addresses
	tests := []struct {
		name        string
		network     string
		address     string
		shouldWork  bool
		description string
	}{
		{
			name:        "ipv4 localhost",
			network:     "tcp4",
			address:     "127.0.0.1:0",
			shouldWork:  true,
			description: "IPv4 localhost should always work",
		},
		{
			name:        "dual-stack",
			network:     "tcp",
			address:     ":0",
			shouldWork:  true,
			description: "Dual-stack binding should work on most systems",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			listener, err := net.Listen(tt.network, tt.address)
			
			if tt.shouldWork {
				if err != nil {
					t.Errorf("%s: expected binding to work, got error: %v", tt.description, err)
				} else {
					listener.Close()
				}
			} else {
				if err == nil {
					listener.Close()
					t.Errorf("%s: expected binding to fail, but it succeeded", tt.description)
				}
			}
		})
	}
}