# Traefik Quota Plugin

A Traefik middleware plugin that provides **exact-match rate limiting and quota management** with multiple identifier support and Redis persistence.

## Current Features

- ✅ **Exact Identifier Matching**: Only matches when header/cookie values exactly equal configuration values
- ✅ **Multiple User Support**: Different limits per specific user identifier
- ✅ **Flexible Enable/Disable**: Rate limiting and quota can be independently enabled/disabled
- ✅ **Redis Persistence**: Simple Redis client implementation without external dependencies
- ✅ **Custom JSON Responses**: Configurable HTTP status codes and JSON response bodies
- ✅ **Content-Type Detection**: Automatic JSON vs text/plain content type setting
- ✅ **No Identifier = 403**: Blocks requests without valid identifiers

## Current Behavior

### Identifier Matching Logic

The plugin matches identifiers using **exact value comparison**:

1. **Exact Match**: Header value must exactly equal configured value
2. **No Fallback Chain**: Each identifier is independent
3. **First Match Wins**: Plugin uses first matching identifier and stops
4. **No Match = 403**: Returns 403 Forbidden if no identifier matches

### Request Flow Examples

#### 1. Valid User Request
```bash
curl -H "X-User-ID: sk-didingateng" http://chat.localhost/
```
**Result**: Uses `sk-didingateng` identifier with 10 req/min rate limit + 500/month quota

#### 2. Different Valid User
```bash
curl -H "X-User-ID: sk-aloyganteng" http://chat.localhost/
```
**Result**: Uses `sk-aloyganteng` identifier with 5 req/min rate limit + 200/month quota

#### 3. Unknown User Value
```bash
curl -H "X-User-ID: some-random-value" http://chat.localhost/
```
**Result**: 403 Forbidden - No matching identifier found

#### 4. No Header
```bash
curl http://chat.localhost/
```
**Result**: 403 Forbidden - No valid identifier found

#### 5. Fallback for Empty Header (Special Case)
For identifiers with `Value: "sk-unknown"` and no header present:
```bash
curl http://chat.localhost/  # No X-User-ID header
```
**Result**: Uses `sk-unknown` fallback identifier (if configured)

## Configuration Structure

### Complete Example
```yaml
middlewares:
  chat-quota:
    plugin:
      quota:
        Persistence: 
          Redis:
            Address: "redis:6379"
            Password: ""
            DB: 0
        Identifiers: 
          # Premium user
          - Type: "Header"
            Name: "X-User-ID"
            Value: "sk-didingateng"
            RateLimit:
              Enabled: true
              Rate: 10
              Burst: 20
              Period: "1m"
              ResponseReachedLimitCode: 429
              ResponseReachedLimitBody: |
                {
                  "error": "Premium rate limit exceeded",
                  "user": "sk-didingateng"
                }
            Quota:
              Enabled: true
              Limit: 500
              Period: "Monthly"
              ResponseReachedLimitCode: 402
              ResponseReachedLimitBody: |
                {
                  "error": "Premium quota exceeded",
                  "user": "sk-didingateng"
                }
          
          # Rate limit only user
          - Type: "Header"
            Name: "X-User-ID"
            Value: "sk-testuser"
            RateLimit:
              Enabled: true
              Rate: 2
              Burst: 5
              Period: "1m"
              ResponseReachedLimitCode: 429
              ResponseReachedLimitBody: |
                {
                  "error": "Test rate limit exceeded",
                  "user": "sk-testuser"
                }
            Quota:
              Enabled: false  # No quota for this user
```

### Configuration Parameters

#### Identifier Config
- **Type**: `"Header"`, `"Cookie"`, `"IP"`, `"Query"`
- **Name**: Header/Cookie/Query parameter name (empty for IP)
- **Value**: Exact value to match (used as fallback for some types)

#### Rate Limit Config
- **Enabled**: `true`/`false` - Enable/disable rate limiting
- **Rate**: Requests per period (ignored if Enabled=false)
- **Burst**: Maximum burst capacity
- **Period**: Time period (`"1s"`, `"1m"`, `"1h"`, `"1d"`)
- **ResponseReachedLimitCode**: HTTP status code (e.g., 429)
- **ResponseReachedLimitBody**: JSON/text response body

