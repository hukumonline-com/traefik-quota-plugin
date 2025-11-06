package traefik_quota_plugin

import (
	"context"
	"fmt"
	"math"
	"strconv"
	"time"
)

// RateLimiter implements token bucket algorithm for rate limiting
type RateLimiter struct {
	redisClient RedisClient
	config      RateLimitConfig
}

// TokenBucket represents the current state of a token bucket
type TokenBucket struct {
	Tokens       float64       `json:"tokens"`
	LastRefill   time.Time     `json:"last_refill"`
	Rate         int           `json:"rate"`
	Burst        int           `json:"burst"`
	RefillPeriod time.Duration `json:"refill_period"`
}

// NewRateLimiter creates a new rate limiter
func NewRateLimiter(redisClient RedisClient, config RateLimitConfig) *RateLimiter {
	return &RateLimiter{
		redisClient: redisClient,
		config:      config,
	}
}

// Allow checks if a request is allowed under the rate limit
func (rl *RateLimiter) Allow(ctx context.Context, identifier string) (bool, error) {
	key := GetRateLimitKey(identifier)

	// Get current bucket state
	bucket, err := rl.getBucket(ctx, key)
	if err != nil {
		return false, fmt.Errorf("failed to get bucket: %w", err)
	}

	// Refill tokens
	now := time.Now()
	bucket = rl.refillBucket(bucket, now)

	// Check if we have tokens
	if bucket.Tokens >= 1.0 {
		// Consume one token
		bucket.Tokens -= 1.0

		// Save updated bucket
		if err := rl.saveBucket(ctx, key, bucket); err != nil {
			return false, fmt.Errorf("failed to save bucket: %w", err)
		}

		return true, nil
	}

	// No tokens available
	return false, nil
}

// AllowN checks if N requests are allowed under the rate limit
func (rl *RateLimiter) AllowN(ctx context.Context, identifier string, n int) (bool, error) {
	if n <= 0 {
		return true, nil
	}

	key := GetRateLimitKey(identifier)

	// Get current bucket state
	bucket, err := rl.getBucket(ctx, key)
	if err != nil {
		return false, fmt.Errorf("failed to get bucket: %w", err)
	}

	// Refill tokens
	now := time.Now()
	bucket = rl.refillBucket(bucket, now)

	// Check if we have enough tokens
	if bucket.Tokens >= float64(n) {
		// Consume tokens
		bucket.Tokens -= float64(n)

		// Save updated bucket
		if err := rl.saveBucket(ctx, key, bucket); err != nil {
			return false, fmt.Errorf("failed to save bucket: %w", err)
		}

		return true, nil
	}

	// Not enough tokens available
	return false, nil
}

// GetCurrentTokens returns the current number of tokens available
func (rl *RateLimiter) GetCurrentTokens(ctx context.Context, identifier string) (float64, error) {
	key := GetRateLimitKey(identifier)

	// Get current bucket state
	bucket, err := rl.getBucket(ctx, key)
	if err != nil {
		return 0, fmt.Errorf("failed to get bucket: %w", err)
	}

	// Refill tokens
	now := time.Now()
	bucket = rl.refillBucket(bucket, now)

	return bucket.Tokens, nil
}

// Reset resets the rate limiter for a specific identifier
func (rl *RateLimiter) Reset(ctx context.Context, identifier string) error {
	key := GetRateLimitKey(identifier)

	// Create a new full bucket
	period, err := rl.config.ParseRateLimitPeriod()
	if err != nil {
		return fmt.Errorf("invalid period: %w", err)
	}

	bucket := TokenBucket{
		Tokens:       float64(rl.config.Burst),
		LastRefill:   time.Now(),
		Rate:         rl.config.Rate,
		Burst:        rl.config.Burst,
		RefillPeriod: period,
	}

	return rl.saveBucket(ctx, key, bucket)
}

