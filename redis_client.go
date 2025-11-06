package traefik_quota_plugin

import (
	"bufio"
	"context"
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"
)

// RedisClient interface for Redis operations
type RedisClient interface {
	Ping(ctx context.Context) (string, error)
	Get(ctx context.Context, key string) (string, error)
	Set(ctx context.Context, key string, value interface{}, expiration time.Duration) error
	Incr(ctx context.Context, key string) (int64, error)
	IncrBy(ctx context.Context, key string, value int64) (int64, error)
	Expire(ctx context.Context, key string, expiration time.Duration) error
	TTL(ctx context.Context, key string) (time.Duration, error)
	Exists(ctx context.Context, keys ...string) (int64, error)
	Close() error
}

// SimpleRedisClient implements a basic Redis client using raw TCP connection
type SimpleRedisClient struct {
	address  string
	password string
	db       int
	conn     net.Conn
	reader   *bufio.Reader
	timeout  time.Duration
}

// NewRedisClient creates a new simple Redis client
func NewRedisClient(config RedisConfig) (RedisClient, error) {
	client := &SimpleRedisClient{
		address:  config.Address,
		password: config.Password,
		db:       config.DB,
		timeout:  5 * time.Second,
	}

	// Test connection
	if err := client.connect(); err != nil {
		return nil, fmt.Errorf("failed to connect to Redis: %w", err)
	}

	// Test with ping
	_, err := client.Ping(context.Background())
	if err != nil {
		client.Close()
		return nil, fmt.Errorf("failed to ping Redis: %w", err)
	}

	return client, nil
}

// Ping sends a PING command to Redis
func (c *SimpleRedisClient) Ping(ctx context.Context) (string, error) {
	if err := c.writeCommand("PING"); err != nil {
		return "", err
	}

	resp, err := c.readResponse()
	if err != nil {
		return "", err
	}

	return resp, nil
}

// Get retrieves a value from Redis
func (c *SimpleRedisClient) Get(ctx context.Context, key string) (string, error) {
	if err := c.writeCommand("GET", key); err != nil {
		return "", err
	}

	resp, err := c.readResponse()
	if err != nil {
		if strings.Contains(err.Error(), "key not found") {
			return "", fmt.Errorf("key not found")
		}
		return "", err
	}

	return resp, nil
}

// Set stores a value in Redis
func (c *SimpleRedisClient) Set(ctx context.Context, key string, value interface{}, expiration time.Duration) error {
	valueStr := fmt.Sprintf("%v", value)

	if expiration > 0 {
		seconds := int(expiration.Seconds())
		if err := c.writeCommand("SETEX", key, strconv.Itoa(seconds), valueStr); err != nil {
			return err
		}
	} else {
		if err := c.writeCommand("SET", key, valueStr); err != nil {
			return err
		}
	}

	resp, err := c.readResponse()
	if err != nil {
		return err
	}

	if !strings.HasPrefix(resp, "OK") {
		return fmt.Errorf("set failed: %s", resp)
	}

	return nil
}

// Incr increments a key's value by 1
func (c *SimpleRedisClient) Incr(ctx context.Context, key string) (int64, error) {
	if err := c.writeCommand("INCR", key); err != nil {
		return 0, err
	}

	resp, err := c.readResponse()
	if err != nil {
		return 0, err
	}

	value, err := strconv.ParseInt(resp, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid integer response: %s", resp)
	}

	return value, nil
}

// IncrBy increments a key's value by a specified amount
func (c *SimpleRedisClient) IncrBy(ctx context.Context, key string, value int64) (int64, error) {
	if err := c.writeCommand("INCRBY", key, strconv.FormatInt(value, 10)); err != nil {
		return 0, err
	}

	resp, err := c.readResponse()
	if err != nil {
		return 0, err
	}

	result, err := strconv.ParseInt(resp, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid integer response: %s", resp)
	}

	return result, nil
}

// Expire sets an expiration time for a key
func (c *SimpleRedisClient) Expire(ctx context.Context, key string, expiration time.Duration) error {
	seconds := int(expiration.Seconds())
	if err := c.writeCommand("EXPIRE", key, strconv.Itoa(seconds)); err != nil {
		return err
	}

	resp, err := c.readResponse()
	if err != nil {
		return err
	}

	if resp != "1" {
		return fmt.Errorf("expire failed: key may not exist")
	}

	return nil
}

// TTL returns the remaining time to live for a key
func (c *SimpleRedisClient) TTL(ctx context.Context, key string) (time.Duration, error) {
	if err := c.writeCommand("TTL", key); err != nil {
		return 0, err
	}

	resp, err := c.readResponse()
	if err != nil {
		return 0, err
	}

	seconds, err := strconv.ParseInt(resp, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid TTL response: %s", resp)
	}

	if seconds == -1 {
		return -1, nil // No expiration
	}
	if seconds == -2 {
		return 0, fmt.Errorf("key does not exist")
	}

	return time.Duration(seconds) * time.Second, nil
}

