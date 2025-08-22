package middleware

import (
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"go.uber.org/zap"
	"golang.org/x/time/rate"
)

func TestSecurityMiddleware_RateLimit(t *testing.T) {
	// Set up test environment
	os.Setenv("SLACK_MCP_RATE_LIMIT", "60") // 60 requests per minute (1 per second)
	defer os.Unsetenv("SLACK_MCP_RATE_LIMIT")

	logger := zap.NewNop()
	middleware := NewSecurityMiddleware(logger)

	// Create a test handler
	handler := middleware.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	}))

	// Test that first request passes
	req1 := httptest.NewRequest("GET", "/test", nil)
	req1.RemoteAddr = "192.168.1.1:12345"
	w1 := httptest.NewRecorder()
	handler.ServeHTTP(w1, req1)

	if w1.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w1.Code)
	}

	// Test that second request from same IP is rate limited (burst = 1)
	req2 := httptest.NewRequest("GET", "/test", nil)
	req2.RemoteAddr = "192.168.1.1:12346" // Same IP, different port
	w2 := httptest.NewRecorder()
	handler.ServeHTTP(w2, req2)

	if w2.Code != http.StatusTooManyRequests {
		t.Errorf("Expected status 429, got %d", w2.Code)
	}

	// Test that request from different IP passes
	req3 := httptest.NewRequest("GET", "/test", nil)
	req3.RemoteAddr = "192.168.1.2:12345" // Different IP
	w3 := httptest.NewRecorder()
	handler.ServeHTTP(w3, req3)

	if w3.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w3.Code)
	}
}

func TestSecurityMiddleware_CORS(t *testing.T) {
	// Set up test environment with specific CORS origins
	os.Setenv("SLACK_MCP_CORS_ORIGINS", "https://example.com,https://test.com")
	defer os.Unsetenv("SLACK_MCP_CORS_ORIGINS")

	logger := zap.NewNop()
	middleware := NewSecurityMiddleware(logger)

	handler := middleware.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// Test allowed origin
	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Origin", "https://example.com")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Header().Get("Access-Control-Allow-Origin") != "https://example.com" {
		t.Errorf("Expected CORS origin to be set to https://example.com, got %s", 
			w.Header().Get("Access-Control-Allow-Origin"))
	}
}

func TestSecurityMiddleware_SecurityHeaders(t *testing.T) {
	logger := zap.NewNop()
	middleware := NewSecurityMiddleware(logger)

	handler := middleware.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	// Check security headers
	expectedHeaders := map[string]string{
		"X-Content-Type-Options": "nosniff",
		"X-Frame-Options":        "DENY",
		"X-XSS-Protection":       "1; mode=block",
		"Referrer-Policy":        "strict-origin-when-cross-origin",
	}

	for header, expectedValue := range expectedHeaders {
		if w.Header().Get(header) != expectedValue {
			t.Errorf("Expected %s header to be %s, got %s", 
				header, expectedValue, w.Header().Get(header))
		}
	}
}

func TestSecurityMiddleware_PreflightRequest(t *testing.T) {
	logger := zap.NewNop()
	middleware := NewSecurityMiddleware(logger)

	handler := middleware.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("Handler should not be called for OPTIONS request")
	}))

	req := httptest.NewRequest("OPTIONS", "/test", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200 for OPTIONS request, got %d", w.Code)
	}
}

