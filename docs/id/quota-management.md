# Quota Management

Quota management menyediakan kontrol usage jangka panjang dengan pembatasan total requests per periode waktu tertentu (daily, weekly, monthly). Berbeda dengan rate limiting yang fokus pada throughput per detik/menit, quota management mengontrol volume total usage.

## Konsep Dasar

Quota bekerja sebagai "batas maksimal usage" dalam periode waktu yang lebih panjang:
- **Limit**: Total maksimal requests yang diizinkan
- **Period**: Periode quota (Daily, Weekly, Monthly)  
- **Usage**: Jumlah requests yang sudah digunakan
- **Reset**: Waktu quota direset untuk periode baru

## Konfigurasi Quota

### Basic Configuration
```yaml
identifiers:
  - type: "Header"
    name: "X-API-Key"
    value: "premium-user"
    quota:
      enabled: true
      limit: 10000        # 10k requests per month
      period: "Monthly"   # Monthly quota
      response_reached_limit_code: 402
      response_reached_limit_body: |
        {
          "error": "Monthly quota exceeded",
          "limit": 10000,
          "contact": "billing@example.com"
        }
```

### Parameter Details

#### Enabled
- **Type**: boolean
- **Default**: false
- **Description**: Enable/disable quota management

#### Limit
- **Type**: integer
- **Required**: true (jika enabled=true)
- **Description**: Maksimal requests yang diizinkan per period
- **Example**: `limit: 10000` = 10,000 requests per period

#### Period
- **Type**: string (enum)
- **Required**: true (jika enabled=true)
- **Valid Values**: "Daily", "Weekly", "Monthly"
- **Case Sensitive**: Must be exact case

#### Response Configuration
- **ResponseReachedLimitCode**: HTTP status (default: 403)
- **ResponseReachedLimitBody**: Custom JSON response

## Quota Periods Explained

### Daily Quota
```yaml
quota:
  enabled: true
  limit: 1000
  period: "Daily"    # Reset setiap 00:00 UTC
```

**Reset Schedule**: Setiap hari jam 00:00 UTC
**Use Case**: Daily API limits, daily report generation

### Weekly Quota  
```yaml
quota:
  enabled: true
  limit: 5000
  period: "Weekly"   # Reset setiap Senin 00:00 UTC
```

**Reset Schedule**: Setiap Senin jam 00:00 UTC
**Use Case**: Weekly usage limits, weekly batch processing

### Monthly Quota
```yaml
quota:
  enabled: true
  limit: 50000
  period: "Monthly"  # Reset setiap tanggal 1 jam 00:00 UTC
```

**Reset Schedule**: Tanggal 1 setiap bulan jam 00:00 UTC
**Use Case**: Billing periods, subscription limits

## Quota Configuration Examples

### 1. SaaS API Tiers
```yaml
# Premium Tier
quota:
  enabled: true
  limit: 100000
  period: "Monthly"
  response_reached_limit_code: 402
  response_reached_limit_body: |
    {
      "error": "Premium monthly quota exceeded",
      "tier": "premium",
      "limit": 100000,
      "upgrade_url": "https://billing.example.com/upgrade"
    }

# Basic Tier  
quota:
  enabled: true
  limit: 10000
  period: "Monthly"
  response_reached_limit_code: 402
  response_reached_limit_body: |
    {
      "error": "Basic monthly quota exceeded", 
      "tier": "basic",
      "limit": 10000,
      "upgrade_url": "https://billing.example.com/upgrade"
    }

# Free Tier
quota:
  enabled: true
  limit: 1000
  period: "Monthly"
  response_reached_limit_code: 402
  response_reached_limit_body: |
    {
      "error": "Free tier quota exceeded",
      "tier": "free", 
      "limit": 1000,
      "upgrade_url": "https://billing.example.com/signup"
    }
```

### 2. Daily Usage Limits
```yaml
# Content API - daily quota
quota:
  enabled: true
  limit: 500
  period: "Daily"
  response_reached_limit_code: 403
  response_reached_limit_body: |
    {
      "error": "Daily content quota exceeded",
      "limit": 500,
      "reset_time": "00:00 UTC tomorrow",
      "suggestion": "Try again tomorrow"
    }
```

### 3. Weekly Batch Processing
```yaml
# Weekly report generation
quota:
  enabled: true
  limit: 50
  period: "Weekly"
  response_reached_limit_code: 403
  response_reached_limit_body: |
    {
      "error": "Weekly report quota exceeded",
      "limit": 50,
      "reset_day": "Monday 00:00 UTC",
      "next_reset": "2024-11-18T00:00:00Z"
    }
```

### 4. Development vs Production
```yaml
# Development Environment - lenient quota
quota:
  enabled: true
  limit: 50000
  period: "Daily"

# Production Environment - strict business quota  
quota:
  enabled: true
  limit: 1000000
  period: "Monthly"
```

## Response Headers

Plugin menambahkan headers informatif untuk quota tracking:

```http
X-Quota-Limit: 10000
X-Quota-Used: 2500
X-Quota-Remaining: 7500
X-Quota-Reset: 1701388800
X-Quota-Period: Monthly
```

### Header Descriptions
- **X-Quota-Limit**: Total quota limit untuk period
- **X-Quota-Used**: Jumlah requests yang sudah digunakan
- **X-Quota-Remaining**: Sisa quota yang tersedia
- **X-Quota-Reset**: Unix timestamp saat quota direset
- **X-Quota-Period**: Period type (Daily/Weekly/Monthly)

