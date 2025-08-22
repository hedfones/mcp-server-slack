package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/korotovsky/slack-mcp-server/pkg/provider"
	"go.uber.org/zap"
)

func TestHealthChecker_HealthHandler(t *testing.T) {
	logger := zap.NewNop()
	
	// Create a mock provider for testing
	provider := &provider.ApiProvider{}
	
	healthChecker := NewHealthChecker(provider, logger)
	
	req := httptest.NewRequest("GET", "/health", nil)
	w := httptest.NewRecorder()
	
	healthChecker.HealthHandler(w, req)
	
	resp := w.Result()
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusServiceUnavailable {
		t.Errorf("Expected status 200 or 503, got %d", resp.StatusCode)
	}
	
	var healthResp HealthResponse
	if err := json.NewDecoder(resp.Body).Decode(&healthResp); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}
	
	if healthResp.Status != HealthStatusHealthy && healthResp.Status != HealthStatusUnhealthy {
		t.Errorf("Expected status to be healthy or unhealthy, got %s", healthResp.Status)
	}
	
	if healthResp.Version == "" {
		t.Error("Expected version to be set")
	}
	
	if healthResp.Checks == nil {
		t.Error("Expected checks to be set")
	}
}

func TestHealthChecker_LivenessHandler(t *testing.T) {
	logger := zap.NewNop()
	provider := &provider.ApiProvider{}
	
	healthChecker := NewHealthChecker(provider, logger)
	
	req := httptest.NewRequest("GET", "/health/live", nil)
	w := httptest.NewRecorder()
	
	healthChecker.LivenessHandler(w, req)
	
	resp := w.Result()
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}
	
	var healthResp HealthResponse
	if err := json.NewDecoder(resp.Body).Decode(&healthResp); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}
	
	if healthResp.Status != HealthStatusHealthy {
		t.Errorf("Expected status to be healthy, got %s", healthResp.Status)
	}
	
	if healthResp.Uptime == nil {
		t.Error("Expected uptime to be set")
	}
	
	if *healthResp.Uptime <= 0 {
		t.Error("Expected uptime to be positive")
	}
}

func TestHealthChecker_ReadinessHandler(t *testing.T) {
	logger := zap.NewNop()
	provider := &provider.ApiProvider{}
	
	healthChecker := NewHealthChecker(provider, logger)
	
	req := httptest.NewRequest("GET", "/health/ready", nil)
	w := httptest.NewRecorder()
	
	healthChecker.ReadinessHandler(w, req)
	
	resp := w.Result()
	defer resp.Body.Close()
	
	// Readiness check may fail due to Slack API connectivity
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusServiceUnavailable {
		t.Errorf("Expected status 200 or 503, got %d", resp.StatusCode)
	}
	
	var healthResp HealthResponse
	if err := json.NewDecoder(resp.Body).Decode(&healthResp); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}
	
	if healthResp.Checks == nil {
		t.Error("Expected checks to be set")
	}
	
	// Should include both cache and slack_api checks
	if _, exists := healthResp.Checks["cache"]; !exists {
		t.Error("Expected cache check to be present")
	}
	
	if _, exists := healthResp.Checks["slack_api"]; !exists {
		t.Error("Expected slack_api check to be present")
	}
}

func TestIsHealthCheckEnabled(t *testing.T) {
	// Test default behavior (should be enabled)
	if !IsHealthCheckEnabled() {
		t.Error("Expected health checks to be enabled by default")
	}
	
	// Test explicit enable
	os.Setenv("SLACK_MCP_HEALTH_ENABLED", "true")
	defer os.Unsetenv("SLACK_MCP_HEALTH_ENABLED")
	
	if !IsHealthCheckEnabled() {
		t.Error("Expected health checks to be enabled when set to true")
	}
	
	// Test explicit disable
	os.Setenv("SLACK_MCP_HEALTH_ENABLED", "false")
	
	if IsHealthCheckEnabled() {
		t.Error("Expected health checks to be disabled when set to false")
	}
}

