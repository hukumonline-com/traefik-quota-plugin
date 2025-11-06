package traefik_quota_plugin

import (
	"fmt"
	"time"
)

// Config holds the complete plugin configuration (main entry point)
type Config struct {
	Persistence PersistenceConfig  `json:"persistence,omitempty" yaml:"Persistence,omitempty"`
	Identifiers []IdentifierConfig `json:"identifiers,omitempty" yaml:"Identifiers,omitempty"`
}

// QuotaConfig holds the complete quota configuration (for backward compatibility)
type QuotaConfig struct {
	Persistence PersistenceConfig  `json:"persistence,omitempty" yaml:"Persistence,omitempty"`
	Identifiers []IdentifierConfig `json:"identifiers,omitempty" yaml:"Identifiers,omitempty"`
}

// PersistenceConfig holds Redis configuration
type PersistenceConfig struct {
	Redis RedisConfig `json:"redis,omitempty" yaml:"Redis,omitempty"`
}

// RedisConfig holds Redis connection settings
type RedisConfig struct {
	Address  string `json:"address,omitempty" yaml:"Address,omitempty"`
	Password string `json:"password,omitempty" yaml:"Password,omitempty"`
	DB       int    `json:"db,omitempty" yaml:"DB,omitempty"`
}

// IdentifierConfig holds identifier configuration with its own rate limit and quota
type IdentifierConfig struct {
	Type      string          `json:"type,omitempty" yaml:"Type,omitempty"`   // Header, IP, etc.
	Name      string          `json:"name,omitempty" yaml:"Name,omitempty"`   // Header name
	Value     string          `json:"value,omitempty" yaml:"Value,omitempty"` // Default value
	RateLimit RateLimitConfig `json:"rate_limit,omitempty" yaml:"RateLimit,omitempty"`
	Quota     QuotaSettings   `json:"quota,omitempty" yaml:"Quota,omitempty"`
}

// RateLimitConfig holds rate limiting configuration
type RateLimitConfig struct {
	Enabled                  bool   `json:"enabled,omitempty" yaml:"Enabled,omitempty"`                                      // Enable/disable rate limiting
	Rate                     int    `json:"rate,omitempty" yaml:"Rate,omitempty"`                                            // Requests per period
	Burst                    int    `json:"burst,omitempty" yaml:"Burst,omitempty"`                                          // Burst capacity
	Period                   string `json:"period,omitempty" yaml:"Period,omitempty"`                                        // Time period (1m, 1h, etc.)
	ResponseReachedLimitCode int    `json:"response_reached_limit_code,omitempty" yaml:"ResponseReachedLimitCode,omitempty"` // HTTP status code when limit reached
	ResponseReachedLimitBody string `json:"response_reached_limit_body,omitempty" yaml:"ResponseReachedLimitBody,omitempty"` // Response body when limit reached
}

// QuotaSettings holds quota configuration
type QuotaSettings struct {
	Enabled                  bool   `json:"enabled,omitempty" yaml:"Enabled,omitempty"`
	Limit                    int64  `json:"limit,omitempty" yaml:"Limit,omitempty"`                                          // Total quota limit
	Period                   string `json:"period,omitempty" yaml:"Period,omitempty"`                                        // Daily, Weekly, Monthly
	ResponseReachedLimitCode int    `json:"response_reached_limit_code,omitempty" yaml:"ResponseReachedLimitCode,omitempty"` // HTTP status code when limit reached
	ResponseReachedLimitBody string `json:"response_reached_limit_body,omitempty" yaml:"ResponseReachedLimitBody,omitempty"` // Response body when limit reached
}

// ParseRateLimitPeriod parses rate limit period string to duration
func (rlc *RateLimitConfig) ParseRateLimitPeriod() (time.Duration, error) {
	if rlc.Period == "" {
		return time.Minute, nil // default to 1 minute
	}

	return time.ParseDuration(rlc.Period)
}

// ParseQuotaPeriod parses quota period string to duration
func (qs *QuotaSettings) ParseQuotaPeriod() (time.Duration, error) {
	switch qs.Period {
	case "Daily":
		return 24 * time.Hour, nil
	case "Weekly":
		return 7 * 24 * time.Hour, nil
	case "Monthly":
		return 30 * 24 * time.Hour, nil // Approximation
	default:
		return 0, fmt.Errorf("unsupported quota period: %s", qs.Period)
	}
}

// Validate validates the quota configuration
func (qc *QuotaConfig) Validate() error {
	// Validate Redis config
	if qc.Persistence.Redis.Address == "" {
		return fmt.Errorf("redis address is required")
	}

	// Validate identifiers
	if len(qc.Identifiers) == 0 {
		return fmt.Errorf("at least one identifier is required")
	}

	for i, identifier := range qc.Identifiers {
		if err := identifier.Validate(); err != nil {
			return fmt.Errorf("identifier %d validation failed: %w", i, err)
		}
	}

	return nil
}

// Validate validates the identifier configuration
func (ic *IdentifierConfig) Validate() error {
	// Validate identifier config
	if ic.Type == "" {
		return fmt.Errorf("identifier type is required")
	}
	if ic.Type == "Header" && ic.Name == "" {
		return fmt.Errorf("header name is required for header-based identification")
	}

	// Check that at least one feature is enabled
	if !ic.RateLimit.Enabled && !ic.Quota.Enabled {
		return fmt.Errorf("at least one feature (rate limit or quota) must be enabled")
	}

	// Validate rate limit config if enabled
	if ic.RateLimit.Enabled {
		if ic.RateLimit.Rate <= 0 {
			return fmt.Errorf("rate limit rate must be positive when rate limiting is enabled")
		}
		if ic.RateLimit.Burst <= 0 {
			return fmt.Errorf("rate limit burst must be positive when rate limiting is enabled")
		}
		if _, err := ic.RateLimit.ParseRateLimitPeriod(); err != nil {
			return fmt.Errorf("invalid rate limit period: %w", err)
		}
	}

	// Validate quota config if enabled
	if ic.Quota.Enabled {
		if ic.Quota.Limit <= 0 {
			return fmt.Errorf("quota limit must be positive when quota is enabled")
		}
		if _, err := ic.Quota.ParseQuotaPeriod(); err != nil {
			return fmt.Errorf("invalid quota period: %w", err)
		}
	}

	return nil
}

// GetIdentifier extracts identifier from request based on configuration
func (ic *IdentifierConfig) GetIdentifier(req interface{}) string {
	// This will be implemented based on request type
	// For now return default value
	return ic.Value
}
