# Traefik Quota Plugin

A powerful Traefik middleware plugin that provides rate limiting and quota management with multiple identifier support and Redis persistence.

## Features

- **Multiple Identifier Support**: Priority-based identifier matching (Headers, IP, Cookies, Query parameters)
- **Rate Limiting**: Token bucket algorithm with configurable rates, bursts, and periods
- **Quota Management**: Daily, Weekly, Monthly quotas with automatic reset
- **Redis Persistence**: Simple Redis client implementation without external dependencies
- **Custom Responses**: Configurable HTTP status codes and JSON response bodies
- **Fallback Chain**: Support for multiple identifiers with priority-based selection

## How It Works

### Rate Limiting Mechanism

The plugin uses a **Token Bucket Algorithm** for rate limiting:

1. **Token Bucket**: Each identifier gets a bucket with a maximum capacity (burst)
2. **Token Refill**: Tokens are added to the bucket at a configured rate
3. **Request Processing**: Each request consumes one token from the bucket
4. **Rate Limiting**: When bucket is empty, requests are rejected until tokens refill

**Example**: Rate of 100 req/hour with burst of 200 means:
- Bucket starts with 200 tokens
- Tokens refill at ~1.67 tokens/minute (100/hour)
- Can handle 200 immediate requests, then limited to 100/hour

### Quota Mechanism

Quota provides longer-term limits with automatic reset periods:

1. **Quota Tracking**: Counts total requests consumed within a period
2. **Period Types**: Daily (resets at 00:00 UTC), Weekly (Monday 00:00 UTC), Monthly (1st day 00:00 UTC)
3. **Quota Enforcement**: When limit reached, requests are blocked until next reset
4. **Persistence**: Quota counters stored in Redis with expiration

### Identifier Priority System

The plugin checks identifiers in order and uses the **first match**:

1. **Header-based**: `X-API-Key`, `Authorization`, custom headers
2. **Cookie-based**: Session IDs, user tokens
3. **IP-based**: Client IP address (fallback)
4. **Query-based**: URL parameters

### Response Handling

- **Content-Type Detection**: Automatically sets `application/json` for JSON responses, `text/plain` for others
- **Custom Status Codes**: Configure different HTTP codes for rate limit vs quota exceeded
- **Custom Response Bodies**: JSON error messages with contextual information

## Configuration Examples

## Basic Configuration

### Simple Rate Limiting Only
```yaml
middlewares:
  basic-rate-limit:
    plugin:
      quota:
        Persistence:
          Redis:
            Address: "redis:6379"
            DB: 0
        Identifiers:
          - Type: "Header"
            Name: "X-API-Key"
            Value: "anonymous"  # Default value when header is missing
            RateLimit:
              Rate: 100          # 100 requests per period
              Burst: 200         # Allow burst of 200 requests
              Period: "1h"       # Per hour
              ResponseReachedLimitCode: 429
              ResponseReachedLimitBody: |
                {
                  "error": "Rate limit exceeded",
                  "limit": 100,
                  "period": "1 hour",
                  "retry_after": 3600
                }
            # Quota disabled for this example
            Quota:
              Enabled: false
```

### Simple Quota Only
```yaml
middlewares:
  basic-quota:
    plugin:
      quota:
        Persistence:
          Redis:
            Address: "redis:6379"
            DB: 0
        Identifiers:
          - Type: "IP"
            Name: ""
            Value: "unknown"
            # Rate limiting disabled
            RateLimit:
              Rate: 0
            Quota:
              Enabled: true
              Limit: 1000        # 1000 requests per period
              Period: "Daily"    # Resets daily at 00:00 UTC
              ResponseReachedLimitCode: 403
              ResponseReachedLimitBody: |
                {
                  "error": "Daily quota exceeded",
                  "limit": 1000,
                  "reset_time": "00:00 UTC"
                }
```

### Combined Rate Limit + Quota
```yaml
middlewares:
  rate-and-quota:
    plugin:
      quota:
        Persistence:
          Redis:
            Address: "redis:6379"
            DB: 0
        Identifiers:
          - Type: "Header"
            Name: "Authorization"
            Value: "guest"
            RateLimit:
              Rate: 60           # 60 requests/hour (short-term limit)
              Burst: 120         # Allow bursts up to 120
              Period: "1h"
              ResponseReachedLimitCode: 429
              ResponseReachedLimitBody: |
                {
                  "error": "Rate limit exceeded",
                  "message": "Too many requests per hour"
                }
            Quota:
              Enabled: true
              Limit: 10000       # 10,000 requests/month (long-term limit)
              Period: "Monthly"
              ResponseReachedLimitCode: 402
              ResponseReachedLimitBody: |
                {
                  "error": "Monthly quota exceeded",
                  "contact": "billing@example.com"
                }
```