func TestHealthResponse_JSONSerialization(t *testing.T) {
	uptime := 5 * time.Minute
	response := &HealthResponse{
		Status:    HealthStatusHealthy,
		Timestamp: time.Now(),
		Version:   "1.0.0",
		Checks: map[string]CheckStatus{
			"cache":     CheckStatusOK,
			"slack_api": CheckStatusOK,
		},
		Uptime: &uptime,
		Details: map[string]string{
			"info": "All systems operational",
		},
	}
	
	data, err := json.Marshal(response)
	if err != nil {
		t.Fatalf("Failed to marshal response: %v", err)
	}
	
	var decoded HealthResponse
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}
	
	if decoded.Status != response.Status {
		t.Errorf("Expected status %s, got %s", response.Status, decoded.Status)
	}
	
	if decoded.Version != response.Version {
		t.Errorf("Expected version %s, got %s", response.Version, decoded.Version)
	}
	
	if len(decoded.Checks) != len(response.Checks) {
		t.Errorf("Expected %d checks, got %d", len(response.Checks), len(decoded.Checks))
	}
}

func TestHealthChecker_PerformHealthChecks(t *testing.T) {
	logger := zap.NewNop()
	
	tests := []struct {
		name             string
		provider         *provider.ApiProvider
		includeReadiness bool
		expectedStatus   HealthStatus
		expectedChecks   []string
	}{
		{
			name:             "basic health check without readiness",
			provider:         &provider.ApiProvider{},
			includeReadiness: false,
			expectedStatus:   HealthStatusUnhealthy, // Cache not ready
			expectedChecks:   []string{"cache"},
		},
		{
			name:             "readiness check includes slack api",
			provider:         &provider.ApiProvider{},
			includeReadiness: true,
			expectedStatus:   HealthStatusUnhealthy,
			expectedChecks:   []string{"cache", "slack_api"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			healthChecker := NewHealthChecker(tt.provider, logger)
			ctx := context.Background()
			
			response := healthChecker.performHealthChecks(ctx, tt.includeReadiness)
			
			if response.Status != tt.expectedStatus {
				t.Errorf("Expected status %s, got %s", tt.expectedStatus, response.Status)
			}
			
			for _, check := range tt.expectedChecks {
				if _, exists := response.Checks[check]; !exists {
					t.Errorf("Expected check %s to be present", check)
				}
			}
			
			if response.Version == "" {
				t.Error("Expected version to be set")
			}
			
			if response.Uptime == nil || *response.Uptime <= 0 {
				t.Error("Expected uptime to be positive")
			}
		})
	}
}

func TestHealthChecker_CheckCacheSystem(t *testing.T) {
	logger := zap.NewNop()
	
	tests := []struct {
		name           string
		provider       *provider.ApiProvider
		expectedStatus CheckStatus
	}{
		{
			name:           "nil provider",
			provider:       nil,
			expectedStatus: CheckStatusError,
		},
		{
			name:           "provider not ready",
			provider:       &provider.ApiProvider{},
			expectedStatus: CheckStatusError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			healthChecker := NewHealthChecker(tt.provider, logger)
			
			status := healthChecker.checkCacheSystem()
			
			if status != tt.expectedStatus {
				t.Errorf("Expected cache status %s, got %s", tt.expectedStatus, status)
			}
		})
	}
}

