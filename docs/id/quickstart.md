# Quick Start Guide

Panduan cepat untuk memulai menggunakan Traefik Quota Plugin dalam 10 menit.

## Prerequisites

- ✅ Docker dan Docker Compose terinstall
- ✅ Akses internet untuk download plugin
- ✅ Text editor (VS Code, vim, dll)

## Step 1: Setup Environment

### 1.1 Buat direktori project
```bash
mkdir traefik-quota-demo
cd traefik-quota-demo
```

### 1.2 Buat struktur folder
```bash
mkdir -p traefik config
```

## Step 2: Configuration Files

### 2.1 Docker Compose
Buat `docker-compose.yml`:

```yaml
version: '3.8'
services:
  traefik:
    image: traefik:v3.0
    command:
      - --api.insecure=true
      - --providers.docker=true
      - --entrypoints.web.address=:80
      - --experimental.plugins.quota.modulename=github.com/hukumonline-com/traefik-quota-plugin
      - --experimental.plugins.quota.version=v1.0.0
      - --providers.file.directory=/etc/traefik
      - --providers.file.watch=true
      - --log.level=INFO
    ports:
      - "80:80"
      - "8080:8080"  # Traefik dashboard
    volumes:
      - /var/run/docker.sock:/var/run/docker.sock:ro
      - ./traefik:/etc/traefik:ro
    depends_on:
      - redis
      - demo-app

  redis:
    image: redis:7-alpine
    ports:
      - "6379:6379"

  demo-app:
    image: nginx:alpine
    labels:
      - "traefik.enable=true"
      - "traefik.http.routers.demo.rule=PathPrefix(`/`)"
      - "traefik.http.routers.demo.middlewares=api-quota"
    volumes:
      - ./config/index.html:/usr/share/nginx/html/index.html
```

### 2.2 Demo App Content
Buat `config/index.html`:

```html
<!DOCTYPE html>
<html>
<head>
    <title>Quota Plugin Demo</title>
    <style>
        body { font-family: Arial, sans-serif; margin: 40px; }
        .header { background: #f0f0f0; padding: 20px; border-radius: 5px; }
        .info { margin: 20px 0; }
        .command { background: #2d3748; color: #e2e8f0; padding: 15px; border-radius: 5px; font-family: monospace; }
    </style>
</head>
<body>
    <div class="header">
        <h1>🎯 Traefik Quota Plugin Demo</h1>
        <p>This page is protected by quota and rate limiting!</p>
    </div>
    
    <div class="info">
        <h2>Test Commands:</h2>
        <div class="command">
# Test with API key (allowed)<br>
curl -H "X-API-Key: demo-key" http://localhost/<br><br>

# Test without API key (forbidden)<br>
curl http://localhost/<br><br>

# Test rate limiting (send multiple requests)<br>
for i in {1..15}; do curl -H "X-API-Key: demo-key" http://localhost/; sleep 1; done
        </div>
    </div>
    
    <div class="info">
        <h2>Check Headers:</h2>
        <div class="command">
curl -I -H "X-API-Key: demo-key" http://localhost/
        </div>
    </div>
</body>
</html>
```

### 2.3 Traefik Plugin Configuration
Buat `traefik/dynamic.yaml`:

```yaml
http:
  middlewares:
    api-quota:
      plugin:
        quota:
          Persistence:
            Redis:
              Address: "redis:6379"
              Password: ""
              DB: 0
          Identifiers:
            # Demo user dengan rate limit dan quota
            - Type: "Header"
              Name: "X-API-Key"
              Value: "demo-key"
              RateLimit:
                Enabled: true
                Rate: 10              # 10 requests per minute
                Burst: 5              # Allow burst of 5
                Period: "1m"
                ResponseReachedLimitCode: 429
                ResponseReachedLimitBody: |
                  {
                    "error": "Demo rate limit exceeded",
                    "message": "You can only make 10 requests per minute",
                    "retry_after": 60,
                    "demo": true
                  }
              Quota:
                Enabled: true
                Limit: 100            # 100 requests per day
                Period: "Daily"
                ResponseReachedLimitCode: 403
                ResponseReachedLimitBody: |
                  {
                    "error": "Daily quota exceeded",
                    "message": "You have used all 100 daily requests",
                    "reset_time": "Midnight UTC",
                    "demo": true,
                    "upgrade": "Contact admin for higher limits"
                  }
            
            # Premium user dengan limit lebih tinggi
            - Type: "Header"
              Name: "X-API-Key"
              Value: "premium-key"
              RateLimit:
                Enabled: true
                Rate: 100             # 100 requests per minute
                Burst: 50
                Period: "1m"
                ResponseReachedLimitCode: 429
                ResponseReachedLimitBody: |
                  {
                    "error": "Premium rate limit exceeded",
                    "tier": "premium",
                    "limit": 100
                  }
              Quota:
                Enabled: true
                Limit: 10000          # 10k requests per month
                Period: "Monthly"
                ResponseReachedLimitCode: 403
                ResponseReachedLimitBody: |
                  {
                    "error": "Monthly quota exceeded",
                    "tier": "premium",
                    "limit": 10000,
                    "contact": "billing@example.com"
                  }
```

## Step 3: Launch Services