## Advanced Configuration Examples

## 1. Multiple Access Levels (API Key + User ID)
```yaml
middlewares:
  multi-tier-access:
    plugin:
      quota:
        Persistence:
          Redis:
            Address: "redis:6379"
            DB: 0
        Identifiers:
          # Premium API Key users
          - Type: "Header"
            Name: "X-API-Key"
            Value: "premium-key"
            RateLimit:
              Rate: 1000
              Burst: 2000
              Period: "1h"
              ResponseReachedLimitCode: 429
              ResponseReachedLimitBody: |
                {
                  "error": "Premium API rate limit exceeded",
                  "limit": 1000,
                  "period": "1 hour",
                  "retry_after": "3600"
                }
            Quota:
              Enabled: true
              Limit: 100000
              Period: "Monthly"
              ResponseReachedLimitCode: 402
              ResponseReachedLimitBody: |
                {
                  "error": "Premium monthly quota exceeded",
                  "contact": "upgrade@example.com"
                }
          
          # Regular users by User ID
          - Type: "Header"
            Name: "X-User-ID"
            Value: "anonymous"
            RateLimit:
              Rate: 60
              Burst: 120
              Period: "1h"
              ResponseReachedLimitCode: 429
              ResponseReachedLimitBody: |
                {
                  "error": "User rate limit exceeded",
                  "message": "Please wait before making more requests"
                }
            Quota:
              Enabled: true
              Limit: 1000
              Period: "Daily"
              ResponseReachedLimitCode: 403
              ResponseReachedLimitBody: |
                {
                  "error": "Daily quota exceeded",
                  "reset_time": "00:00 UTC"
                }
```

## 2. Fallback Chain (Session → User → IP)
```yaml
middlewares:
  fallback-identification:
    plugin:
      quota:
        Persistence:
          Redis:
            Address: "redis:6379"
            DB: 1
        Identifiers:
          # First priority: Session ID
          - Type: "Cookie"
            Name: "session_id"
            Value: ""
            RateLimit:
              Rate: 100
              Burst: 200
              Period: "1h"
              ResponseReachedLimitCode: 429
              ResponseReachedLimitBody: |
                {
                  "error": "Session rate limit exceeded",
                  "session_based": true
                }
            Quota:
              Enabled: true
              Limit: 5000
              Period: "Daily"
              ResponseReachedLimitCode: 403
              ResponseReachedLimitBody: |
                {
                  "error": "Session daily quota exceeded"
                }
          
          # Second priority: User ID
          - Type: "Header"
            Name: "Authorization"
            Value: ""
            RateLimit:
              Rate: 50
              Burst: 100
              Period: "1h"
              ResponseReachedLimitCode: 429
              ResponseReachedLimitBody: |
                {
                  "error": "User rate limit exceeded",
                  "authenticated": true
                }
            Quota:
              Enabled: true
              Limit: 2000
              Period: "Daily"
              ResponseReachedLimitCode: 403
              ResponseReachedLimitBody: |
                {
                  "error": "User daily quota exceeded"
                }
          
          # Fallback: IP Address
          - Type: "IP"
            Name: ""
            Value: "unknown"
            RateLimit:
              Rate: 10
              Burst: 20
              Period: "1h"
              ResponseReachedLimitCode: 429
              ResponseReachedLimitBody: |
                {
                  "error": "IP rate limit exceeded",
                  "anonymous": true,
                  "message": "Please authenticate for higher limits"
                }
            Quota:
              Enabled: true
              Limit: 100
              Period: "Daily"
              ResponseReachedLimitCode: 403
              ResponseReachedLimitBody: |
                {
                  "error": "Anonymous IP daily quota exceeded",
                  "solution": "Please create an account"
                }
```