func TestHealthChecker_CheckSlackAPI(t *testing.T) {
	logger := zap.NewNop()
	
	tests := []struct {
		name           string
		provider       *provider.ApiProvider
		envVars        map[string]string
		expectedStatus CheckStatus
	}{
		{
			name:           "nil provider",
			provider:       nil,
			expectedStatus: CheckStatusError,
		},
		{
			name:     "demo mode xoxp",
			provider: &provider.ApiProvider{},
			envVars: map[string]string{
				"SLACK_MCP_XOXP_TOKEN": "demo",
			},
			expectedStatus: CheckStatusError, // Provider.Slack() returns nil
		},
		{
			name:     "demo mode xoxc/xoxd",
			provider: &provider.ApiProvider{},
			envVars: map[string]string{
				"SLACK_MCP_XOXC_TOKEN": "demo",
				"SLACK_MCP_XOXD_TOKEN": "demo",
			},
			expectedStatus: CheckStatusError, // Provider.Slack() returns nil
		},
		{
			name:     "provider without slack client",
			provider: &provider.ApiProvider{},
			envVars: map[string]string{
				"SLACK_MCP_XOXP_TOKEN": "real-token",
			},
			expectedStatus: CheckStatusError, // No Slack client initialized
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clear environment variables
			os.Unsetenv("SLACK_MCP_XOXP_TOKEN")
			os.Unsetenv("SLACK_MCP_XOXC_TOKEN")
			os.Unsetenv("SLACK_MCP_XOXD_TOKEN")

			// Set test environment variables
			for key, value := range tt.envVars {
				os.Setenv(key, value)
			}

			// Clean up after test
			defer func() {
				os.Unsetenv("SLACK_MCP_XOXP_TOKEN")
				os.Unsetenv("SLACK_MCP_XOXC_TOKEN")
				os.Unsetenv("SLACK_MCP_XOXD_TOKEN")
			}()

			healthChecker := NewHealthChecker(tt.provider, logger)
			ctx := context.Background()
			
			status := healthChecker.checkSlackAPI(ctx)
			
			if status != tt.expectedStatus {
				t.Errorf("Expected Slack API status %s, got %s", tt.expectedStatus, status)
			}
		})
	}
}

func TestHealthChecker_WriteHealthResponse(t *testing.T) {
	logger := zap.NewNop()
	healthChecker := NewHealthChecker(&provider.ApiProvider{}, logger)
	
	tests := []struct {
		name           string
		response       *HealthResponse
		expectedStatus int
		expectedHeader string
	}{
		{
			name: "healthy response",
			response: &HealthResponse{
				Status:    HealthStatusHealthy,
				Timestamp: time.Now(),
				Version:   "1.0.0",
				Checks:    map[string]CheckStatus{"cache": CheckStatusOK},
			},
			expectedStatus: http.StatusOK,
			expectedHeader: "application/json",
		},
		{
			name: "unhealthy response",
			response: &HealthResponse{
				Status:    HealthStatusUnhealthy,
				Timestamp: time.Now(),
				Version:   "1.0.0",
				Checks:    map[string]CheckStatus{"cache": CheckStatusError},
			},
			expectedStatus: http.StatusServiceUnavailable,
			expectedHeader: "application/json",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			
			healthChecker.writeHealthResponse(w, tt.response)
			
			resp := w.Result()
			defer resp.Body.Close()
			
			if resp.StatusCode != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d", tt.expectedStatus, resp.StatusCode)
			}
			
			contentType := resp.Header.Get("Content-Type")
			if contentType != tt.expectedHeader {
				t.Errorf("Expected Content-Type %s, got %s", tt.expectedHeader, contentType)
			}
			
			var decoded HealthResponse
			if err := json.NewDecoder(resp.Body).Decode(&decoded); err != nil {
				t.Fatalf("Failed to decode response: %v", err)
			}
			
			if decoded.Status != tt.response.Status {
				t.Errorf("Expected status %s, got %s", tt.response.Status, decoded.Status)
			}
		})
	}
}

