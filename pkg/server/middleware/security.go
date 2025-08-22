package middleware

import (
	"fmt"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"golang.org/x/time/rate"
	"go.uber.org/zap"
)

// SecurityConfig holds configuration for security middleware
type SecurityConfig struct {
	CORSOrigins          []string
	EnableSecurityHeaders bool
	RateLimit            time.Duration
	Logger               *zap.Logger
}

// SecurityMiddleware provides CORS, security headers, and rate limiting
type SecurityMiddleware struct {
	config      SecurityConfig
	rateLimiters map[string]*rate.Limiter
	mu          sync.RWMutex
}

// NewSecurityMiddleware creates a new security middleware instance
func NewSecurityMiddleware(logger *zap.Logger) *SecurityMiddleware {
	config := SecurityConfig{
		CORSOrigins:          parseCORSOrigins(),
		EnableSecurityHeaders: parseSecurityHeaders(),
		RateLimit:            parseRateLimit(),
		Logger:               logger,
	}

	return &SecurityMiddleware{
		config:       config,
		rateLimiters: make(map[string]*rate.Limiter),
	}
}

// Handler returns an HTTP middleware function
func (sm *SecurityMiddleware) Handler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Apply rate limiting
		if !sm.checkRateLimit(r, w) {
			return
		}

		// Apply CORS headers
		sm.applyCORS(w, r)

		// Apply security headers
		if sm.config.EnableSecurityHeaders {
			sm.applySecurityHeaders(w)
		}

		// Handle preflight requests
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// checkRateLimit checks if the request should be rate limited
func (sm *SecurityMiddleware) checkRateLimit(r *http.Request, w http.ResponseWriter) bool {
	if sm.config.RateLimit == 0 {
		return true // Rate limiting disabled
	}

	clientIP := getClientIP(r)
	limiter := sm.getRateLimiter(clientIP)

	if !limiter.Allow() {
		sm.config.Logger.Warn("Rate limit exceeded",
			zap.String("client_ip", clientIP),
			zap.String("path", r.URL.Path),
		)

		sm.writeErrorResponse(w, http.StatusTooManyRequests, "RATE_LIMIT_EXCEEDED", 
			"Too many requests from this client", 
			fmt.Sprintf("Rate limit of %.0f requests per minute exceeded", 60.0/sm.config.RateLimit.Minutes()))
		return false
	}

	return true
}

// getRateLimiter gets or creates a rate limiter for the given IP
func (sm *SecurityMiddleware) getRateLimiter(ip string) *rate.Limiter {
	sm.mu.RLock()
	limiter, exists := sm.rateLimiters[ip]
	sm.mu.RUnlock()

	if !exists {
		sm.mu.Lock()
		// Double-check after acquiring write lock
		if limiter, exists = sm.rateLimiters[ip]; !exists {
			// Create new rate limiter: requests per minute converted to requests per second
			rps := 1.0 / sm.config.RateLimit.Seconds()
			limiter = rate.NewLimiter(rate.Limit(rps), 1) // Burst of 1
			sm.rateLimiters[ip] = limiter
		}
		sm.mu.Unlock()
	}

	return limiter
}

// applyCORS applies CORS headers to the response
func (sm *SecurityMiddleware) applyCORS(w http.ResponseWriter, r *http.Request) {
	origin := r.Header.Get("Origin")
	
	// If no origins configured, allow all origins for private network deployment
	if len(sm.config.CORSOrigins) == 0 {
		w.Header().Set("Access-Control-Allow-Origin", "*")
	} else {
		// Check if origin is in allowed list
		for _, allowedOrigin := range sm.config.CORSOrigins {
			if allowedOrigin == "*" || allowedOrigin == origin {
				w.Header().Set("Access-Control-Allow-Origin", origin)
				break
			}
		}
	}

	// Set other CORS headers
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Requested-With")
	w.Header().Set("Access-Control-Allow-Credentials", "true")
	w.Header().Set("Access-Control-Max-Age", "86400") // 24 hours
}

// applySecurityHeaders applies basic security headers for private network deployment
func (sm *SecurityMiddleware) applySecurityHeaders(w http.ResponseWriter) {
	// Basic security headers appropriate for private network deployment
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("X-Frame-Options", "DENY")
	w.Header().Set("X-XSS-Protection", "1; mode=block")
	w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
	
	// Content Security Policy for private network deployment
	w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'")
}

// writeErrorResponse writes a standardized error response
func (sm *SecurityMiddleware) writeErrorResponse(w http.ResponseWriter, statusCode int, errorCode, message, details string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	
	errorResponse := fmt.Sprintf(`{
  "error": {
    "code": "%s",
    "message": "%s",
    "details": "%s"
  },
  "timestamp": "%s",
  "path": "%s"
}`, errorCode, message, details, time.Now().UTC().Format(time.RFC3339), "")

	w.Write([]byte(errorResponse))
}

// getClientIP extracts the client IP address from the request
func getClientIP(r *http.Request) string {
	// Check X-Forwarded-For header first (for proxies/load balancers)
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		// Take the first IP in the chain
		ips := strings.Split(xff, ",")
		return strings.TrimSpace(ips[0])
	}

	// Check X-Real-IP header
	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		return xri
	}

	// Fall back to RemoteAddr
	ip := r.RemoteAddr
	if strings.Contains(ip, ":") {
		// Remove port if present
		if host, _, err := net.SplitHostPort(ip); err == nil {
			ip = host
		}
	}

	return ip
}

// parseCORSOrigins parses CORS origins from environment variable
func parseCORSOrigins() []string {
	corsOrigins := os.Getenv("SLACK_MCP_CORS_ORIGINS")
	if corsOrigins == "" {
		return []string{} // Empty means allow all origins
	}

	origins := strings.Split(corsOrigins, ",")
	var result []string
	for _, origin := range origins {
		if trimmed := strings.TrimSpace(origin); trimmed != "" {
			result = append(result, trimmed)
		}
	}

	return result
}

// parseSecurityHeaders parses security headers configuration from environment
func parseSecurityHeaders() bool {
	value := os.Getenv("SLACK_MCP_SECURITY_HEADERS")
	if value == "" {
		return true // Default to enabled
	}

	enabled, err := strconv.ParseBool(value)
	if err != nil {
		return true // Default to enabled on parse error
	}

	return enabled
}

// parseRateLimit parses rate limit configuration from environment
func parseRateLimit() time.Duration {
	value := os.Getenv("SLACK_MCP_RATE_LIMIT")
	if value == "" {
		return time.Minute // Default: 1 request per minute (60 requests per hour)
	}

	// Parse as requests per minute
	requestsPerMinute, err := strconv.Atoi(value)
	if err != nil || requestsPerMinute <= 0 {
		return time.Minute // Default on parse error
	}

	// Convert to duration between requests
	return time.Minute / time.Duration(requestsPerMinute)
}