#### Quota Config
- **Enabled**: `true`/`false` - Enable/disable quota
- **Limit**: Maximum requests per period (ignored if Enabled=false)
- **Period**: `"Daily"`, `"Weekly"`, `"Monthly"`
- **ResponseReachedLimitCode**: HTTP status code (e.g., 403)
- **ResponseReachedLimitBody**: JSON/text response body

## Current Implementation Details

### Validation Rules
1. **At least one feature must be enabled**: Either RateLimit.Enabled=true OR Quota.Enabled=true
2. **Positive values required**: Rate, Burst, Limit must be > 0 when enabled
3. **Valid periods**: Rate limit periods use duration format, quota periods use preset values

### Redis Key Structure
```
# Rate limiting keys
rate_limit:header:X-User-ID:sk-didingateng:1699123200

# Quota keys  
quota:header:X-User-ID:sk-didingateng:monthly:2023-11-01
```

### Response Headers
```
X-RateLimit-Limit: 10
X-RateLimit-Remaining: 7
X-RateLimit-Reset: 1699123260
X-Quota-Limit: 500
X-Quota-Used: 45
X-Quota-Remaining: 455
X-Quota-Reset: 1701388800
```

### Error Responses

#### No Identifier Found (403)
```json
{
  "error": "Access denied",
  "message": "No valid identifier found in request"
}
```

#### Rate Limit Exceeded (429)
```json
{
  "error": "Premium rate limit exceeded",
  "user": "sk-didingateng",
  "retry_after": 60
}
```

#### Quota Exceeded (402/403)
```json
{
  "error": "Premium quota exceeded",
  "user": "sk-didingateng",
  "contact": "billing@example.com"
}
```

## Supported Identifier Types

### 1. Header-based
```yaml
- Type: "Header"
  Name: "X-User-ID"
  Value: "specific-user-id"
```
**Matches**: Only when `X-User-ID: specific-user-id` header is present

### 2. Cookie-based
```yaml
- Type: "Cookie"
  Name: "session_id"
  Value: "expected-session-value"
```
**Matches**: Only when cookie value exactly equals expected value

### 3. IP-based
```yaml
- Type: "IP"
  Name: ""
  Value: "unknown"
```
**Matches**: Always returns client IP (from headers or RemoteAddr)

### 4. Query Parameter
```yaml
- Type: "Query"
  Name: "api_key"
  Value: "expected-key-value"
```
**Matches**: Only when `?api_key=expected-key-value` parameter matches

## Current Limitations

1. **No True Fallback Chain**: Each identifier is independent, no priority-based fallback
2. **Exact Match Only**: No pattern matching or wildcards
3. **Single Match**: Plugin stops at first matching identifier
4. **No Authentication Integration**: Manual identifier management required

## Use Cases

### 1. Multiple User Tiers
Configure different limits for premium, standard, and free users with exact user ID matching.

### 2. Service-Specific Limits
Use different middleware configurations for different API endpoints.

### 3. Testing Users
Create special identifiers with different limits for testing purposes.

### 4. Emergency Rate Limiting
Quickly add rate limiting to specific problematic users by their exact identifier.

## Deployment

### Docker Compose
```yaml
version: '3.8'
services:
  traefik:
    image: traefik:v3.0
    command:
      - --experimental.plugins.quota.modulename=github.com/your-org/traefik-quota-plugin
      - --experimental.plugins.quota.version=v1.0.0
    volumes:
      - ./traefik:/etc/traefik:ro
      
  redis:
    image: redis:7-alpine
    ports:
      - "6379:6379"
```

### File Structure
```
traefik/
├── traefik.yaml      # Main Traefik config
├── dynamic.yaml      # Plugin middleware config
```

## Monitoring

### Log Messages
```
Initialized manager for identifier Header:X-User-ID:sk-didingateng (rate: 10/1m, quota: 500/Monthly)
Checking identifier: Header:X-User-ID:sk-didingateng
Header matches! Returning: sk-didingateng
Identifier matched: Header:X-User-ID:sk-didingateng (allowed: true)
Request allowed for identifier: sk-didingateng
```

### Debug Information
Enable detailed logging to see identifier matching process and Redis operations.

## Performance

- **Simple Redis Protocol**: No external dependencies
- **Efficient Matching**: First-match wins, no unnecessary processing
- **Cached Connections**: Redis connection pooling
- **Memory Efficient**: Minimal memory footprint per request

---

**Note**: This implementation focuses on exact identifier matching rather than fallback chains. For true fallback behavior, consider using different middleware configurations or IP-based identifiers as the final fallback.