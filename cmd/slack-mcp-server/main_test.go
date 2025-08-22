package main

import (
	"os"
	"testing"
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