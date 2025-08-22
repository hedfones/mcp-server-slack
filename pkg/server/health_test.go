package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
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