### 3.1 Start services
```bash
docker-compose up -d
```

### 3.2 Verify services are running
```bash
docker-compose ps
```

Expected output:
```
NAME                   COMMAND                  SERVICE             STATUS
demo-app              "/docker-entrypoint.…"   demo-app            Up
redis                 "docker-entrypoint.s…"   redis               Up  
traefik               "/entrypoint.sh --ap…"   traefik             Up
```

### 3.3 Check Traefik dashboard
Open browser: http://localhost:8080

Verify:
- ✅ Middleware `api-quota` terlihat di dashboard
- ✅ Router `demo` terhubung ke middleware
- ✅ Plugin `quota` terload

## Step 4: Test Plugin Functionality

### 4.1 Test Basic Access
```bash
# Test without API key - should return 403
curl -v http://localhost/

# Expected response:
# HTTP/1.1 403 Forbidden
# {"error": "Access denied", "message": "No valid identifier found in request"}
```

### 4.2 Test Valid API Key
```bash
# Test with valid API key - should return 200
curl -H "X-API-Key: demo-key" http://localhost/

# Expected: HTML page with demo content
```

### 4.3 Test Response Headers
```bash
# Check quota and rate limit headers
curl -I -H "X-API-Key: demo-key" http://localhost/
```

Expected headers:
```http
HTTP/1.1 200 OK
X-RateLimit-Limit: 10
X-RateLimit-Remaining: 9  
X-RateLimit-Reset: 1699891320
X-Quota-Limit: 100
X-Quota-Used: 1
X-Quota-Remaining: 99
X-Quota-Reset: 1699920000
```

### 4.4 Test Rate Limiting
```bash
# Send rapid requests to trigger rate limit
for i in {1..12}; do
  echo "Request $i:"
  curl -s -w "HTTP %{http_code}\n" -H "X-API-Key: demo-key" http://localhost/ | grep -E "HTTP|error"
  sleep 1
done
```

Expected output:
```
Request 1: HTTP 200
Request 2: HTTP 200
...
Request 10: HTTP 200
Request 11: HTTP 429
Request 12: HTTP 429
```

### 4.5 Test Premium User
```bash
# Test premium key dengan higher limits
curl -I -H "X-API-Key: premium-key" http://localhost/
```

Expected headers with higher limits:
```http
X-RateLimit-Limit: 100
X-Quota-Limit: 10000
```

## Step 5: Monitor dengan Redis

### 5.1 Connect ke Redis
```bash
docker exec -it $(docker-compose ps -q redis) redis-cli
```

### 5.2 Inspect Rate Limiting Data
```redis
# List all quota plugin keys
KEYS rate_limit:*
KEYS quota:*

# Check specific user's rate limit bucket
GET rate_limit:header:X-API-Key:demo-key:tokens
GET rate_limit:header:X-API-Key:demo-key:last_refill

# Check quota usage
GET quota:header:X-API-Key:demo-key:daily:2024-11-15
```

## Step 6: Experiment dengan Configuration

### 6.1 Modify Limits
Edit `traefik/dynamic.yaml` dan ubah rate/quota limits:

```yaml
Rate: 5     # Reduce to 5 requests per minute
Limit: 50   # Reduce to 50 requests per day
```

### 6.2 Reload Configuration
```bash
# Traefik akan auto-reload karena watch=true
# Atau restart jika diperlukan
docker-compose restart traefik
```

### 6.3 Test New Limits
```bash
# Test dengan limit yang lebih ketat
for i in {1..8}; do
  curl -s -w "Request $i: HTTP %{http_code}\n" -H "X-API-Key: demo-key" http://localhost/ | grep "Request"
  sleep 1  
done
```

## Troubleshooting Quick Start

### Plugin tidak terload
```bash
# Check Traefik logs
docker-compose logs traefik | grep -i plugin

# Should see:
# Loading plugin 'quota' from module 'github.com/hukumonline-com/traefik-quota-plugin'
```

### Redis connection issues
```bash
# Test Redis connectivity
docker exec traefik sh -c "nc -z redis 6379 && echo 'Redis OK' || echo 'Redis Failed'"

# Check Redis logs
docker-compose logs redis
```

### Configuration errors
```bash
# Check Traefik logs for configuration errors
docker-compose logs traefik | grep -i error

# Validate YAML syntax
python -c "import yaml; yaml.safe_load(open('traefik/dynamic.yaml'))"
```

## Next Steps

Setelah quick start berhasil:

1. 📖 **Pelajari configuration lebih detail**: [Configuration Guide](./configuration.md)
2. 🎯 **Eksplorasi use cases**: [Use Cases](./use-cases.md)
3. 🔧 **Customize untuk production**: [Best Practices](./best-practices.md)
4. 📊 **Setup monitoring**: [Monitoring Guide](./response-headers.md)

## Clean Up

Untuk menghapus demo environment:

```bash
# Stop dan hapus containers
docker-compose down

# Hapus volumes (opsional)
docker-compose down -v

# Hapus folder project
cd .. && rm -rf traefik-quota-demo
```

---

🎉 **Selamat!** Anda telah berhasil setup dan test Traefik Quota Plugin. Plugin sekarang siap digunakan untuk production dengan konfigurasi yang sesuai kebutuhan Anda.