# Token Bucket Algorithm

Token Bucket adalah algoritma rate limiting yang menggunakan konsep "ember token" untuk mengontrol lalu lintas request. Algoritma ini memungkinkan burst traffic dalam batas tertentu sambil tetap menjaga rata-rata throughput yang dikonfigurasi.

## Konsep Dasar

### Token Bucket Components

```
🪣 TOKEN BUCKET
┌─────────────────┐
│ 🟡🟡🟡🟡🟡    │ ← Tokens (sisa token yang tersedia)
│ ⚪⚪⚪⚪⚪    │
└─────────────────┘
   ↑           ↑
 Rate       Burst
(pengisian) (kapasitas)
```

### Variable Utama

#### **Rate (Kecepatan Pengisian)**
- **Definisi**: Berapa banyak token yang diisi per detik ke dalam ember
- **Contoh**: Rate = 5 artinya setiap detik akan diisi 5 token baru
- **Analogi**: Seperti keran air yang mengalir dengan kecepatan tetap

#### **Burst (Kapasitas Ember)**
- **Definisi**: Maksimal token yang bisa ditampung dalam ember
- **Contoh**: Burst = 10 artinya ember bisa menampung maksimal 10 token
- **Analogi**: Seperti ukuran ember/wadah token

#### **Tokens**
- **Definisi**: Jumlah token yang tersedia saat ini dalam bucket
- **Fungsi**: Berkurang saat ada request, bertambah seiring waktu berdasarkan rate

#### **Last_refill**
- **Definisi**: Timestamp terakhir kali bucket di-refill
- **Fungsi**: Menghitung berapa lama waktu yang telah berlalu untuk menentukan token yang harus ditambahkan

## Algoritma Refill

```go
func (rl *RateLimiter) refillBucket(bucket TokenBucket, now time.Time) TokenBucket {
    // Hitung waktu yang berlalu sejak refill terakhir
    elapsed := now.Sub(bucket.LastRefill)

    // Hitung token yang harus ditambahkan berdasarkan rate
    tokensToAdd := float64(bucket.Rate) * elapsed.Seconds() / bucket.RefillPeriod.Seconds()

    // Tambahkan token, tapi jangan melebihi burst capacity
    bucket.Tokens = math.Min(bucket.Tokens+tokensToAdd, float64(bucket.Burst))
    bucket.LastRefill = now

    return bucket
}
```

## Simulasi Konfigurasi

### Konfigurasi 1: Rate 5, Burst 10 (Balanced)

**Karakteristik**: Pengisian sedang, ember sedang

```
Waktu 0 detik:
🪣 Ember: [🟡🟡🟡🟡🟡🟡🟡🟡🟡🟡] (10/10 token)

Request 8 token sekaligus:
🪣 Ember: [🟡🟡⚪⚪⚪⚪⚪⚪⚪⚪] (2/10 token tersisa)
✅ Request diterima

Tunggu 1 detik (Rate = 5):
⏰ Pengisian: +5 token
🪣 Ember: [🟡🟡🟡🟡🟡🟡🟡⚪⚪⚪] (7/10 token)

Request 10 token:
❌ Ditolak! (hanya ada 7 token, butuh 10)
```

### Konfigurasi 2: Rate 100, Burst 10 (Fast Refill, Small Bucket)

**Karakteristik**: Pengisian sangat cepat, ember kecil

```
🪣 Ember: [🟡🟡🟡🟡🟡🟡🟡🟡🟡🟡] (maksimal 10 token)
⚡ Pengisian: 100 token per detik = 1 token setiap 0.01 detik

Skenario Burst Traffic:
Waktu 0.000s: Request 8 token
✅ DITERIMA 
🪣 [🟡🟡⚪⚪⚪⚪⚪⚪⚪⚪] (2/10 token tersisa)

Waktu 0.080s (80ms kemudian):
⚡ Pengisian: 100 × 0.08 = 8 token ditambahkan
🪣 [🟡🟡🟡🟡🟡🟡🟡🟡🟡🟡] (10/10 token - penuh lagi!)

Recovery Time: ~0.08 detik untuk penuh
```

### Konfigurasi 3: Rate 10, Burst 100 (Slow Refill, Large Bucket)

**Karakteristik**: Pengisian lambat, ember besar

```
🪣 Ember: [🟡🟡🟡...🟡] (100 token kapasitas)
🐌 Pengisian: 10 token per detik

Skenario Burst Traffic:
Waktu 0s: Request 50 token sekaligus
✅ DITERIMA 
🪣 Token tersisa: 50

Waktu 5s: Request 60 token
❌ DITOLAK (ada 50 + (10×5) = 100 token total, tapi hanya 50 tersisa + 50 refill = 100, butuh 60)

Contoh yang benar:
Waktu 0s: Token awal = 100, Request 50 token
✅ DITERIMA, sisa = 50 token

Waktu 5s: 
- Token yang masuk = 10 × 5 = 50 token
- Total token = min(50 + 50, 100) = 100 token (dibatasi burst capacity)
- Request 60 token
✅ DITERIMA, sisa = 100 - 60 = 40 token

Recovery Time: ~10 detik untuk penuh dari kosong
```

## Pattern Traffic Berbagai Konfigurasi

