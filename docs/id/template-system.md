# Template System

Template system memungkinkan pembuatan identifier dinamis dengan menggabungkan data dari HTTP headers, query parameters, cookies, dan metadata request lainnya.

## Template Delimiters

Template system menggunakan delimiter khusus `[[` dan `]]` untuk menghindari konflik dengan template system lain:

```yaml
# ✅ Correct - Custom delimiters
value: "[[.Headers.X-API-Key]]-[[.Query.user_id]]"

# ❌ Wrong - Standard Go template delimiters  
value: "{{.Headers.X-API-Key}}-{{.Query.user_id}}"
```

## Available Template Data

### Request Metadata
```yaml
[[.IP]]        # Client IP address
[[.Method]]    # HTTP method (GET, POST, etc.)
[[.Path]]      # Request path
[[.Scheme]]    # http or https
[[.Host]]      # Host header
```

### Headers
```yaml
[[.Headers.Authorization]]     # Authorization header
[[.Headers.X-API-Key]]        # Custom API key header
[[.Headers.User-Agent]]       # User agent
[[.Headers.X-Forwarded-For]]  # Forwarded IP
[[.Headers.Content-Type]]     # Content type
```

### Query Parameters
```yaml
[[.Query.user_id]]     # ?user_id=123
[[.Query.client_id]]   # ?client_id=abc
[[.Query.version]]     # ?version=v2
[[.Query.api_key]]     # ?api_key=secret
```

### Cookies
```yaml
[[.Cookies.session_id]]    # Session ID cookie
[[.Cookies.auth_token]]    # Authentication token cookie
[[.Cookies.user_pref]]     # User preferences cookie
```

## Template Syntax

### Basic Concatenation
```yaml
# Simple concatenation
value: "[[.Headers.X-API-Key]]-[[.Query.user_id]]"
# Result: "sk-abc123-user456"

# With static prefixes
value: "api:[[.Headers.X-API-Key]]"  
# Result: "api:sk-abc123"
```

### Conditional Logic

#### Simple If Statement
```yaml
value: "[[if .Headers.X-API-Key]]api:[[.Headers.X-API-Key]][[end]]"
# Result: "api:sk-abc123" (if header exists) or "" (if not)
```

#### If-Else Statement
```yaml
value: "[[if .Headers.X-API-Key]]api:[[.Headers.X-API-Key]][[else]]ip:[[.IP]][[end]]"
# Result: "api:sk-abc123" or "ip:192.168.1.100"
```

#### Multiple Conditions
```yaml
value: "[[if .Headers.Authorization]]auth:[[.Headers.Authorization]][[else if .Cookies.session_id]]session:[[.Cookies.session_id]][[else]]ip:[[.IP]][[end]]"
# Fallback chain: Authorization → Session Cookie → IP
```

### Complex Examples

#### Multi-tier Identification
```yaml
value: "[[if .Headers.X-API-Key]]premium:[[.Headers.X-API-Key]][[else if .Query.client_id]]basic:[[.Query.client_id]][[else]]guest:[[.IP]][[end]]"
```

#### Tenant-based Identification  
```yaml
value: "tenant:[[.Headers.X-Tenant-ID]]/user:[[.Headers.X-User-ID]]"
# Result: "tenant:company123/user:john.doe"
```

#### Service Endpoint Identification
```yaml
value: "service:[[.Path]]/method:[[.Method]]"
# Result: "service:/api/v1/users/method:GET"
```

## Configuration Examples

### Basic Template Identifier
```yaml
identifiers:
  - type: "Template"  
    name: "api-key-fallback"
    value: "[[if .Headers.X-API-Key]][[.Headers.X-API-Key]][[else]][[.IP]][[end]]"
    rate_limit:
      enabled: true
      rate: 100
      burst: 10
      period: "1h"
```

### Multi-source Template
```yaml
identifiers:
  - type: "Template"
    name: "multi-source"
    value: "[[if .Headers.Authorization]]auth:[[.Headers.Authorization]][[else if .Cookies.session_id]]session:[[.Cookies.session_id]][[else if .Query.api_key]]query:[[.Query.api_key]][[else]]ip:[[.IP]][[end]]"
    quota:
      enabled: true
      limit: 1000
      period: "Daily"
```

### Tenant-User Combination
```yaml
identifiers:
  - type: "Template"
    name: "tenant-user"
    value: "[[.Headers.X-Tenant-ID]]:[[.Headers.X-User-ID]]"
    rate_limit:
      enabled: true
      rate: 50
      burst: 100
      period: "1m"
    quota:
      enabled: true
      limit: 10000
      period: "Monthly"
```

## Advanced Use Cases

