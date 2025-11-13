# Rate Limiting

Rate limiting menggunakan algoritma Token Bucket untuk mengontrol jumlah request per satuan waktu. Fitur ini memungkinkan burst traffic dalam batas tertentu sambil tetap menjaga throughput rata-rata.

## Konsep Dasar

Rate limiting bekerja dengan konsep "token bucket":
- **Rate**: Kecepatan pengisian token (requests per second)
- **Burst**: Kapasitas maksimal token dalam bucket
- **Period**: Periode pengisian (1s, 1m, 1h, 1d)

## Konfigurasi Rate Limit

### Basic Configuration
```yaml
identifiers:
  - type: "Header"
    name: "X-API-Key"
    value: "premium-user"
    rate_limit:
      enabled: true
      rate: 100          # 100 requests per period
      burst: 200         # Maximum burst capacity
      period: "1m"       # Per minute
      response_reached_limit_code: 429
      response_reached_limit_body: |
        {
          "error": "Rate limit exceeded",
          "retry_after": 60
        }
```

### Parameter Details

#### Enabled
- **Type**: boolean
- **Default**: false
- **Description**: Enable/disable rate limiting untuk identifier ini

#### Rate
- **Type**: integer
- **Required**: true (jika enabled=true)
- **Description**: Jumlah requests yang diizinkan per period
- **Example**: `rate: 100` = 100 requests per period

#### Burst  
- **Type**: integer
- **Required**: true (jika enabled=true)
- **Description**: Kapasitas maksimal token bucket
- **Recommendation**: Biasanya 1-5x dari rate value
- **Example**: `burst: 200` untuk `rate: 100`

#### Period
- **Type**: string (duration)
- **Required**: true (jika enabled=true)
- **Valid Values**: "1s", "30s", "1m", "5m", "1h", "24h", "1d"
- **Example**: `period: "1m"` = per minute

#### Response Codes & Bodies
- **ResponseReachedLimitCode**: HTTP status code (default: 429)
- **ResponseReachedLimitBody**: Custom JSON response body

## Rate Configurations Examples

### 1. API Endpoint Protection
```yaml
# Standard API rate limiting
rate_limit:
  enabled: true
  rate: 60              # 60 requests/minute = 1 per second
  burst: 10             # Allow burst up to 10 requests
  period: "1m"
  response_reached_limit_code: 429
  response_reached_limit_body: |
    {
      "error": "API rate limit exceeded",
      "limit": 60,
      "period": "1 minute"
    }
```

### 2. Premium User Tier
```yaml
# Higher limits for premium users
rate_limit:
  enabled: true
  rate: 1000            # 1000 requests/hour
  burst: 100            # Large burst capacity
  period: "1h"
  response_reached_limit_code: 429
  response_reached_limit_body: |
    {
      "error": "Premium rate limit exceeded",
      "tier": "premium",
      "contact": "upgrade@example.com"
    }
```

### 3. Login/Authentication Protection
```yaml
# Brute force protection
rate_limit:
  enabled: true
  rate: 5               # Only 5 attempts per minute
  burst: 3              # Small burst for legitimate retries
  period: "1m"
  response_reached_limit_code: 429
  response_reached_limit_body: |
    {
      "error": "Too many login attempts",
      "retry_after": 60,
      "lockout_duration": "1 minute"
    }
```

### 4. File Upload Limiting
```yaml
# Bandwidth protection
rate_limit:
  enabled: true
  rate: 10              # 10 uploads per hour
  burst: 3              # Allow small burst
  period: "1h"
  response_reached_limit_code: 429
  response_reached_limit_body: |
    {
      "error": "Upload rate limit exceeded",
      "max_uploads": 10,
      "period": "1 hour"
    }
```

### 5. High-Performance Internal API
```yaml
# Internal service communication
rate_limit:
  enabled: true
  rate: 10000           # 10k requests/minute
  burst: 5000           # Large burst capacity
  period: "1m"
  response_reached_limit_code: 503
  response_reached_limit_body: |
    {
      "error": "Service temporarily unavailable",
      "type": "internal_rate_limit"
    }
```

## Token Bucket Behavior

### Pengisian Token
```
Rate: 100/minute = 1.67 tokens/second
Burst: 200 tokens

Waktu 0s:    [200 tokens] (full bucket)
Request 50:  [150 tokens] (consumed 50)
Waktu 30s:   [200 tokens] (refilled: 150 + 50 = 200, capped at burst)
```

### Burst Traffic Handling
```yaml
# Configuration: Rate=10/min, Burst=30
rate: 10
burst: 30
period: "1m"

# Scenario:
# T=0s:    30 tokens available (bucket full)
# T=0s:    Send 25 requests → 5 tokens remaining  ✅
# T=5s:    Send 10 requests → DENIED (only 5 tokens) ❌
# T=60s:   Send 10 requests → 5 tokens remaining  ✅ (bucket refilled)
```

