package traefik_quota_plugin

import (
	"context"
	"fmt"
	"strconv"
	"time"
)

// QuotaManager manages quota tracking and enforcement
type QuotaManager struct {
	redisClient RedisClient
	config      QuotaSettings
}

// QuotaInfo contains information about quota usage
type QuotaInfo struct {
	Limit     int64         `json:"limit"`      // Total quota limit
	Used      int64         `json:"used"`       // Currently used quota
	Remaining int64         `json:"remaining"`  // Remaining quota
	Period    string        `json:"period"`     // Quota period (Daily/Weekly/Monthly)
	ResetTime time.Time     `json:"reset_time"` // When quota resets
	ResetIn   time.Duration `json:"reset_in"`   // Time until reset
}

// NewQuotaManager creates a new quota manager
func NewQuotaManager(redisClient RedisClient, config QuotaSettings) *QuotaManager {
	return &QuotaManager{
		redisClient: redisClient,
		config:      config,
	}
}

// CheckQuota checks if a request is allowed under the quota
func (qm *QuotaManager) CheckQuota(ctx context.Context, identifier string) (bool, *QuotaInfo, error) {
	if !qm.config.Enabled {
		return true, nil, nil
	}

	// Get current quota usage
	info, err := qm.GetQuotaInfo(ctx, identifier)
	if err != nil {
		return false, nil, fmt.Errorf("failed to get quota info: %w", err)
	}

	// Check if quota is exceeded
	if info.Used >= info.Limit {
		return false, info, nil
	}

	return true, info, nil
}

// ConsumeQuota consumes quota for a request
func (qm *QuotaManager) ConsumeQuota(ctx context.Context, identifier string, amount int64) (*QuotaInfo, error) {
	if !qm.config.Enabled {
		return nil, nil
	}

	if amount <= 0 {
		amount = 1
	}

	// Generate quota key
	periodKey := GetQuotaPeriodKey(qm.config.Period)
	key := GetQuotaKey(identifier, periodKey)

	// Increment usage
	newUsage, err := qm.redisClient.IncrBy(ctx, key, amount)
	if err != nil {
		return nil, fmt.Errorf("failed to increment quota: %w", err)
	}

	// Set expiration if this is a new key
	if newUsage == amount {
		// Set expiration to the end of the current period
		resetTime := qm.getNextResetTime()
		timeUntilReset := time.Until(resetTime)

		if err := qm.redisClient.Expire(ctx, key, timeUntilReset); err != nil {
			return nil, fmt.Errorf("failed to set quota expiration: %w", err)
		}
	}

	// Get updated quota info
	info, err := qm.GetQuotaInfo(ctx, identifier)
	if err != nil {
		return nil, fmt.Errorf("failed to get updated quota info: %w", err)
	}

	return info, nil
}

// GetQuotaInfo retrieves current quota information
func (qm *QuotaManager) GetQuotaInfo(ctx context.Context, identifier string) (*QuotaInfo, error) {
	if !qm.config.Enabled {
		return &QuotaInfo{
			Limit:     0,
			Used:      0,
			Remaining: 0,
			Period:    "Disabled",
			ResetTime: time.Time{},
			ResetIn:   0,
		}, nil
	}

	// Generate quota key
	periodKey := GetQuotaPeriodKey(qm.config.Period)
	key := GetQuotaKey(identifier, periodKey)

	// Get current usage
	usageStr, err := qm.redisClient.Get(ctx, key)
	var used int64 = 0
	if err == nil {
		if parsedUsage, parseErr := strconv.ParseInt(usageStr, 10, 64); parseErr == nil {
			used = parsedUsage
		}
	}

	// Calculate remaining quota
	remaining := qm.config.Limit - used
	if remaining < 0 {
		remaining = 0
	}

	// Calculate reset time
	resetTime := qm.getNextResetTime()
	resetIn := time.Until(resetTime)

	return &QuotaInfo{
		Limit:     qm.config.Limit,
		Used:      used,
		Remaining: remaining,
		Period:    qm.config.Period,
		ResetTime: resetTime,
		ResetIn:   resetIn,
	}, nil
}

