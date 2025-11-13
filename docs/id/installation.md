# Installation Guide

## System Requirements

- **Traefik**: v2.10+ or v3.0+
- **Go**: 1.19+ (untuk development)
- **Redis**: 6.0+ (opsional, untuk persistence)

## Installation Methods

### Method 1: Plugin Registry (Recommended)

Tambahkan plugin melalui Traefik configuration:

```yaml
# traefik.yaml
experimental:
  plugins:
    quota:
      moduleName: "github.com/hukumonline-com/traefik-quota-plugin"
      version: "v1.0.0"
```

### Method 2: Local Development

1. **Clone repository:**
```bash
git clone https://github.com/hukumonline-com/traefik-quota-plugin.git
cd traefik-quota-plugin
```

2. **Setup Go module:**
```bash
go mod tidy
```

3. **Configure Traefik untuk local plugin:**
```yaml
# traefik.yaml
experimental:
  localPlugins:
    quota:
      moduleName: "github.com/hukumonline-com/traefik-quota-plugin"
```

## Docker Setup

### With Docker Compose

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
    ports:
      - "80:80"
      - "8080:8080"
    volumes:
      - /var/run/docker.sock:/var/run/docker.sock:ro
      - ./traefik:/etc/traefik:ro
    depends_on:
      - redis

  redis:
    image: redis:7-alpine
    ports:
      - "6379:6379"
    volumes:
      - redis_data:/data
    command: redis-server --appendonly yes

volumes:
  redis_data:
```

### Traefik Configuration Files

Buat struktur folder:
```
traefik/
├── traefik.yaml      # Main configuration
├── dynamic.yaml      # Plugin middleware configuration
```

**traefik/traefik.yaml:**
```yaml
global:
  checkNewVersion: false
  sendAnonymousUsage: false

api:
  dashboard: true
  insecure: true

entryPoints:
  web:
    address: ":80"

providers:
  docker:
    exposedByDefault: false
  file:
    directory: /etc/traefik
    watch: true

experimental:
  plugins:
    quota:
      moduleName: "github.com/hukumonline-com/traefik-quota-plugin"
      version: "v1.0.0"
```

**traefik/dynamic.yaml:**
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
            - Type: "Header"
              Name: "X-API-Key"
              Value: "test-key"
              RateLimit:
                Enabled: true
                Rate: 10
                Burst: 20
                Period: "1m"
                ResponseReachedLimitCode: 429
                ResponseReachedLimitBody: |
                  {
                    "error": "Rate limit exceeded",
                    "retry_after": 60
                  }
              Quota:
                Enabled: true
                Limit: 1000
                Period: "Daily"
                ResponseReachedLimitCode: 403
                ResponseReachedLimitBody: |
                  {
                    "error": "Daily quota exceeded",
                    "contact": "support@example.com"
                  }
```

## Kubernetes Setup

### Using Traefik Helm Chart

```yaml
# values.yaml
experimental:
  plugins:
    quota:
      moduleName: "github.com/hukumonline-com/traefik-quota-plugin"
      version: "v1.0.0"

additionalArguments:
  - --experimental.plugins.quota.modulename=github.com/hukumonline-com/traefik-quota-plugin
  - --experimental.plugins.quota.version=v1.0.0
```

### Manual Kubernetes Deployment

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: traefik
spec:
  template:
    spec:
      containers:
      - name: traefik
        image: traefik:v3.0
        args:
          - --experimental.plugins.quota.modulename=github.com/hukumonline-com/traefik-quota-plugin
          - --experimental.plugins.quota.version=v1.0.0
        volumeMounts:
        - name: config
          mountPath: /etc/traefik
      volumes:
      - name: config
        configMap:
          name: traefik-config
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: traefik-config
data:
  traefik.yaml: |
    experimental:
      plugins:
        quota:
          moduleName: "github.com/hukumonline-com/traefik-quota-plugin"
          version: "v1.0.0"
```

## Verification

### 1. Check Plugin Loading

Cek Traefik logs untuk memastikan plugin terload:
```bash
docker logs traefik 2>&1 | grep quota
```

Expected output:
```
Loading plugin 'quota' from module 'github.com/hukumonline-com/traefik-quota-plugin'
Plugin 'quota' loaded successfully
```

### 2. Test Basic Functionality

Test dengan curl:
```bash
# Request tanpa header (should return 403)
curl -v http://localhost/

# Request dengan valid header
curl -H "X-API-Key: test-key" http://localhost/

# Multiple requests untuk test rate limiting
for i in {1..15}; do
  curl -H "X-API-Key: test-key" http://localhost/
  sleep 1
done
```

### 3. Check Response Headers

Verify response headers:
```bash
curl -I -H "X-API-Key: test-key" http://localhost/
```

Expected headers:
```
X-RateLimit-Limit: 10
X-RateLimit-Remaining: 9
X-RateLimit-Reset: 1699891260
X-Quota-Limit: 1000
X-Quota-Used: 1
X-Quota-Remaining: 999
```

## Redis Setup (Optional)

### Redis Configuration

Jika menggunakan Redis untuk persistence:

```yaml
# docker-compose.yaml
redis:
  image: redis:7-alpine
  ports:
    - "6379:6379"
  volumes:
    - redis_data:/data
  command: redis-server --appendonly yes --maxmemory 256mb --maxmemory-policy allkeys-lru
```

### Redis Security

Untuk production, gunakan password:

```yaml
redis:
  image: redis:7-alpine
  environment:
    - REDIS_PASSWORD=your-secure-password
  command: redis-server --requirepass your-secure-password
```

Update konfigurasi plugin:
```yaml
Persistence:
  Redis:
    Address: "redis:6379"
    Password: "your-secure-password"
    DB: 0
```

## Troubleshooting Installation

### Common Issues

1. **Plugin tidak terload:**
   - Verify moduleName dan version benar
   - Check internet connectivity untuk download plugin
   - Restart Traefik setelah konfigurasi

2. **Redis connection error:**
   - Verify Redis service running: `docker ps | grep redis`
   - Test Redis connectivity: `redis-cli -h localhost ping`
   - Check network connectivity antar container

3. **Permission denied errors:**
   - Check Docker socket permissions
   - Verify user memiliki akses ke Docker

### Debug Mode

Enable debug logging:
```yaml
# traefik.yaml
log:
  level: DEBUG

accessLog: {}
```

## Next Steps

Setelah installation berhasil:
1. 📖 Baca [Quick Start Guide](./quickstart.md)
2. ⚙️ Pelajari [Configuration Overview](./configuration.md)  
3. 🎯 Lihat [Use Cases](./use-cases.md) untuk inspiration

---

Jika mengalami masalah, check [Troubleshooting Guide](./troubleshooting.md) atau [FAQ](./faq.md).