func TestGetClientIP(t *testing.T) {
	tests := []struct {
		name           string
		remoteAddr     string
		xForwardedFor  string
		xRealIP        string
		expectedIP     string
	}{
		{
			name:       "RemoteAddr only",
			remoteAddr: "192.168.1.1:12345",
			expectedIP: "192.168.1.1",
		},
		{
			name:          "X-Forwarded-For header",
			remoteAddr:    "192.168.1.1:12345",
			xForwardedFor: "203.0.113.1, 192.168.1.1",
			expectedIP:    "203.0.113.1",
		},
		{
			name:       "X-Real-IP header",
			remoteAddr: "192.168.1.1:12345",
			xRealIP:    "203.0.113.2",
			expectedIP: "203.0.113.2",
		},
		{
			name:       "IPv6 address",
			remoteAddr: "[2001:db8::1]:12345",
			expectedIP: "2001:db8::1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/test", nil)
			req.RemoteAddr = tt.remoteAddr
			
			if tt.xForwardedFor != "" {
				req.Header.Set("X-Forwarded-For", tt.xForwardedFor)
			}
			if tt.xRealIP != "" {
				req.Header.Set("X-Real-IP", tt.xRealIP)
			}

			ip := getClientIP(req)
			if ip != tt.expectedIP {
				t.Errorf("Expected IP %s, got %s", tt.expectedIP, ip)
			}
		})
	}
}

func TestSecurityMiddleware_RateLimitDisabled(t *testing.T) {
	// Test with rate limiting disabled by manually setting RateLimit to 0
	logger := zap.NewNop()
	middleware := &SecurityMiddleware{
		config: SecurityConfig{
			CORSOrigins:          []string{},
			EnableSecurityHeaders: true,
			RateLimit:            0, // Disabled
			Logger:               logger,
		},
		rateLimiters: make(map[string]*rate.Limiter),
	}

	handler := middleware.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	}))

	// Make multiple requests from same IP - should all pass when rate limiting is disabled
	for i := 0; i < 5; i++ {
		req := httptest.NewRequest("GET", "/test", nil)
		req.RemoteAddr = "192.168.1.1:12345"
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Request %d: Expected status 200, got %d", i+1, w.Code)
		}
	}
}

func TestSecurityMiddleware_RateLimitDifferentIPs(t *testing.T) {
	os.Setenv("SLACK_MCP_RATE_LIMIT", "60") // 60 requests per minute
	defer os.Unsetenv("SLACK_MCP_RATE_LIMIT")

	logger := zap.NewNop()
	middleware := NewSecurityMiddleware(logger)

	handler := middleware.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// Test that different IPs have separate rate limiters
	ips := []string{"192.168.1.1:12345", "192.168.1.2:12345", "192.168.1.3:12345"}
	
	for _, ip := range ips {
		req := httptest.NewRequest("GET", "/test", nil)
		req.RemoteAddr = ip
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Request from IP %s: Expected status 200, got %d", ip, w.Code)
		}
	}
}

func TestSecurityMiddleware_RateLimitErrorResponse(t *testing.T) {
	os.Setenv("SLACK_MCP_RATE_LIMIT", "60") // 60 requests per minute
	defer os.Unsetenv("SLACK_MCP_RATE_LIMIT")

	logger := zap.NewNop()
	middleware := NewSecurityMiddleware(logger)

	handler := middleware.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// First request should pass
	req1 := httptest.NewRequest("GET", "/test", nil)
	req1.RemoteAddr = "192.168.1.1:12345"
	w1 := httptest.NewRecorder()
	handler.ServeHTTP(w1, req1)

	if w1.Code != http.StatusOK {
		t.Errorf("First request: Expected status 200, got %d", w1.Code)
	}

	// Second request should be rate limited
	req2 := httptest.NewRequest("GET", "/test", nil)
	req2.RemoteAddr = "192.168.1.1:12346" // Same IP, different port
	w2 := httptest.NewRecorder()
	handler.ServeHTTP(w2, req2)

	if w2.Code != http.StatusTooManyRequests {
		t.Errorf("Second request: Expected status 429, got %d", w2.Code)
	}

	// Check error response format
	contentType := w2.Header().Get("Content-Type")
	if contentType != "application/json" {
		t.Errorf("Expected Content-Type application/json, got %s", contentType)
	}

	body := w2.Body.String()
	if !strings.Contains(body, "RATE_LIMIT_EXCEEDED") {
		t.Error("Expected error response to contain RATE_LIMIT_EXCEEDED")
	}
	if !strings.Contains(body, "Too many requests") {
		t.Error("Expected error response to contain rate limit message")
	}
}

