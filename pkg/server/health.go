package server

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"time"

	"github.com/korotovsky/slack-mcp-server/pkg/provider"
	"github.com/korotovsky/slack-mcp-server/pkg/version"
	"go.uber.org/zap"
)

// HealthStatus represents the overall health status
type HealthStatus string

const (
	HealthStatusHealthy   HealthStatus = "healthy"
	HealthStatusUnhealthy HealthStatus = "unhealthy"
)

// CheckStatus represents the status of individual health checks
type CheckStatus string

const (
	CheckStatusOK    CheckStatus = "ok"
	CheckStatusError CheckStatus = "error"
)

// HealthResponse represents the JSON response for health endpoints
type HealthResponse struct {
	Status    HealthStatus           `json:"status"`
	Timestamp time.Time              `json:"timestamp"`
	Version   string                 `json:"version"`
	Checks    map[string]CheckStatus `json:"checks"`
	Uptime    *time.Duration         `json:"uptime,omitempty"`
	Details   map[string]string      `json:"details,omitempty"`
}

// HealthChecker manages health check functionality
type HealthChecker struct {
	provider  *provider.ApiProvider
	logger    *zap.Logger
	startTime time.Time
}

// NewHealthChecker creates a new health checker instance
func NewHealthChecker(provider *provider.ApiProvider, logger *zap.Logger) *HealthChecker {
	return &HealthChecker{
		provider:  provider,
		logger:    logger,
		startTime: time.Now(),
	}
}

// HealthHandler handles the basic health check endpoint
func (h *HealthChecker) HealthHandler(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	response := h.performHealthChecks(ctx, false)
	h.writeHealthResponse(w, response)
}

// ReadinessHandler handles the readiness check endpoint
func (h *HealthChecker) ReadinessHandler(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()

	response := h.performHealthChecks(ctx, true)
	h.writeHealthResponse(w, response)
}

// LivenessHandler handles the liveness check endpoint
func (h *HealthChecker) LivenessHandler(w http.ResponseWriter, r *http.Request) {
	// Liveness check is simpler - just verify the application is responsive
	uptime := time.Since(h.startTime)
	response := &HealthResponse{
		Status:    HealthStatusHealthy,
		Timestamp: time.Now(),
		Version:   version.Version,
		Checks: map[string]CheckStatus{
			"application": CheckStatusOK,
		},
		Uptime: &uptime,
	}

	h.writeHealthResponse(w, response)
}

// performHealthChecks executes all health checks and returns the aggregated result
func (h *HealthChecker) performHealthChecks(ctx context.Context, includeReadiness bool) *HealthResponse {
	checks := make(map[string]CheckStatus)
	details := make(map[string]string)
	overallStatus := HealthStatusHealthy

	// Check cache system
	cacheStatus := h.checkCacheSystem()
	checks["cache"] = cacheStatus
	if cacheStatus == CheckStatusError {
		overallStatus = HealthStatusUnhealthy
		details["cache"] = "Cache system not ready"
	}

	// Check Slack API connectivity (only for readiness checks)
	if includeReadiness {
		slackStatus := h.checkSlackAPI(ctx)
		checks["slack_api"] = slackStatus
		if slackStatus == CheckStatusError {
			overallStatus = HealthStatusUnhealthy
			details["slack_api"] = "Slack API connectivity failed"
		}
	}

	uptime := time.Since(h.startTime)
	return &HealthResponse{
		Status:    overallStatus,
		Timestamp: time.Now(),
		Version:   version.Version,
		Checks:    checks,
		Uptime:    &uptime,
		Details:   details,
	}
}

// checkCacheSystem validates the cache system status
func (h *HealthChecker) checkCacheSystem() CheckStatus {
	if h.provider == nil {
		return CheckStatusError
	}

	ready, err := h.provider.IsReady()
	if err != nil || !ready {
		h.logger.Debug("Cache system check failed",
			zap.Bool("ready", ready),
			zap.Error(err),
		)
		return CheckStatusError
	}

	return CheckStatusOK
}

// checkSlackAPI validates Slack API connectivity
func (h *HealthChecker) checkSlackAPI(ctx context.Context) CheckStatus {
	if h.provider == nil || h.provider.Slack() == nil {
		return CheckStatusError
	}

	// Skip Slack API check in demo mode
	if os.Getenv("SLACK_MCP_XOXP_TOKEN") == "demo" || 
		(os.Getenv("SLACK_MCP_XOXC_TOKEN") == "demo" && os.Getenv("SLACK_MCP_XOXD_TOKEN") == "demo") {
		return CheckStatusOK
	}

	// Perform a lightweight API call to verify connectivity
	_, err := h.provider.Slack().AuthTestContext(ctx)
	if err != nil {
		h.logger.Debug("Slack API connectivity check failed",
			zap.Error(err),
		)
		return CheckStatusError
	}

	return CheckStatusOK
}

// writeHealthResponse writes the health response as JSON
func (h *HealthChecker) writeHealthResponse(w http.ResponseWriter, response *HealthResponse) {
	w.Header().Set("Content-Type", "application/json")
	
	// Set appropriate HTTP status code
	if response.Status == HealthStatusUnhealthy {
		w.WriteHeader(http.StatusServiceUnavailable)
	} else {
		w.WriteHeader(http.StatusOK)
	}

	if err := json.NewEncoder(w).Encode(response); err != nil {
		h.logger.Error("Failed to encode health response",
			zap.Error(err),
		)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// Log health check results
	h.logger.Debug("Health check completed",
		zap.String("status", string(response.Status)),
		zap.Any("checks", response.Checks),
	)
}

// IsHealthCheckEnabled returns true if health checks are enabled via environment variable
func IsHealthCheckEnabled() bool {
	enabled := os.Getenv("SLACK_MCP_HEALTH_ENABLED")
	return enabled == "" || enabled == "true" // Default to enabled
}