// ResetQuota resets the quota for a specific identifier
func (qm *QuotaManager) ResetQuota(ctx context.Context, identifier string) error {
	if !qm.config.Enabled {
		return nil
	}

	// Generate quota key
	periodKey := GetQuotaPeriodKey(qm.config.Period)
	key := GetQuotaKey(identifier, periodKey)

	// Reset to 0
	return qm.redisClient.Set(ctx, key, 0, 0)
}

// GetUsageHistory returns usage history for different periods
func (qm *QuotaManager) GetUsageHistory(ctx context.Context, identifier string, periods []string) (map[string]int64, error) {
	if !qm.config.Enabled {
		return nil, nil
	}

	history := make(map[string]int64)

	for _, period := range periods {
		key := GetQuotaKey(identifier, period)
		usageStr, err := qm.redisClient.Get(ctx, key)
		if err != nil {
			history[period] = 0
			continue
		}

		usage, err := strconv.ParseInt(usageStr, 10, 64)
		if err != nil {
			history[period] = 0
			continue
		}

		history[period] = usage
	}

	return history, nil
}

// getNextResetTime calculates when the quota will reset next
func (qm *QuotaManager) getNextResetTime() time.Time {
	now := time.Now()

	switch qm.config.Period {
	case "Daily":
		// Reset at midnight
		return time.Date(now.Year(), now.Month(), now.Day()+1, 0, 0, 0, 0, now.Location())
	case "Weekly":
		// Reset at Sunday midnight
		daysUntilSunday := (7 - int(now.Weekday())) % 7
		if daysUntilSunday == 0 {
			daysUntilSunday = 7
		}
		return time.Date(now.Year(), now.Month(), now.Day()+daysUntilSunday, 0, 0, 0, 0, now.Location())
	case "Monthly":
		// Reset at the first day of next month
		return time.Date(now.Year(), now.Month()+1, 1, 0, 0, 0, 0, now.Location())
	default:
		// Default to daily
		return time.Date(now.Year(), now.Month(), now.Day()+1, 0, 0, 0, 0, now.Location())
	}
}

// IsQuotaEnabled checks if quota is enabled
func (qm *QuotaManager) IsQuotaEnabled() bool {
	return qm.config.Enabled
}

// GetQuotaLimit returns the configured quota limit
func (qm *QuotaManager) GetQuotaLimit() int64 {
	return qm.config.Limit
}

// GetQuotaPeriod returns the configured quota period
func (qm *QuotaManager) GetQuotaPeriod() string {
	return qm.config.Period
}

// SetQuotaUsage sets the quota usage to a specific value (for testing/admin purposes)
func (qm *QuotaManager) SetQuotaUsage(ctx context.Context, identifier string, usage int64) error {
	if !qm.config.Enabled {
		return nil
	}

	// Generate quota key
	periodKey := GetQuotaPeriodKey(qm.config.Period)
	key := GetQuotaKey(identifier, periodKey)

	// Set usage
	return qm.redisClient.Set(ctx, key, usage, 0)
}

// GetActiveQuotaKeys returns all active quota keys (for monitoring/admin purposes)
func (qm *QuotaManager) GetActiveQuotaKeys(ctx context.Context) ([]string, error) {
	// This would require a Redis SCAN operation in a real implementation
	// For the simple implementation, we can't easily get all keys
	// In production, you might want to maintain a separate index of active keys
	return []string{}, nil
}

// CleanupExpiredQuotas removes expired quota entries (maintenance function)
func (qm *QuotaManager) CleanupExpiredQuotas(ctx context.Context) error {
	// This would be implemented as a background job in production
	// It would scan for expired keys and remove them
	return nil
}