func TestSecurityMiddleware_CORSAllowAll(t *testing.T) {
	// Test with no CORS origins configured (should allow all)
	os.Unsetenv("SLACK_MCP_CORS_ORIGINS")

	logger := zap.NewNop()
	middleware := NewSecurityMiddleware(logger)

	handler := middleware.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Origin", "https://random-origin.com")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Header().Get("Access-Control-Allow-Origin") != "*" {
		t.Errorf("Expected CORS origin to be *, got %s", 
			w.Header().Get("Access-Control-Allow-Origin"))
	}
}

func TestSecurityMiddleware_CORSWildcard(t *testing.T) {
	// Test with wildcard in CORS origins
	os.Setenv("SLACK_MCP_CORS_ORIGINS", "*")
	defer os.Unsetenv("SLACK_MCP_CORS_ORIGINS")

	logger := zap.NewNop()
	middleware := NewSecurityMiddleware(logger)

	handler := middleware.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Origin", "https://any-origin.com")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Header().Get("Access-Control-Allow-Origin") != "https://any-origin.com" {
		t.Errorf("Expected CORS origin to be https://any-origin.com, got %s", 
			w.Header().Get("Access-Control-Allow-Origin"))
	}
}

func TestSecurityMiddleware_CORSBlocked(t *testing.T) {
	// Test with specific CORS origins that don't match request
	os.Setenv("SLACK_MCP_CORS_ORIGINS", "https://allowed.com,https://also-allowed.com")
	defer os.Unsetenv("SLACK_MCP_CORS_ORIGINS")

	logger := zap.NewNop()
	middleware := NewSecurityMiddleware(logger)

	handler := middleware.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Origin", "https://blocked.com")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	// Should not set Access-Control-Allow-Origin for blocked origins
	if w.Header().Get("Access-Control-Allow-Origin") == "https://blocked.com" {
		t.Error("CORS origin should not be set for blocked origin")
	}
}

func TestSecurityMiddleware_CORSHeaders(t *testing.T) {
	logger := zap.NewNop()
	middleware := NewSecurityMiddleware(logger)

	handler := middleware.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	// Check all CORS headers are set
	expectedHeaders := map[string]string{
		"Access-Control-Allow-Methods":     "GET, POST, PUT, DELETE, OPTIONS",
		"Access-Control-Allow-Headers":     "Content-Type, Authorization, X-Requested-With",
		"Access-Control-Allow-Credentials": "true",
		"Access-Control-Max-Age":           "86400",
	}

	for header, expectedValue := range expectedHeaders {
		if w.Header().Get(header) != expectedValue {
			t.Errorf("Expected %s header to be %s, got %s", 
				header, expectedValue, w.Header().Get(header))
		}
	}
}

func TestSecurityMiddleware_SecurityHeadersDisabled(t *testing.T) {
	// Test with security headers disabled
	os.Setenv("SLACK_MCP_SECURITY_HEADERS", "false")
	defer os.Unsetenv("SLACK_MCP_SECURITY_HEADERS")

	logger := zap.NewNop()
	middleware := NewSecurityMiddleware(logger)

	handler := middleware.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	// Security headers should not be set
	securityHeaders := []string{
		"X-Content-Type-Options",
		"X-Frame-Options", 
		"X-XSS-Protection",
		"Referrer-Policy",
		"Content-Security-Policy",
	}

	for _, header := range securityHeaders {
		if w.Header().Get(header) != "" {
			t.Errorf("Security header %s should not be set when disabled, got %s", 
				header, w.Header().Get(header))
		}
	}
}