## Response Headers

Saat rate limiting aktif, plugin menambahkan headers informatif:

```http
X-RateLimit-Limit: 100
X-RateLimit-Remaining: 45
X-RateLimit-Reset: 1699891260
X-RateLimit-RetryAfter: 15
```

### Header Descriptions
- **X-RateLimit-Limit**: Total requests allowed per period
- **X-RateLimit-Remaining**: Remaining requests dalam period saat ini
- **X-RateLimit-Reset**: Unix timestamp saat limit direset
- **X-RateLimit-RetryAfter**: Seconds until next request allowed (saat limited)

## Multiple Identifiers dengan Rate Limiting

```yaml
identifiers:
  # Premium users - high limits
  - type: "Header"
    name: "X-API-Key"
    value: "premium-key"
    rate_limit:
      enabled: true
      rate: 1000
      burst: 500
      period: "1h"
  
  # Basic users - standard limits  
  - type: "Header"
    name: "X-API-Key"
    value: "basic-key"
    rate_limit:
      enabled: true
      rate: 100
      burst: 50
      period: "1h"
  
  # Anonymous users - restrictive limits
  - type: "IP"
    name: ""
    value: "anonymous"
    rate_limit:
      enabled: true
      rate: 10
      burst: 5
      period: "1h"
```

## Disable Rate Limiting

Untuk disable rate limiting tetapi tetap menggunakan quota:

```yaml
rate_limit:
  enabled: false    # Rate limiting disabled
quota:
  enabled: true     # But quota still active
  limit: 1000
  period: "Daily"
```

## Rate Limiting Strategies

### 1. Progressive Rate Limiting
```yaml
# Mulai permissive, kemudian restrictive
# Stage 1: Loose limits
rate: 1000
burst: 2000
period: "1h"

# Stage 2: Tighter limits  
rate: 500
burst: 100
period: "1h"

# Stage 3: Strict limits
rate: 100
burst: 20
period: "1h"
```

### 2. Service-Based Rate Limiting
```yaml
# Different limits per service type
# Search API - frequent usage expected
rate: 300
burst: 100
period: "1m"

# User Profile API - moderate usage
rate: 100  
burst: 50
period: "1m"

# Admin API - restricted usage
rate: 10
burst: 5
period: "1m"
```

### 3. Time-Based Rate Limiting
```yaml
# Business hours - higher limits
# Peak hours: 9 AM - 5 PM
rate: 1000
burst: 500
period: "1h"

# Off hours: 6 PM - 8 AM  
rate: 100
burst: 50
period: "1h"
```

## Common Rate Limit Patterns

### Web API Rate Limiting
```yaml
rate: 100      # 100 requests/minute
burst: 20      # Allow burst of 20
period: "1m"   # Per minute window
```

### Mobile App Rate Limiting  
```yaml
rate: 300      # 300 requests/hour (5/minute average)
burst: 60      # Allow burst for app startup
period: "1h"   # Hourly window
```

### Webhook Rate Limiting
```yaml
rate: 10       # 10 webhooks/minute
burst: 5       # Small burst allowance
period: "1m"   # Quick recovery
```

## Testing Rate Limits

### Manual Testing
```bash
#!/bin/bash
# Test rate limiting behavior

API_KEY="test-key"
ENDPOINT="http://localhost/api"

echo "Testing rate limit..."
for i in {1..25}; do
  response=$(curl -s -w ",%{http_code}" -H "X-API-Key: $API_KEY" "$ENDPOINT")
  echo "Request $i: $response"
  sleep 0.1
done
```

### Expected Behavior
```
Request 1: {"message":"success"},200
Request 2: {"message":"success"},200
...
Request 10: {"message":"success"},200
Request 11: {"error":"Rate limit exceeded"},429
Request 12: {"error":"Rate limit exceeded"},429
...
```

## Redis Storage

Rate limiting data disimpan di Redis dengan key pattern:
```
rate_limit:{identifier}:tokens = "45.5"
rate_limit:{identifier}:last_refill = "1699891200123456789"
TTL = 120 seconds (2x period untuk safety)
```

## Performance Considerations

### Redis Operations per Request
- 2 Redis operations: GET (bucket state) + SET (update state)
- Atomic operations untuk thread safety
- TTL auto-cleanup untuk memory efficiency

### Memory Usage
- Minimal memory per bucket: ~100 bytes
- TTL ensures automatic cleanup
- No memory leaks dengan proper TTL

---

Rate limiting dengan Token Bucket provides smooth traffic control dengan burst tolerance, ideal untuk production API protection.