## 3. Service-Specific Limits
```yaml
# For different API endpoints with different limits
middlewares:
  api-v1-limits:
    plugin:
      quota:
        Persistence:
          Redis:
            Address: "redis:6379"
            DB: 2
        Identifiers:
          - Type: "Header"
            Name: "X-API-Key"
            Value: "free-tier"
            RateLimit:
              Rate: 10
              Burst: 20
              Period: "1m"
              ResponseReachedLimitCode: 429
              ResponseReachedLimitBody: |
                {
                  "error": "API rate limit exceeded",
                  "service": "api-v1",
                  "upgrade_url": "https://example.com/upgrade"
                }
            Quota:
              Enabled: true
              Limit: 1000
              Period: "Monthly"
              ResponseReachedLimitCode: 402
              ResponseReachedLimitBody: |
                {
                  "error": "Monthly API quota exceeded",
                  "service": "api-v1",
                  "current_plan": "free",
                  "upgrade_required": true
                }

  webhook-limits:
    plugin:
      quota:
        Persistence:
          Redis:
            Address: "redis:6379"
            DB: 2
        Identifiers:
          - Type: "Header"
            Name: "X-Webhook-Secret"
            Value: "invalid"
            RateLimit:
              Rate: 100
              Burst: 500
              Period: "1m"
              ResponseReachedLimitCode: 429
              ResponseReachedLimitBody: |
                {
                  "error": "Webhook rate limit exceeded",
                  "service": "webhooks"
                }
            Quota:
              Enabled: true
              Limit: 50000
              Period: "Monthly"
              ResponseReachedLimitCode: 403
              ResponseReachedLimitBody: |
                {
                  "error": "Webhook quota exceeded",
                  "contact": "support@example.com"
                }
```

## 4. Geographic/Regional Limits
```yaml
middlewares:
  regional-limits:
    plugin:
      quota:
        Persistence:
          Redis:
            Address: "redis:6379"
            DB: 3
        Identifiers:
          # High-capacity regions
          - Type: "Header"
            Name: "X-Region"
            Value: "us-east"
            RateLimit:
              Rate: 1000
              Burst: 2000
              Period: "1m"
              ResponseReachedLimitCode: 429
              ResponseReachedLimitBody: |
                {
                  "error": "Regional rate limit exceeded",
                  "region": "us-east",
                  "capacity": "high"
                }
            Quota:
              Enabled: true
              Limit: 1000000
              Period: "Monthly"
              ResponseReachedLimitCode: 503
              ResponseReachedLimitBody: |
                {
                  "error": "Regional capacity exceeded",
                  "region": "us-east",
                  "try_region": "us-west"
                }
          
          # Lower-capacity regions
          - Type: "Header"
            Name: "X-Region"
            Value: "ap-southeast"
            RateLimit:
              Rate: 100
              Burst: 200
              Period: "1m"
              ResponseReachedLimitCode: 429
              ResponseReachedLimitBody: |
                {
                  "error": "Regional rate limit exceeded",
                  "region": "ap-southeast",
                  "capacity": "limited"
                }
            Quota:
              Enabled: true
              Limit: 100000
              Period: "Monthly"
              ResponseReachedLimitCode: 503
              ResponseReachedLimitBody: |
                {
                  "error": "Regional quota exceeded",
                  "region": "ap-southeast"
                }
```

## 5. Complete Router Configuration with Multiple Services