func TestSecurityMiddleware_ContentSecurityPolicy(t *testing.T) {
	logger := zap.NewNop()
	middleware := NewSecurityMiddleware(logger)

	handler := middleware.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	csp := w.Header().Get("Content-Security-Policy")
	expectedCSP := "default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'"
	
	if csp != expectedCSP {
		t.Errorf("Expected CSP %s, got %s", expectedCSP, csp)
	}
}

func TestFormatIPAddress(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "empty IP",
			input:    "",
			expected: "unknown",
		},
		{
			name:     "IPv4 address",
			input:    "192.168.1.1",
			expected: "192.168.1.1",
		},
		{
			name:     "IPv6 address without brackets",
			input:    "2001:db8::1",
			expected: "[2001:db8::1]",
		},
		{
			name:     "IPv6 address with brackets",
			input:    "[2001:db8::1]",
			expected: "[2001:db8::1]",
		},
		{
			name:     "IPv6 localhost",
			input:    "::1",
			expected: "[::1]",
		},
		{
			name:     "invalid IP",
			input:    "not-an-ip",
			expected: "not-an-ip",
		},
		{
			name:     "hostname",
			input:    "localhost",
			expected: "localhost",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatIPAddress(tt.input)
			if result != tt.expected {
				t.Errorf("Expected %s, got %s", tt.expected, result)
			}
		})
	}
}

func TestGetClientIP_XForwardedForMultiple(t *testing.T) {
	req := httptest.NewRequest("GET", "/test", nil)
	req.RemoteAddr = "192.168.1.1:12345"
	req.Header.Set("X-Forwarded-For", "203.0.113.1, 203.0.113.2, 192.168.1.1")

	ip := getClientIP(req)
	expected := "203.0.113.1" // Should take the first IP

	if ip != expected {
		t.Errorf("Expected IP %s, got %s", expected, ip)
	}
}

func TestGetClientIP_XForwardedForWithSpaces(t *testing.T) {
	req := httptest.NewRequest("GET", "/test", nil)
	req.RemoteAddr = "192.168.1.1:12345"
	req.Header.Set("X-Forwarded-For", "  203.0.113.1  , 192.168.1.1")

	ip := getClientIP(req)
	expected := "203.0.113.1" // Should trim spaces

	if ip != expected {
		t.Errorf("Expected IP %s, got %s", expected, ip)
	}
}

func TestGetClientIP_Precedence(t *testing.T) {
	// Test that X-Forwarded-For takes precedence over X-Real-IP
	req := httptest.NewRequest("GET", "/test", nil)
	req.RemoteAddr = "192.168.1.1:12345"
	req.Header.Set("X-Forwarded-For", "203.0.113.1")
	req.Header.Set("X-Real-IP", "203.0.113.2")

	ip := getClientIP(req)
	expected := "203.0.113.1" // X-Forwarded-For should take precedence

	if ip != expected {
		t.Errorf("Expected IP %s, got %s", expected, ip)
	}
}

func TestParseCORSOrigins(t *testing.T) {
	tests := []struct {
		name     string
		envValue string
		expected []string
	}{
		{
			name:     "empty environment variable",
			envValue: "",
			expected: []string{},
		},
		{
			name:     "single origin",
			envValue: "https://example.com",
			expected: []string{"https://example.com"},
		},
		{
			name:     "multiple origins",
			envValue: "https://example.com,https://test.com",
			expected: []string{"https://example.com", "https://test.com"},
		},
		{
			name:     "origins with spaces",
			envValue: " https://example.com , https://test.com ",
			expected: []string{"https://example.com", "https://test.com"},
		},
		{
			name:     "origins with empty values",
			envValue: "https://example.com,,https://test.com",
			expected: []string{"https://example.com", "https://test.com"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			os.Setenv("SLACK_MCP_CORS_ORIGINS", tt.envValue)
			defer os.Unsetenv("SLACK_MCP_CORS_ORIGINS")

			result := parseCORSOrigins()
			
			if len(result) != len(tt.expected) {
				t.Errorf("Expected %d origins, got %d", len(tt.expected), len(result))
				return
			}

			for i, expected := range tt.expected {
				if result[i] != expected {
					t.Errorf("Expected origin %s, got %s", expected, result[i])
				}
			}
		})
	}
}

