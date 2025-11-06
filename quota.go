package traefik_quota_plugin

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
)

func init() {
	log.SetOutput(os.Stdout)
}

// CreateConfig creates and initializes the plugin configuration
func CreateConfig() *Config {
	return &Config{}
}

// quotaPlugin holds the plugin instance
type quotaPlugin struct {
	name        string
	next        http.Handler
	config      *Config
	redisClient RedisClient
	managers    map[string]*IdentifierManager
}

// IdentifierManager manages rate limiting and quota for a specific identifier
type IdentifierManager struct {
	config       *IdentifierConfig
	rateLimiter  *RateLimiter
	quotaManager *QuotaManager
}

// QuotaResponse contains the result of quota checking
type QuotaResponse struct {
	Allowed        bool           `json:"allowed"`
	RateLimit      *RateLimitInfo `json:"rate_limit,omitempty"`
	Quota          *QuotaInfo     `json:"quota,omitempty"`
	Identifier     string         `json:"identifier"`
	IdentifierType string         `json:"identifier_type"`
	Reason         string         `json:"reason,omitempty"`
	ResponseCode   int            `json:"response_code,omitempty"`
	ResponseBody   string         `json:"response_body,omitempty"`
}

// New creates and returns a new quota plugin instance
func New(ctx context.Context, next http.Handler, config *Config, name string) (http.Handler, error) {
	// Validate configuration
	if config.Persistence.Redis.Address == "" {
		return nil, fmt.Errorf("redis address is required")
	}

	if len(config.Identifiers) == 0 {
		return nil, fmt.Errorf("at least one identifier is required")
	}

	// Initialize Redis client
	redisClient, err := NewRedisClient(config.Persistence.Redis)
	if err != nil {
		return nil, fmt.Errorf("failed to create Redis client: %w", err)
	}

	// Initialize managers for each identifier
	managers := make(map[string]*IdentifierManager)
	for i, identifierConfig := range config.Identifiers {
		log.Printf("load identifier %s", identifierConfig.Name)
		// Validate identifier config
		if err := identifierConfig.Validate(); err != nil {
			return nil, fmt.Errorf("identifier %d validation failed: %w", i, err)
		}

		// Create a copy of the config to avoid pointer issues
		configCopy := identifierConfig

		// Create manager for this identifier
		manager := &IdentifierManager{
			config:       &configCopy,
			quotaManager: NewQuotaManager(redisClient, configCopy.Quota),
		}

		// Only create rate limiter if rate limiting is enabled
		if configCopy.RateLimit.Enabled {
			manager.rateLimiter = NewRateLimiter(redisClient, configCopy.RateLimit)
		}

		// Use a combination of type, name, and value as key to avoid conflicts
		key := fmt.Sprintf("%s:%s:%s", configCopy.Type, configCopy.Name, configCopy.Value)
		managers[key] = manager

		// Log manager initialization
		rateLimitStatus := "disabled"
		if configCopy.RateLimit.Enabled {
			rateLimitStatus = fmt.Sprintf("%d/%s", configCopy.RateLimit.Rate, configCopy.RateLimit.Period)
		}

		quotaStatus := "disabled"
		if configCopy.Quota.Enabled {
			quotaStatus = fmt.Sprintf("%d/%s", configCopy.Quota.Limit, configCopy.Quota.Period)
		}

		log.Printf("Initialized manager for identifier %s:%s:%s (rate: %s, quota: %s)",
			configCopy.Type, configCopy.Name, configCopy.Value, rateLimitStatus, quotaStatus)
	}

	plugin := &quotaPlugin{
		name:        name,
		next:        next,
		config:      config,
		redisClient: redisClient,
		managers:    managers,
	}

	log.Printf("Quota plugin '%s' initialized with %d identifiers", name, len(managers))
	return plugin, nil
}