```yaml
http:
  routers:
    # API v1 with strict limits
    api-v1:
      rule: "Host(`api.example.com`) && PathPrefix(`/v1/`)"
      middlewares:
        - api-v1-limits
      service: api-v1-backend
    
    # API v2 with higher limits
    api-v2:
      rule: "Host(`api.example.com`) && PathPrefix(`/v2/`)"
      middlewares:
        - api-v2-limits
      service: api-v2-backend
    
    # Webhook endpoint
    webhooks:
      rule: "Host(`webhook.example.com`)"
      middlewares:
        - webhook-limits
      service: webhook-backend

  middlewares:
    api-v1-limits:
      plugin:
        quota:
          Persistence:
            Redis:
              Address: "redis-cluster:6379"
              Password: "secure-password"
              DB: 0
          Identifiers:
            - Type: "Header"
              Name: "X-API-Key"
              Value: "unauthenticated"
              RateLimit:
                Rate: 60
                Burst: 120
                Period: "1h"
                ResponseReachedLimitCode: 429
                ResponseReachedLimitBody: |
                  {
                    "error": "API v1 rate limit exceeded",
                    "version": "1.0",
                    "retry_after_seconds": 3600
                  }
              Quota:
                Enabled: true
                Limit: 10000
                Period: "Monthly"
                ResponseReachedLimitCode: 402
                ResponseReachedLimitBody: |
                  {
                    "error": "API v1 monthly quota exceeded",
                    "version": "1.0",
                    "upgrade_to": "v2",
                    "contact": "billing@example.com"
                  }

    api-v2-limits:
      plugin:
        quota:
          Persistence:
            Redis:
              Address: "redis-cluster:6379"
              Password: "secure-password"
              DB: 1
          Identifiers:
            - Type: "Header"
              Name: "Authorization"
              Value: "guest"
              RateLimit:
                Rate: 1000
                Burst: 2000
                Period: "1h"
                ResponseReachedLimitCode: 429
                ResponseReachedLimitBody: |
                  {
                    "error": "API v2 rate limit exceeded",
                    "version": "2.0",
                    "retry_after_seconds": 3600
                  }
              Quota:
                Enabled: true
                Limit: 100000
                Period: "Monthly"
                ResponseReachedLimitCode: 402
                ResponseReachedLimitBody: |
                  {
                    "error": "API v2 monthly quota exceeded",
                    "version": "2.0",
                    "contact": "enterprise@example.com"
                  }

  services:
    api-v1-backend:
      loadBalancer:
        servers:
          - url: "http://api-v1:8080"
    
    api-v2-backend:
      loadBalancer:
        servers:
          - url: "http://api-v2:8080"
    
    webhook-backend:
      loadBalancer:
        servers:
          - url: "http://webhook-service:8080"
```

```

## Configuration Reference

### Persistence Settings
```yaml
Persistence:
  Redis:
    Address: "redis:6379"    # Redis server address
    Password: ""             # Redis password (optional)
    DB: 0                    # Redis database number
```

### Identifier Configuration
```yaml
Identifiers:
  - Type: "Header|Cookie|IP|Query"  # Identifier type
    Name: "header-name"             # Header/Cookie/Query name (empty for IP)
    Value: "default-value"          # Default when identifier missing
    RateLimit:                      # Rate limiting settings
      Rate: 100                     # Requests per period (0 = disabled)
      Burst: 200                    # Maximum burst capacity
      Period: "1m|1h|1d"           # Time period
      ResponseReachedLimitCode: 429 # HTTP status code
      ResponseReachedLimitBody: ""  # Response body (JSON or text)
    Quota:                          # Quota settings
      Enabled: true                 # Enable/disable quota
      Limit: 10000                  # Maximum requests per period
      Period: "Daily|Weekly|Monthly" # Reset period
      ResponseReachedLimitCode: 403 # HTTP status code
      ResponseReachedLimitBody: ""  # Response body (JSON or text)
```

### Period Formats
- **Rate Limit Periods**: `1s`, `30s`, `1m`, `5m`, `1h`, `24h`, `1d`
- **Quota Periods**: `Daily`, `Weekly`, `Monthly`

### Identifier Types
- **Header**: HTTP headers (`X-API-Key`, `Authorization`, etc.)
- **Cookie**: HTTP cookies (`session_id`, `user_token`, etc.)
- **IP**: Client IP address (no Name needed)
- **Query**: URL query parameters (`api_key`, `token`, etc.)

## Usage Scenarios

### Scenario 1: API Service with Tiered Access
**Use Case**: Different limits for free vs premium users

```yaml
Identifiers:
  # Premium users (higher limits)
  - Type: "Header"
    Name: "X-API-Key" 
    Value: "premium-key"
    RateLimit:
      Rate: 1000
      Period: "1h"
    Quota:
      Limit: 1000000
      Period: "Monthly"
  
  # Free users (lower limits)  
  - Type: "Header"
    Name: "X-API-Key"
    Value: "free-key"
    RateLimit:
      Rate: 100
      Period: "1h" 
    Quota:
      Limit: 10000
      Period: "Monthly"
```

### Scenario 2: Progressive Fallback
**Use Case**: Try authenticated user, fallback to IP-based limiting

```yaml
Identifiers:
  # Authenticated users (by session)
  - Type: "Cookie"
    Name: "session_id"
    Value: ""
    RateLimit:
      Rate: 300
      Period: "1h"
    Quota:
      Limit: 50000
      Period: "Monthly"
      
  # Anonymous users (by IP)
  - Type: "IP" 
    Name: ""
    Value: "anonymous"
    RateLimit:
      Rate: 60
      Period: "1h"
    Quota:
      Limit: 1000
      Period: "Daily"