func TestParseSecurityHeaders(t *testing.T) {
	tests := []struct {
		name     string
		envValue string
		expected bool
	}{
		{
			name:     "empty (default enabled)",
			envValue: "",
			expected: true,
		},
		{
			name:     "explicitly enabled",
			envValue: "true",
			expected: true,
		},
		{
			name:     "explicitly disabled",
			envValue: "false",
			expected: false,
		},
		{
			name:     "numeric true",
			envValue: "1",
			expected: true,
		},
		{
			name:     "numeric false",
			envValue: "0",
			expected: false,
		},
		{
			name:     "invalid value (default enabled)",
			envValue: "invalid",
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			os.Setenv("SLACK_MCP_SECURITY_HEADERS", tt.envValue)
			defer os.Unsetenv("SLACK_MCP_SECURITY_HEADERS")

			result := parseSecurityHeaders()
			if result != tt.expected {
				t.Errorf("Expected %t, got %t", tt.expected, result)
			}
		})
	}
}

func TestParseRateLimit(t *testing.T) {
	tests := []struct {
		name     string
		envValue string
		expected time.Duration
	}{
		{
			name:     "empty (default)",
			envValue: "",
			expected: time.Minute,
		},
		{
			name:     "60 requests per minute",
			envValue: "60",
			expected: time.Second, // 60 requests per minute = 1 per second
		},
		{
			name:     "120 requests per minute",
			envValue: "120",
			expected: 500 * time.Millisecond, // 120 requests per minute = 1 per 500ms
		},
		{
			name:     "invalid value (default)",
			envValue: "invalid",
			expected: time.Minute,
		},
		{
			name:     "zero value (default)",
			envValue: "0",
			expected: time.Minute,
		},
		{
			name:     "negative value (default)",
			envValue: "-10",
			expected: time.Minute,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			os.Setenv("SLACK_MCP_RATE_LIMIT", tt.envValue)
			defer os.Unsetenv("SLACK_MCP_RATE_LIMIT")

			result := parseRateLimit()
			if result != tt.expected {
				t.Errorf("Expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestSecurityMiddleware_IntegrationTest(t *testing.T) {
	// Integration test that combines multiple middleware features
	os.Setenv("SLACK_MCP_CORS_ORIGINS", "https://allowed.com")
	os.Setenv("SLACK_MCP_RATE_LIMIT", "60")
	os.Setenv("SLACK_MCP_SECURITY_HEADERS", "true")
	defer func() {
		os.Unsetenv("SLACK_MCP_CORS_ORIGINS")
		os.Unsetenv("SLACK_MCP_RATE_LIMIT")
		os.Unsetenv("SLACK_MCP_SECURITY_HEADERS")
	}()

	logger := zap.NewNop()
	middleware := NewSecurityMiddleware(logger)

	handler := middleware.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Success"))
	}))

	// Test successful request with all features
	req := httptest.NewRequest("GET", "/api/test", nil)
	req.Header.Set("Origin", "https://allowed.com")
	req.RemoteAddr = "203.0.113.1:12345"
	w := httptest.NewRecorder()
	
	handler.ServeHTTP(w, req)

	// Verify response
	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	// Verify CORS headers
	if w.Header().Get("Access-Control-Allow-Origin") != "https://allowed.com" {
		t.Error("CORS origin not set correctly")
	}

	// Verify security headers
	if w.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Error("Security headers not set correctly")
	}

	// Verify response body
	if w.Body.String() != "Success" {
		t.Errorf("Expected response body 'Success', got %s", w.Body.String())
	}
}