func TestHealthChecker_EndpointIntegration(t *testing.T) {
	logger := zap.NewNop()
	provider := &provider.ApiProvider{}
	healthChecker := NewHealthChecker(provider, logger)
	
	tests := []struct {
		name        string
		endpoint    string
		handler     http.HandlerFunc
		expectOK    bool
		expectJSON  bool
		description string
	}{
		{
			name:        "health endpoint",
			endpoint:    "/health",
			handler:     healthChecker.HealthHandler,
			expectOK:    false, // Provider not ready
			expectJSON:  true,
			description: "Health endpoint should return JSON response",
		},
		{
			name:        "readiness endpoint",
			endpoint:    "/health/ready",
			handler:     healthChecker.ReadinessHandler,
			expectOK:    false, // Provider not ready
			expectJSON:  true,
			description: "Readiness endpoint should return JSON response",
		},
		{
			name:        "liveness endpoint",
			endpoint:    "/health/live",
			handler:     healthChecker.LivenessHandler,
			expectOK:    true, // Liveness should always be OK
			expectJSON:  true,
			description: "Liveness endpoint should always return OK",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", tt.endpoint, nil)
			w := httptest.NewRecorder()
			
			tt.handler(w, req)
			
			resp := w.Result()
			defer resp.Body.Close()
			
			if tt.expectOK {
				if resp.StatusCode != http.StatusOK {
					t.Errorf("%s: expected status 200, got %d", tt.description, resp.StatusCode)
				}
			} else {
				if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusServiceUnavailable {
					t.Errorf("%s: expected status 200 or 503, got %d", tt.description, resp.StatusCode)
				}
			}
			
			if tt.expectJSON {
				contentType := resp.Header.Get("Content-Type")
				if !strings.Contains(contentType, "application/json") {
					t.Errorf("%s: expected JSON content type, got %s", tt.description, contentType)
				}
				
				var healthResp HealthResponse
				if err := json.NewDecoder(resp.Body).Decode(&healthResp); err != nil {
					t.Errorf("%s: failed to decode JSON response: %v", tt.description, err)
				}
			}
		})
	}
}

func TestHealthChecker_ContextTimeout(t *testing.T) {
	logger := zap.NewNop()
	provider := &provider.ApiProvider{}
	healthChecker := NewHealthChecker(provider, logger)
	
	// Test with a very short timeout
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Nanosecond)
	defer cancel()
	
	// Wait for context to timeout
	time.Sleep(1 * time.Millisecond)
	
	response := healthChecker.performHealthChecks(ctx, true)
	
	// Should still return a response even with timeout
	if response == nil {
		t.Error("Expected response even with timeout")
	}
	
	if response.Status != HealthStatusUnhealthy {
		t.Errorf("Expected unhealthy status with timeout, got %s", response.Status)
	}
}

func TestHealthResponse_AllFields(t *testing.T) {
	uptime := 10 * time.Minute
	response := &HealthResponse{
		Status:    HealthStatusHealthy,
		Timestamp: time.Now(),
		Version:   "1.2.3",
		Checks: map[string]CheckStatus{
			"cache":     CheckStatusOK,
			"slack_api": CheckStatusOK,
		},
		Uptime: &uptime,
		Details: map[string]string{
			"cache":     "All systems operational",
			"slack_api": "Connected successfully",
		},
	}
	
	// Test JSON marshaling and unmarshaling
	data, err := json.Marshal(response)
	if err != nil {
		t.Fatalf("Failed to marshal response: %v", err)
	}
	
	var decoded HealthResponse
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}
	
	// Verify all fields are preserved
	if decoded.Status != response.Status {
		t.Errorf("Status mismatch: expected %s, got %s", response.Status, decoded.Status)
	}
	
	if decoded.Version != response.Version {
		t.Errorf("Version mismatch: expected %s, got %s", response.Version, decoded.Version)
	}
	
	if len(decoded.Checks) != len(response.Checks) {
		t.Errorf("Checks count mismatch: expected %d, got %d", len(response.Checks), len(decoded.Checks))
	}
	
	if decoded.Uptime == nil || *decoded.Uptime != *response.Uptime {
		t.Error("Uptime mismatch")
	}
	
	if len(decoded.Details) != len(response.Details) {
		t.Errorf("Details count mismatch: expected %d, got %d", len(response.Details), len(decoded.Details))
	}
}