### 1. API Gateway with Multiple Auth Methods
```yaml
# Support JWT, API Key, dan Session-based auth
value: "[[if .Headers.Authorization]]jwt:[[.Headers.Authorization]][[else if .Headers.X-API-Key]]key:[[.Headers.X-API-Key]][[else if .Cookies.session_id]]session:[[.Cookies.session_id]][[else]]anonymous:[[.IP]][[end]]"
```

### 2. Microservice Rate Limiting
```yaml
# Rate limit per service endpoint
value: "service:[[.Headers.X-Service-Name]]/path:[[.Path]]"
```

### 3. User Tier Detection
```yaml
# Detect user tier from multiple sources
value: "[[if .Headers.X-Premium-User]]premium:[[.Headers.X-User-ID]][[else if .Headers.X-User-ID]]basic:[[.Headers.X-User-ID]][[else]]guest:[[.IP]][[end]]"
```

### 4. Geographic Rate Limiting
```yaml
# Combine region with user ID
value: "region:[[.Headers.X-Region]]/user:[[if .Headers.X-User-ID]][[.Headers.X-User-ID]][[else]][[.IP]][[end]]"
```

### 5. Client Application Identification
```yaml
# Identify by client app and version
value: "app:[[.Headers.X-Client-App]]/version:[[.Headers.X-App-Version]]/user:[[if .Query.user_id]][[.Query.user_id]][[else]][[.IP]][[end]]"
```

## Template Best Practices

### 1. Always Provide Fallbacks
```yaml
# ✅ Good - Always has a fallback
value: "[[if .Headers.X-API-Key]][[.Headers.X-API-Key]][[else]][[.IP]][[end]]"

# ❌ Bad - Can result in empty identifier
value: "[[.Headers.X-API-Key]]"
```

### 2. Use Meaningful Prefixes
```yaml
# ✅ Good - Clear identifier types
value: "[[if .Headers.X-API-Key]]api:[[.Headers.X-API-Key]][[else]]guest:[[.IP]][[end]]"

# ❌ Bad - Ambiguous identifiers
value: "[[if .Headers.X-API-Key]][[.Headers.X-API-Key]][[else]][[.IP]][[end]]"
```

### 3. Consider Uniqueness
```yaml
# ✅ Good - Ensures uniqueness across tenants
value: "tenant:[[.Headers.X-Tenant-ID]]/user:[[.Headers.X-User-ID]]"

# ⚠️ Potentially problematic - User IDs might overlap across tenants
value: "[[.Headers.X-User-ID]]"
```

### 4. Handle Missing Data Gracefully
```yaml
# ✅ Good - Handles missing headers
value: "[[if .Headers.X-Tenant-ID]]tenant:[[.Headers.X-Tenant-ID]][[else]]default[[end]]/[[if .Headers.X-User-ID]]user:[[.Headers.X-User-ID]][[else]]anonymous[[end]]"
```

### 5. Keep Templates Readable
```yaml
# ✅ Good - Simple and readable
value: "[[if .Headers.X-API-Key]]premium:[[.Headers.X-API-Key]][[else]]basic:[[.IP]][[end]]"

# ❌ Bad - Too complex, hard to debug
value: "[[if .Headers.X-API-Key]][[if .Headers.X-Premium]]premium:[[.Headers.X-API-Key]][[else]]basic:[[.Headers.X-API-Key]][[end]][[else]][[if .Query.guest_token]]guest:[[.Query.guest_token]][[else]]anon:[[.IP]][[end]][[end]]"
```

## Testing Templates

### Manual Testing
```bash
# Test dengan berbagai header combinations
curl -H "X-API-Key: test-key" -H "X-User-ID: user123" http://localhost/

curl -H "X-User-ID: user123" http://localhost/

curl http://localhost/
```

### Template Validation
Template yang salah akan menghasilkan identifier kosong dan di-skip. Check logs untuk error:
```
Template execution failed: template: identifier:1: unexpected "}" in operand
Skipping identifier due to template error
```

## Error Handling

### Template Syntax Errors
- Template dengan syntax error akan di-skip
- Error akan dicatat dalam log
- Plugin akan lanjut ke identifier berikutnya

### Missing Variables
- Variable yang tidak ada akan menghasilkan empty string
- Gunakan conditional logic untuk handle missing data
- Selalu sediakan fallback

### Empty Results
- Jika template menghasilkan string kosong, identifier akan di-skip
- Pastikan ada fallback yang selalu menghasilkan nilai

## Performance Considerations

### Template Caching
- Template di-parse sekali saat plugin startup
- Execution sangat cepat setelah parsing
- Minimal performance overhead

### Complex Templates
- Template yang sangat complex dapat impact performance
- Batasi depth conditional logic
- Prefer simple concatenation when possible

---

Template system memberikan fleksibilitas maksimal untuk identifier management sambil menjaga performa dan reliability.