### Rate 100, Burst 10
```
Traffic Pattern: ████⚪⚪████⚪⚪████⚪⚪
Artinya: 10 request cepat, istirahat sebentar, 10 lagi
Cocok untuk: API yang butuh responsif tapi cegah spam
```

### Rate 10, Burst 100
```
Traffic Pattern: ████████████████████████████████⚪⚪⚪⚪⚪⚪⚪⚪⚪⚪
Artinya: 100 request langsung, tunggu lama (10 detik) untuk refill
Cocok untuk: Batch processing, upload file besar
```

### Rate 100, Burst 100
```
Traffic Pattern: ████████████████████████████████████████████████
Artinya: 100 request terus menerus tanpa jeda
Cocok untuk: High-throughput API, internal services
```

## Implementasi dalam Kode

### Token Bucket Structure
```go
type TokenBucket struct {
    Tokens       float64       `json:"tokens"`        // Jumlah token saat ini
    LastRefill   time.Time     `json:"last_refill"`   // Waktu terakhir refill
    Rate         int           `json:"rate"`          // Token per detik
    Burst        int           `json:"burst"`         // Kapasitas maksimal
    RefillPeriod time.Duration `json:"refill_period"` // Periode pengisian
}
```

### Allow Method
```go
func (rl *RateLimiter) Allow(ctx context.Context, identifier string) (bool, error) {
    // 1. Get current bucket state dari Redis
    bucket, err := rl.getBucket(ctx, key)
    
    // 2. Refill tokens berdasarkan elapsed time
    bucket = rl.refillBucket(bucket, time.Now())
    
    // 3. Check apakah ada token yang tersedia
    if bucket.Tokens >= 1.0 {
        bucket.Tokens -= 1.0  // Konsumsi 1 token
        rl.saveBucket(ctx, key, bucket)  // Simpan state
        return true, nil
    }
    
    return false, nil  // Tidak ada token
}
```

## Simulasi Testing

### Test Code Example
```go
func simulasiRate100Burst10() {
    start := time.Now()
    
    for i := 0; i < 25; i++ {
        allowed, _ := limiter.Allow(ctx, "user123")
        elapsed := time.Since(start).Milliseconds()
        
        if allowed {
            fmt.Printf("✅ Request %d pada %dms: BERHASIL\n", i+1, elapsed)
        } else {
            fmt.Printf("❌ Request %d pada %dms: DITOLAK\n", i+1, elapsed)
        }
        
        time.Sleep(5 * time.Millisecond)
    }
}
```

### Expected Output (Rate 100, Burst 10)
```
✅ Request 1 pada 0ms: BERHASIL
✅ Request 2 pada 5ms: BERHASIL
...
✅ Request 10 pada 45ms: BERHASIL
❌ Request 11 pada 50ms: DITOLAK
❌ Request 12 pada 55ms: DITOLAK  
✅ Request 13 pada 60ms: BERHASIL (mulai refill)
✅ Request 14 pada 65ms: BERHASIL
```

## Use Cases dan Rekomendasi

### API Login/Authentication
```
Rate: 5/detik, Burst: 10
Tujuan: Cegah brute force, tapi izinkan retry normal
```

### API Search/Query
```
Rate: 20/detik, Burst: 50  
Tujuan: Izinkan pencarian cepat, cegah scraping massal
```

### File Upload API
```
Rate: 2/detik, Burst: 5
Tujuan: Batasi upload simultan, hemat bandwidth
```

### Internal API/Microservices
```
Rate: 1000/detik, Burst: 2000
Tujuan: High throughput dengan protection
```

### Public API dengan Quota
```
Rate: 100/detik, Burst: 200
Tujuan: Balanced performance dan protection
```

## Redis Storage Pattern

```
Key Pattern:
- rate_limit:{identifier}:tokens → "7.5"
- rate_limit:{identifier}:last_refill → "1699891200123456789" (nanoseconds)

TTL: 2x refill_period (untuk safety)

Contoh:
rate_limit:user123:tokens = "8.5"
rate_limit:user123:last_refill = "1699891200500000000"
TTL = 2 seconds
```

## Keuntungan Token Bucket

1. **Burst Tolerance**: Memungkinkan lonjakan traffic sementara
2. **Smooth Rate**: Menjaga rata-rata throughput jangka panjang
3. **Flexible Configuration**: Rate dan burst bisa disesuaikan kebutuhan
4. **Memory Efficient**: Hanya perlu simpan state minimal
5. **Distributed Ready**: Bisa diimplementasi dengan Redis untuk multi-instance

## Perbandingan dengan Algoritma Lain

### vs Fixed Window
```
Token Bucket: ████⚪⚪████⚪⚪████ (smooth)
Fixed Window: ██████⚪⚪⚪⚪██████ (bursty di awal window)
```

### vs Sliding Window Log  
```
Token Bucket: O(1) complexity, minimal storage
Sliding Window: O(n) complexity, banyak storage untuk log
```

### vs Leaky Bucket
```
Token Bucket: Allow burst traffic
Leaky Bucket: Constant output rate, no burst
```

---

*Dokumentasi ini menjelaskan implementasi Token Bucket algorithm dalam Traefik Quota Plugin untuk rate limiting yang efektif dan fleksibel.*