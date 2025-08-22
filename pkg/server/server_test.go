package server

import (
	"os"
	"testing"

	"go.uber.org/zap"
)

func TestDetermineBaseURL(t *testing.T) {
	// Create a minimal MCPServer for testing
	logger := zap.NewNop()
	server := &MCPServer{logger: logger}

	tests := []struct {
		name        string
		addr        string
		envVars     map[string]string
		expected    string
		description string
	}{
		{
			name:        "explicit base URL",
			addr:        ":8080",
			envVars:     map[string]string{"SLACK_MCP_BASE_URL": "https://custom.example.com"},
			expected:    "https://custom.example.com",
			description: "Should use explicit SLACK_MCP_BASE_URL when set",
		},
		{
			name:        "railway public domain",
			addr:        ":8080",
			envVars:     map[string]string{"RAILWAY_PUBLIC_DOMAIN": "myapp.railway.app"},
			expected:    "https://myapp.railway.app",
			description: "Should use Railway public domain when available",
		},
		{
			name:        "dual-stack binding",
			addr:        ":8080",
			envVars:     map[string]string{},
			expected:    "http://localhost:8080",
			description: "Should handle dual-stack binding with empty host",
		},
		{
			name:        "IPv4 address",
			addr:        "127.0.0.1:8080",
			envVars:     map[string]string{},
			expected:    "http://127.0.0.1:8080",
			description: "Should handle IPv4 addresses correctly",
		},
		{
			name:        "IPv6 address",
			addr:        "[::1]:8080",
			envVars:     map[string]string{},
			expected:    "http://[::1]:8080",
			description: "Should handle IPv6 addresses with proper bracketing",
		},
		{
			name:        "hostname with port",
			addr:        "localhost:8080",
			envVars:     map[string]string{},
			expected:    "http://localhost:8080",
			description: "Should handle hostname with port correctly",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clear environment variables
			os.Unsetenv("SLACK_MCP_BASE_URL")
			os.Unsetenv("RAILWAY_PUBLIC_DOMAIN")

			// Set test environment variables
			for key, value := range tt.envVars {
				os.Setenv(key, value)
			}

			// Clean up after test
			defer func() {
				for key := range tt.envVars {
					os.Unsetenv(key)
				}
			}()

			result := server.determineBaseURL(tt.addr)
			if result != tt.expected {
				t.Errorf("%s: got %q, expected %q", tt.description, result, tt.expected)
			}
		})
	}
}