// getBucket retrieves the current bucket state from Redis
func (rl *RateLimiter) getBucket(ctx context.Context, key string) (TokenBucket, error) {
	// Try to get existing bucket
	bucketData, err := rl.redisClient.Get(ctx, key+":tokens")
	if err != nil {
		// Bucket doesn't exist, create new one
		return rl.createNewBucket()
	}

	tokens, err := strconv.ParseFloat(bucketData, 64)
	if err != nil {
		return rl.createNewBucket()
	}

	lastRefillData, err := rl.redisClient.Get(ctx, key+":last_refill")
	if err != nil {
		return rl.createNewBucket()
	}

	lastRefillUnix, err := strconv.ParseInt(lastRefillData, 10, 64)
	if err != nil {
		return rl.createNewBucket()
	}

	period, err := rl.config.ParseRateLimitPeriod()
	if err != nil {
		return TokenBucket{}, fmt.Errorf("invalid period: %w", err)
	}

	return TokenBucket{
		Tokens:       tokens,
		LastRefill:   time.Unix(0, lastRefillUnix),
		Rate:         rl.config.Rate,
		Burst:        rl.config.Burst,
		RefillPeriod: period,
	}, nil
}

// createNewBucket creates a new token bucket with default values
func (rl *RateLimiter) createNewBucket() (TokenBucket, error) {
	period, err := rl.config.ParseRateLimitPeriod()
	if err != nil {
		return TokenBucket{}, fmt.Errorf("invalid period: %w", err)
	}

	return TokenBucket{
		Tokens:       float64(rl.config.Burst),
		LastRefill:   time.Now(),
		Rate:         rl.config.Rate,
		Burst:        rl.config.Burst,
		RefillPeriod: period,
	}, nil
}

// refillBucket refills tokens based on elapsed time
func (rl *RateLimiter) refillBucket(bucket TokenBucket, now time.Time) TokenBucket {
	// Calculate time elapsed since last refill
	elapsed := now.Sub(bucket.LastRefill)

	// Calculate tokens to add based on rate
	tokensToAdd := float64(bucket.Rate) * elapsed.Seconds() / bucket.RefillPeriod.Seconds()

	// Add tokens, but don't exceed burst capacity
	bucket.Tokens = math.Min(bucket.Tokens+tokensToAdd, float64(bucket.Burst))
	bucket.LastRefill = now

	return bucket
}

// saveBucket saves the bucket state to Redis
func (rl *RateLimiter) saveBucket(ctx context.Context, key string, bucket TokenBucket) error {
	// Calculate expiration (2x the refill period to be safe)
	expiration := bucket.RefillPeriod * 2

	// Save tokens
	if err := rl.redisClient.Set(ctx, key+":tokens", bucket.Tokens, expiration); err != nil {
		return fmt.Errorf("failed to save tokens: %w", err)
	}

	// Save last refill time
	if err := rl.redisClient.Set(ctx, key+":last_refill", bucket.LastRefill.UnixNano(), expiration); err != nil {
		return fmt.Errorf("failed to save last refill: %w", err)
	}

	return nil
}

// GetLimitInfo returns information about the current rate limit state
func (rl *RateLimiter) GetLimitInfo(ctx context.Context, identifier string) (RateLimitInfo, error) {
	key := GetRateLimitKey(identifier)

	bucket, err := rl.getBucket(ctx, key)
	if err != nil {
		return RateLimitInfo{}, fmt.Errorf("failed to get bucket: %w", err)
	}

	// Refill tokens
	now := time.Now()
	bucket = rl.refillBucket(bucket, now)

	// Calculate time until next token
	var timeUntilReset time.Duration
	if bucket.Tokens < float64(bucket.Burst) {
		timeForOneToken := bucket.RefillPeriod.Seconds() / float64(bucket.Rate)
		timeUntilReset = time.Duration(timeForOneToken * float64(time.Second))
	}

	return RateLimitInfo{
		Limit:      bucket.Rate,
		Burst:      bucket.Burst,
		Available:  int(bucket.Tokens),
		ResetTime:  now.Add(timeUntilReset),
		RetryAfter: timeUntilReset,
	}, nil
}

// RateLimitInfo contains information about rate limit state
type RateLimitInfo struct {
	Limit      int           `json:"limit"`       // Requests per period
	Burst      int           `json:"burst"`       // Burst capacity
	Available  int           `json:"available"`   // Available tokens
	ResetTime  time.Time     `json:"reset_time"`  // When limit resets
	RetryAfter time.Duration `json:"retry_after"` // Time to wait before retry
}