// Exists checks if keys exist
func (c *SimpleRedisClient) Exists(ctx context.Context, keys ...string) (int64, error) {
	args := append([]string{"EXISTS"}, keys...)
	if err := c.writeCommand(args...); err != nil {
		return 0, err
	}

	resp, err := c.readResponse()
	if err != nil {
		return 0, err
	}

	count, err := strconv.ParseInt(resp, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid exists response: %s", resp)
	}

	return count, nil
}

// Close closes the Redis connection
func (c *SimpleRedisClient) Close() error {
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}

// connect establishes connection to Redis
func (c *SimpleRedisClient) connect() error {
	conn, err := net.DialTimeout("tcp", c.address, c.timeout)
	if err != nil {
		return err
	}

	c.conn = conn
	c.reader = bufio.NewReader(conn)

	// Authenticate if password is provided
	if c.password != "" {
		if err := c.auth(); err != nil {
			c.conn.Close()
			return err
		}
	}

	// Select database
	if c.db != 0 {
		if err := c.selectDB(); err != nil {
			c.conn.Close()
			return err
		}
	}

	return nil
}

// auth authenticates with Redis
func (c *SimpleRedisClient) auth() error {
	cmd := fmt.Sprintf("*2\r\n$4\r\nAUTH\r\n$%d\r\n%s\r\n", len(c.password), c.password)
	_, err := c.conn.Write([]byte(cmd))
	if err != nil {
		return err
	}

	resp, err := c.readResponse()
	if err != nil {
		return err
	}

	if !strings.HasPrefix(resp, "+OK") {
		return fmt.Errorf("authentication failed: %s", resp)
	}

	return nil
}

// selectDB selects Redis database
func (c *SimpleRedisClient) selectDB() error {
	dbStr := strconv.Itoa(c.db)
	cmd := fmt.Sprintf("*2\r\n$6\r\nSELECT\r\n$%d\r\n%s\r\n", len(dbStr), dbStr)
	_, err := c.conn.Write([]byte(cmd))
	if err != nil {
		return err
	}

	resp, err := c.readResponse()
	if err != nil {
		return err
	}

	if !strings.HasPrefix(resp, "+OK") {
		return fmt.Errorf("select database failed: %s", resp)
	}

	return nil
}

// writeCommand sends a Redis command
func (c *SimpleRedisClient) writeCommand(args ...string) error {
	if c.conn == nil {
		if err := c.connect(); err != nil {
			return err
		}
	}

	// Build RESP command
	cmd := fmt.Sprintf("*%d\r\n", len(args))
	for _, arg := range args {
		cmd += fmt.Sprintf("$%d\r\n%s\r\n", len(arg), arg)
	}

	_, err := c.conn.Write([]byte(cmd))
	return err
}

// readResponse reads Redis response
func (c *SimpleRedisClient) readResponse() (string, error) {
	if c.reader == nil {
		return "", fmt.Errorf("no connection")
	}

	line, err := c.reader.ReadString('\n')
	if err != nil {
		return "", err
	}

	line = strings.TrimSpace(line)

	switch line[0] {
	case '+': // Simple string
		return line[1:], nil
	case '-': // Error
		return "", fmt.Errorf("redis error: %s", line[1:])
	case ':': // Integer
		return line[1:], nil
	case '$': // Bulk string
		length, err := strconv.Atoi(line[1:])
		if err != nil {
			return "", err
		}
		if length == -1 {
			return "", fmt.Errorf("key not found")
		}
		if length == 0 {
			c.reader.ReadString('\n') // consume \r\n
			return "", nil
		}

		data := make([]byte, length)
		_, err = c.reader.Read(data)
		if err != nil {
			return "", err
		}
		c.reader.ReadString('\n') // consume \r\n
		return string(data), nil
	case '*': // Array
		// For simplicity, we'll handle basic cases
		return line, nil
	default:
		return line, nil
	}
}

// GetQuotaKey generates a Redis key for quota tracking
func GetQuotaKey(identifier, period string) string {
	return fmt.Sprintf("quota:%s:%s", identifier, period)
}

// GetRateLimitKey generates a Redis key for rate limiting
func GetRateLimitKey(identifier string) string {
	return fmt.Sprintf("ratelimit:%s", identifier)
}

// GetQuotaPeriodKey generates a period-specific key
func GetQuotaPeriodKey(period string) string {
	now := time.Now()
	switch period {
	case "Daily":
		return now.Format("2006-01-02")
	case "Weekly":
		year, week := now.ISOWeek()
		return fmt.Sprintf("%d-W%02d", year, week)
	case "Monthly":
		return now.Format("2006-01")
	default:
		return now.Format("2006-01-02")
	}
}