## Quota Reset Logic

### Daily Reset
```go
// Reset setiap 00:00 UTC
func getDailyQuotaPeriod(now time.Time) string {
    utc := now.UTC()
    return fmt.Sprintf("daily:%04d-%02d-%02d", utc.Year(), utc.Month(), utc.Day())
}
```

### Weekly Reset  
```go
// Reset setiap Senin 00:00 UTC
func getWeeklyQuotaPeriod(now time.Time) string {
    utc := now.UTC()
    year, week := utc.ISOWeek()
    return fmt.Sprintf("weekly:%04d-W%02d", year, week)
}
```

### Monthly Reset
```go
// Reset setiap tanggal 1 jam 00:00 UTC
func getMonthlyQuotaPeriod(now time.Time) string {
    utc := now.UTC()
    return fmt.Sprintf("monthly:%04d-%02d", utc.Year(), utc.Month())
}
```

## Combined Rate Limiting + Quota

```yaml
identifiers:
  - type: "Header"
    name: "X-API-Key"  
    value: "premium-user"
    rate_limit:
      enabled: true
      rate: 100        # 100 requests/minute (short-term)
      burst: 200
      period: "1m"
    quota:
      enabled: true    
      limit: 100000    # 100k requests/month (long-term)
      period: "Monthly"
```

**Behavior**: 
- Short-term protection: Max 100 req/min with burst up to 200
- Long-term protection: Max 100k requests total per month
- Both limits enforced independently

## Quota-Only Configuration

Disable rate limiting, hanya gunakan quota:

```yaml
identifiers:
  - type: "Header"
    name: "X-API-Key"
    value: "batch-processor"
    rate_limit:
      enabled: false    # No rate limiting
    quota:
      enabled: true     # Only quota enforcement
      limit: 10000
      period: "Daily"
```

## Multiple Quota Periods

Gunakan multiple identifiers untuk different quota periods:

```yaml
identifiers:
  # Daily quota untuk user actions
  - type: "Template"
    name: "user-daily"
    value: "daily:[[.Headers.X-User-ID]]"
    quota:
      enabled: true
      limit: 1000
      period: "Daily"
  
  # Monthly quota untuk same user
  - type: "Template"  
    name: "user-monthly"
    value: "monthly:[[.Headers.X-User-ID]]"
    quota:
      enabled: true
      limit: 20000
      period: "Monthly"
```

## Quota Monitoring

### Usage Tracking
```bash
# Check current quota usage via Redis
redis-cli GET "quota:header:X-API-Key:premium-user:monthly:2024-11"
# Returns: "2500" (current usage)
```

### Key Pattern dalam Redis
```
# Quota usage keys
quota:{type}:{name}:{value}:{period}:{time_period} = usage_count

# Examples:
quota:header:X-API-Key:premium-user:monthly:2024-11 = 2500
quota:ip:anonymous:daily:2024-11-15 = 45
quota:template:user123:weekly:2024-W46 = 150
```

### TTL Management
```
# Daily quota - TTL 48 hours (safety margin)
TTL quota:...:daily:2024-11-15 = 172800

# Weekly quota - TTL 2 weeks  
TTL quota:...:weekly:2024-W46 = 1209600

# Monthly quota - TTL 2 months
TTL quota:...:monthly:2024-11 = 5184000
```

## Testing Quota Management

### Manual Testing Script
```bash
#!/bin/bash
API_KEY="test-key"
ENDPOINT="http://localhost/api"

echo "Testing quota management..."
for i in {1..1005}; do
  response=$(curl -s -w ",%{http_code}" -H "X-API-Key: $API_KEY" "$ENDPOINT")
  http_code=$(echo "$response" | cut -d',' -f2)
  
  if [ "$http_code" = "403" ] || [ "$http_code" = "402" ]; then
    echo "Request $i: QUOTA EXCEEDED (HTTP $http_code)"
    break
  else
    echo "Request $i: OK"
  fi
  
  # Check quota headers setiap 100 requests
  if [ $((i % 100)) = 0 ]; then
    quota_remaining=$(curl -s -I -H "X-API-Key: $API_KEY" "$ENDPOINT" | grep "X-Quota-Remaining:" | cut -d' ' -f2)
    echo "Quota remaining: $quota_remaining"
  fi
done
```

## Use Cases

### 1. SaaS Billing Integration
```yaml
# Free tier: 1k/month, Basic: 10k/month, Premium: 100k/month
quota:
  limit: "{{ user_tier_limit }}"
  period: "Monthly"
  response_reached_limit_body: |
    {
      "error": "Quota exceeded",
      "current_tier": "{{ user_tier }}",
      "upgrade_url": "{{ billing_url }}"
    }
```

### 2. Content API Limits
```yaml
# Blog API: 500 posts per day
quota:
  limit: 500
  period: "Daily"
  response_reached_limit_body: |
    {
      "error": "Daily content quota exceeded",
      "suggestion": "Consider premium plan for higher limits"
    }
```

### 3. Data Processing Limits
```yaml  
# ETL Pipeline: 1000 jobs per week
quota:
  limit: 1000
  period: "Weekly"
  response_reached_limit_body: |
    {
      "error": "Weekly processing quota exceeded", 
      "next_reset": "Monday 00:00 UTC"
    }
```

---

Quota management provides long-term usage control essential for business model enforcement dan resource protection.