// ServeHTTP processes the HTTP request with quota and rate limiting
func (q *quotaPlugin) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
	// Check all identifiers and find the first match
	var response *QuotaResponse
	var matchedManager *IdentifierManager

	for key, manager := range q.managers {
		log.Printf("Checking identifier: %s", key)
		log.Printf("Manager config - Type: %s, Name: %s, Value: %s",
			manager.config.Type, manager.config.Name, manager.config.Value)
		identifier := q.extractIdentifier(req, manager.config)

		// Skip empty identifiers
		if identifier == "" {
			log.Printf("Identifier %s not found in request, skipping", key)
			continue
		}

		// Check this identifier
		resp, err := q.checkIdentifier(req, manager, identifier)
		if err != nil {
			log.Printf("Error checking identifier %s: %v", key, err)
			continue
		}

		resp.IdentifierType = key
		response = resp
		matchedManager = manager
		log.Printf("Identifier matched: %s (allowed: %v)", key, response.Allowed)
		break // Use first matching identifier
	}

	// If no identifier matched, block the request with 403
	if response == nil {
		log.Printf("Access denied: No valid identifier found for request")

		// Set content type for JSON response
		rw.Header().Set("Content-Type", "application/json")

		// Write 403 Forbidden status
		rw.WriteHeader(http.StatusForbidden)

		// Write JSON error response
		errorResponse := `{
			"error": "Access denied",
			"message": "No valid identifier found in request"
		}`
		rw.Write([]byte(errorResponse))
		return
	}

	// Write quota headers to response
	q.writeQuotaHeaders(rw, response)

	// If request is not allowed, return appropriate error
	if !response.Allowed {
		statusCode := response.ResponseCode
		if statusCode == 0 {
			statusCode = http.StatusTooManyRequests
			if response.Reason == "Quota exceeded" {
				statusCode = http.StatusForbidden
			}
		}

		responseBody := response.ResponseBody
		if responseBody == "" {
			responseBody = response.Reason
		}

		log.Printf("Request blocked: %s (identifier: %s, type: %s)",
			response.Reason, response.Identifier, response.IdentifierType)

		// Set content type based on response body format
		if strings.Contains(responseBody, "{") && strings.Contains(responseBody, "}") {
			rw.Header().Set("Content-Type", "application/json")
		} else {
			rw.Header().Set("Content-Type", "text/plain")
		}

		// Write status code and body manually instead of using http.Error
		rw.WriteHeader(statusCode)
		rw.Write([]byte(responseBody))
		return
	}

	// Request is allowed, consume quota if enabled
	if matchedManager.quotaManager.IsQuotaEnabled() {
		ctx := req.Context()
		_, err := matchedManager.quotaManager.ConsumeQuota(ctx, response.Identifier, 1)
		if err != nil {
			log.Printf("Failed to consume quota: %v", err)
		}
	}

	log.Printf("Request allowed for identifier: %s (type: %s)", response.Identifier, response.IdentifierType)
	q.next.ServeHTTP(rw, req)
}

// checkIdentifier checks if a request is allowed for a specific identifier
func (q *quotaPlugin) checkIdentifier(req *http.Request, manager *IdentifierManager, identifier string) (*QuotaResponse, error) {
	ctx := req.Context()

	var rateLimitAllowed = true
	var rateLimitInfo RateLimitInfo

	// Check rate limiting only if enabled and rateLimiter exists
	if manager.config.RateLimit.Enabled && manager.rateLimiter != nil {
		var err error
		rateLimitAllowed, err = manager.rateLimiter.Allow(ctx, identifier)
		if err != nil {
			log.Printf("Rate limiter error: %v", err)
			// In case of error, allow the request (fail open)
			rateLimitAllowed = true
		}

		// Get rate limit info
		rateLimitInfo, err = manager.rateLimiter.GetLimitInfo(ctx, identifier)
		if err != nil {
			log.Printf("Failed to get rate limit info: %v", err)
			rateLimitInfo = RateLimitInfo{}
		}

		// If rate limited, return immediately
		if !rateLimitAllowed {
			return &QuotaResponse{
				Allowed:        false,
				RateLimit:      &rateLimitInfo,
				Identifier:     identifier,
				IdentifierType: manager.config.Type,
				Reason:         "Rate limit exceeded",
				ResponseCode:   manager.config.RateLimit.ResponseReachedLimitCode,
				ResponseBody:   manager.config.RateLimit.ResponseReachedLimitBody,
			}, nil
		}
	}

	// Check quota if enabled
	var quotaInfo *QuotaInfo
	quotaAllowed := true

	if manager.quotaManager.IsQuotaEnabled() {
		var err error
		quotaAllowed, quotaInfo, err = manager.quotaManager.CheckQuota(ctx, identifier)
		if err != nil {
			log.Printf("Quota manager error: %v", err)
			// In case of error, allow the request (fail open)
			quotaAllowed = true
		}
	}

	// If quota exceeded, return
	if !quotaAllowed {
		response := &QuotaResponse{
			Allowed:        false,
			Quota:          quotaInfo,
			Identifier:     identifier,
			IdentifierType: manager.config.Type,
			Reason:         "Quota exceeded",
			ResponseCode:   manager.config.Quota.ResponseReachedLimitCode,
			ResponseBody:   manager.config.Quota.ResponseReachedLimitBody,
		}

		// Only include rate limit info if rate limiting is enabled and rateLimiter exists
		if manager.config.RateLimit.Enabled && manager.rateLimiter != nil {
			response.RateLimit = &rateLimitInfo
		}

		return response, nil
	}

	response := &QuotaResponse{
		Allowed:        true,
		Quota:          quotaInfo,
		Identifier:     identifier,
		IdentifierType: manager.config.Type,
		Reason:         "Request allowed",
	}

	// Only include rate limit info if rate limiting is enabled and rateLimiter exists
	if manager.config.RateLimit.Enabled && manager.rateLimiter != nil {
		response.RateLimit = &rateLimitInfo
	}

	return response, nil
}