```

### Scenario 3: Service-Specific Limits
**Use Case**: Different limits for different API endpoints

```yaml
# High-volume endpoints
middlewares:
  analytics-limits:
    plugin:
      quota:
        Identifiers:
          - Type: "Header"
            Name: "X-Service"
            Value: "analytics"
            RateLimit:
              Rate: 1000
              Period: "1m"

# Low-volume endpoints  
middlewares:
  admin-limits:
    plugin:
      quota:
        Identifiers:
          - Type: "Header"
            Name: "X-Admin-Token"
            Value: "unauthorized"
            RateLimit:
              Rate: 10
              Period: "1m"
```

## Monitoring and Debugging

### Response Headers
The plugin adds custom headers to help with monitoring:

```
X-RateLimit-Limit: 100
X-RateLimit-Remaining: 85
X-RateLimit-Reset: 1635724800
X-Quota-Limit: 10000
X-Quota-Remaining: 8500
X-Quota-Reset: 1635811200
```

### Log Messages
```
Request allowed: rate_limit_ok, quota_ok (identifier: user123, type: Header)
Request blocked: Rate limit exceeded (identifier: user456, type: IP)
Request blocked: Quota exceeded (identifier: api-key-789, type: Header)
```

### Redis Keys
```
rate_limit:header:X-API-Key:premium-key:1635724800
quota:header:X-API-Key:premium-key:daily:2023-11-04
```

## Best Practices

1. **Order Identifiers by Priority**: Most specific first, most general last
2. **Use Appropriate Periods**: Short periods for rate limits, longer for quotas
3. **Set Meaningful Default Values**: Help identify unclassified traffic
4. **Provide Clear Error Messages**: Include retry information and contact details
5. **Monitor Redis Usage**: Use different DB numbers for different services
6. **Test Fallback Logic**: Ensure anonymous users get reasonable limits

## Deployment with Docker Compose

```yaml
version: '3.8'
services:
  traefik:
    image: traefik:v3.0
    command:
      - --api.insecure=true
      - --providers.docker=true
      - --entrypoints.web.address=:80
      - --experimental.plugins.quota.modulename=github.com/your-org/traefik-quota-plugin
      - --experimental.plugins.quota.version=v1.0.0
    ports:
      - "80:80"
      - "8080:8080"
    volumes:
      - /var/run/docker.sock:/var/run/docker.sock
      
  redis:
    image: redis:7-alpine
    ports:
      - "6379:6379"
      
  app:
    image: your-app:latest
    labels:
      - "traefik.enable=true"
      - "traefik.http.routers.app.rule=Host(`app.localhost`)"
      - "traefik.http.routers.app.middlewares=quota-limits"
      - "traefik.http.middlewares.quota-limits.plugin.quota.Persistence.Redis.Address=redis:6379"
      - "traefik.http.middlewares.quota-limits.plugin.quota.Identifiers[0].Type=Header"
      - "traefik.http.middlewares.quota-limits.plugin.quota.Identifiers[0].Name=X-API-Key"
      - "traefik.http.middlewares.quota-limits.plugin.quota.Identifiers[0].RateLimit.Rate=100"
```

## Troubleshooting

### Common Issues

1. **Redis Connection Failed**
   - Check Redis address and port
   - Verify Redis is running and accessible
   - Check firewall rules

2. **Rate Limits Not Working**
   - Verify identifier matching (check logs)
   - Ensure Rate > 0 in configuration
   - Check Redis key expiration

3. **Content-Type Issues**  
   - Plugin automatically detects JSON vs text responses
   - JSON responses (containing `{` and `}`) get `application/json`
   - Text responses get `text/plain`

4. **Identifier Not Matching**
   - Check header/cookie names (case-sensitive)
   - Verify default values are set
   - Review identifier priority order

### Debug Mode
Enable debug logging in Traefik:
```yaml
log:
  level: DEBUG
```

## Key Features:

1. **Priority-based Selection**: First matching identifier is used
2. **Custom Response Messages**: JSON error responses with contextual information
3. **Different Limits per Identifier**: Each identifier can have unique rate limits and quotas
4. **Flexible Identification**: Headers, IP, Cookies, Query parameters
5. **Redis Separation**: Different DB numbers for different services
6. **Graceful Degradation**: Fallback values when identifiers are missing