// extractIdentifier extracts the identifier from the request based on configuration
func (q *quotaPlugin) extractIdentifier(req *http.Request, config *IdentifierConfig) string {
	switch config.Type {
	case "Header":
		log.Printf("Extracting identifier from header: %s (expected value: %s)", config.Name, config.Value)
		value := req.Header.Get(config.Name)
		log.Printf("Header value from request: '%s'", value)

		if value != "" {
			// If header exists, check if it matches this identifier's expected value
			log.Printf("Comparing header value '%s' with config value '%s': %v", value, config.Value, value == config.Value)
			if value == config.Value {
				log.Printf("Header matches! Returning: %s", value)
				return value
			}
			// If header exists but doesn't match, return empty (no match)
			log.Printf("Header doesn't match config value, returning empty")
			return ""
		}

		// If no header found, check if this is a fallback identifier
		if config.Value == "sk-unknown" || config.Value == "anonymous" || config.Value == "guest" {
			log.Printf("No header found, using fallback value: %s", config.Value)
			return config.Value
		}

		// For specific identifiers, return empty when header is missing
		log.Printf("No header found and not a fallback identifier, returning empty")
		return ""
	case "IP":
		// Extract IP from request
		ip := req.RemoteAddr
		if forwarded := req.Header.Get("X-Forwarded-For"); forwarded != "" {
			// Take first IP in case of multiple
			ips := strings.Split(forwarded, ",")
			ip = strings.TrimSpace(ips[0])
		}
		if realIP := req.Header.Get("X-Real-IP"); realIP != "" {
			ip = realIP
		}
		// Remove port if present
		if idx := strings.LastIndex(ip, ":"); idx != -1 {
			ip = ip[:idx]
		}
		return ip
	case "Query":
		value := req.URL.Query().Get(config.Name)
		if value != "" {
			return value
		}
		return config.Value
	case "Cookie":
		cookie, err := req.Cookie(config.Name)
		if err == nil {
			return cookie.Value
		}
		return config.Value
	default:
		return config.Value
	}
}

// writeQuotaHeaders writes quota headers to HTTP response
func (q *quotaPlugin) writeQuotaHeaders(w http.ResponseWriter, response *QuotaResponse) {
	// Add rate limit headers
	if response.RateLimit != nil {
		w.Header().Set("X-RateLimit-Limit", strconv.Itoa(response.RateLimit.Limit))
		w.Header().Set("X-RateLimit-Remaining", strconv.Itoa(response.RateLimit.Available))
		w.Header().Set("X-RateLimit-Reset", strconv.FormatInt(response.RateLimit.ResetTime.Unix(), 10))
		if response.RateLimit.RetryAfter > 0 {
			w.Header().Set("Retry-After", strconv.FormatInt(int64(response.RateLimit.RetryAfter.Seconds()), 10))
		}
	}

	// Add quota headers
	if response.Quota != nil {
		w.Header().Set("X-Quota-Limit", strconv.FormatInt(response.Quota.Limit, 10))
		w.Header().Set("X-Quota-Used", strconv.FormatInt(response.Quota.Used, 10))
		w.Header().Set("X-Quota-Remaining", strconv.FormatInt(response.Quota.Remaining, 10))
		w.Header().Set("X-Quota-Reset", strconv.FormatInt(response.Quota.ResetTime.Unix(), 10))
	}
}
