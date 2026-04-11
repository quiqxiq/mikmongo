# Implementation Prompt: Integrating Collector Lifecycle into MikMongo Server

## Konteks Proyek

- **Module:** `mikmongo` (Go 1.25.5)
- **HTTP Framework:** Gin v1.12
- **Arsitektur:** Layered Architecture + Registry Pattern
- **Layer yang ada:** `internal/model/` → `internal/repository/` → `internal/domain/` → `internal/service/` → `internal/handler/`
- **PENTING:** Tidak ada layer `usecase/`. Business logic ada di `internal/service/`.
- **Entry point utama:** `cmd/server/main.go`
- **Shared libraries:** `pkg/mikrotik/`, `pkg/redis/`, `pkg/logger/`

---

## Status Saat Ini (Sudah Ada, Jangan Duplikat)

### Pipeline collector sudah fully implemented di `pkg/mikrotik/collector/`

Tiga pipeline sudah berjalan dengan 10 default spec:

| Spec | Pipeline | Mekanisme | Storage | Interval |
|------|----------|-----------|---------|----------|
| `interface_traffic` | A (time_series) | ListenRaw streaming | InfluxDB | real-time |
| `queue_stats` | A (time_series) | ListenRaw streaming | InfluxDB | real-time |
| `system_resource` | A (time_series) | ListenRaw streaming | InfluxDB | real-time |
| `ppp_active` | B (operational) | follow=yes (Tier2) | Redis HSET | event-driven |
| `hotspot_active` | B (operational) | follow=yes (Tier2) | Redis HSET | event-driven |
| `interface_status` | B (operational) | follow=yes (Tier2) | Redis HSET | event-driven |
| `ppp_secrets` | B (operational) | ticker (Tier3) | Redis HSET | 5 menit |
| `ppp_profiles` | B (operational) | ticker (Tier3) | Redis HSET | 5 menit |
| `hotspot_users` | B (operational) | ticker (Tier3) | Redis HSET | 5 menit |
| `ip_pools` | B (operational) | ticker (Tier3) | Redis HSET | 10 menit |

Definisi 10 spec di atas saat ini tersebar antara `DefaultISPSpecs()` dan `Tier3Specs()` di dalam `pkg/mikrotik/collector/`. Struct `CommandSpec` dan type constants juga ada di sana.

### Gap Utama yang Perlu Diselesaikan

Ada dua gap yang harus diselesaikan bersamaan:

**Gap 1 — Spec definition tidak punya rumah yang tepat.**
`CommandSpec` dan spec instances ada di dalam `pkg/mikrotik/collector/`. Ini berarti komponen lain (future agent, validator, dashboard config) harus import `collector` hanya untuk membaca definisi spec — padahal mereka tidak butuh collector logic sama sekali. Ini akan menciptakan import cycle di masa depan.

**Gap 2 — Collector tidak terintegrasi ke server lifecycle.**
`cmd/collector/main.go` adalah standalone CLI tool. `cmd/server/main.go` belum menginisialisasi dan menjalankan collector per router secara otomatis. Akibatnya:
1. Dashboard frontend mengambil data langsung dari RouterOS API setiap request (lambat, membebani router).
2. Tidak ada lifecycle management — collector tidak otomatis start saat router ditambah, tidak stop saat router di-delete.
3. Data Pipeline B yang sudah tersimpan di Redis tidak dimanfaatkan oleh HTTP handler.

**Catatan penting tentang source of truth status router:**
Status router di database (`model.MikrotikRouter.Status`) bukan real-time — hanya di-update saat `SyncAllDevices()` atau `SyncDevice()` dipanggil (cron 5 menit atau manual trigger). Antara dua sync, status DB bisa stale. Oleh karena itu:

- **DB status bukan trigger** untuk start/stop collector
- **`collector.Supervisor`** (health check loop 30 detik, sudah ada di `pkg/mikrotik/collector/`) adalah source of truth runtime untuk konektivitas router
- **DB status adalah lagging indicator** — di-update berdasarkan laporan dari Supervisor, bukan sebaliknya

---

## Tugas yang Harus Dikerjakan

### Tugas 1: Buat Package `pkg/mikrotik/spec/`

Pindahkan semua type definition dan spec instances dari `pkg/mikrotik/collector/` ke package baru yang berdiri sendiri: `pkg/mikrotik/spec/`. Package ini **tidak boleh** import `pkg/mikrotik/collector/` — satu arah saja: collector import spec, bukan sebaliknya.

#### Struktur folder yang harus dibuat:

```
pkg/mikrotik/spec/
  types.go        ← CommandSpec struct + semua type constants
  interface.go    ← Specs domain interface RouterOS
  system.go       ← Specs domain system RouterOS
  ppp.go          ← Specs domain PPP RouterOS
  hotspot.go      ← Specs domain hotspot RouterOS
  queue.go        ← Specs domain queue RouterOS
  network.go      ← Specs domain network (IP, firewall) RouterOS
  registry.go     ← Aggregator: AllSpecs(), DefaultSpecs()
```

---

#### `pkg/mikrotik/spec/types.go`

Pindahkan dan bersihkan dari `pkg/mikrotik/collector/spec.go` yang lama. Isi yang diharapkan:

```go
package spec

import "time"

type PipelineType string

const (
    PipelineTimeSeries  PipelineType = "time_series"
    PipelineOperational PipelineType = "operational"
    PipelineOnDemand    PipelineType = "on_demand"
)

type Tier int

const (
    Tier2 Tier = 2 // Event-driven: follow=yes
    Tier3 Tier = 3 // Static: ticker + TTL
)

type StorageType string

const (
    StorageInfluxDB StorageType = "influxdb"
    StorageRedis    StorageType = "redis"
    StorageNone     StorageType = "none"
)

type CommandSpec struct {
    Name      string
    Pipeline  PipelineType
    Tier      Tier
    Storage   StorageType
    Args      []string
    UseFollow bool
    Interval  time.Duration

    // InfluxDB config (Pipeline A)
    Measurement string
    TagFields   []string
    ValueFields []string

    // Redis config (Pipeline B)
    RedisKey string
    KeyField string
    TTL      time.Duration

    Enabled bool
}

// Helper methods — cek pipeline dan tier tanpa kalkulasi manual

func (s CommandSpec) IsTimeSeries() bool {
    return s.Pipeline == PipelineTimeSeries
}

func (s CommandSpec) IsOperational() bool {
    return s.Pipeline == PipelineOperational
}

func (s CommandSpec) IsOnDemand() bool {
    return s.Pipeline == PipelineOnDemand
}

func (s CommandSpec) IsTier2() bool {
    return s.Pipeline == PipelineOperational && s.Tier == Tier2
}

func (s CommandSpec) IsTier3() bool {
    return s.Pipeline == PipelineOperational && s.Tier == Tier3
}
```

**Catatan:** Tidak ada field `ToInflux bool` (redundant dengan `Pipeline`) dan tidak ada method `IsStreaming()` (redundant dengan `UseFollow`).

---

#### `pkg/mikrotik/spec/interface.go`

Pindahkan spec yang domain-nya interface RouterOS. Tambahkan `interface_list` (spec baru):

```go
package spec

import "time"

func InterfaceSpecs() []CommandSpec {
    return []CommandSpec{
        {
            Name:        "interface_traffic",
            Pipeline:    PipelineTimeSeries,
            Storage:     StorageInfluxDB,
            Args:        []string{"/interface/monitor-traffic", "=interface=all", "=once="},
            UseFollow:   true,
            Measurement: "interface_traffic",
            TagFields:   []string{"name"},
            ValueFields: []string{"rx-bits-per-second", "tx-bits-per-second", "rx-packets-per-second", "tx-packets-per-second", "rx-drops", "tx-drops"},
            Enabled:     true,
        },
        {
            Name:      "interface_status",
            Pipeline:  PipelineOperational,
            Tier:      Tier2,
            Storage:   StorageRedis,
            Args:      []string{"/interface/print", "=follow="},
            UseFollow: true,
            RedisKey:  "interface:status",
            KeyField:  "name",
            Enabled:   true,
        },
        {
            // SPEC BARU — Tier3, polling daftar interface lengkap
            // Divalidasi dari routeros_7_20_8.json: /interface/print tersedia
            Name:     "interface_list",
            Pipeline: PipelineOperational,
            Tier:     Tier3,
            Storage:  StorageRedis,
            Args:     []string{"/interface/print"},
            Interval: 5 * time.Minute,
            RedisKey: "interface:list",
            KeyField: "name",
            Enabled:  true,
        },
    }
}
```

---

#### `pkg/mikrotik/spec/system.go`

Pindahkan spec system. Tambahkan `router_log` (spec baru):

```go
package spec

import "time"

func SystemSpecs() []CommandSpec {
    return []CommandSpec{
        {
            Name:        "system_resource",
            Pipeline:    PipelineTimeSeries,
            Storage:     StorageInfluxDB,
            Args:        []string{"/system/resource/monitor", "=interval=2s", "=once="},
            UseFollow:   true,
            Measurement: "system_resource",
            TagFields:   []string{"router_id"},
            ValueFields: []string{"cpu-load", "free-memory", "total-memory", "uptime", "cpu-count", "version"},
            Enabled:     true,
        },
        {
            // SPEC BARU — log streaming real-time
            // Divalidasi dari routeros_7_20_8.json: /log/print memiliki arg follow
            // Domain struct: LogEntry sudah ada di pkg/mikrotik/domain/system.go
            // TTL: 1 jam karena log berputar — data lama tidak relevan
            Name:      "router_log",
            Pipeline:  PipelineOperational,
            Tier:      Tier2,
            Storage:   StorageRedis,
            Args:      []string{"/log/print", "=follow="},
            UseFollow: true,
            RedisKey:  "log:entries",
            TTL:       1 * time.Hour,
            Enabled:   true,
        },
    }
}
```

---

#### `pkg/mikrotik/spec/ppp.go`

Pindahkan semua spec domain PPP:

```go
package spec

import "time"

func PPPSpecs() []CommandSpec {
    return []CommandSpec{
        {
            Name:      "ppp_active",
            Pipeline:  PipelineOperational,
            Tier:      Tier2,
            Storage:   StorageRedis,
            Args:      []string{"/ppp/active/print", "=follow="},
            UseFollow: true,
            RedisKey:  "ppp:active",
            KeyField:  "name",
            Enabled:   true,
        },
        {
            Name:     "ppp_secrets",
            Pipeline: PipelineOperational,
            Tier:     Tier3,
            Storage:  StorageRedis,
            Args:     []string{"/ppp/secret/print"},
            Interval: 5 * time.Minute,
            RedisKey: "ppp:secrets",
            KeyField: "name",
            Enabled:  true,
        },
        {
            Name:     "ppp_profiles",
            Pipeline: PipelineOperational,
            Tier:     Tier3,
            Storage:  StorageRedis,
            Args:     []string{"/ppp/profile/print"},
            Interval: 5 * time.Minute,
            RedisKey: "ppp:profiles",
            KeyField: "name",
            Enabled:  true,
        },
    }
}
```

---

#### `pkg/mikrotik/spec/hotspot.go`

```go
package spec

import "time"

func HotspotSpecs() []CommandSpec {
    return []CommandSpec{
        {
            Name:      "hotspot_active",
            Pipeline:  PipelineOperational,
            Tier:      Tier2,
            Storage:   StorageRedis,
            Args:      []string{"/ip/hotspot/active/print", "=follow="},
            UseFollow: true,
            RedisKey:  "hotspot:active",
            KeyField:  "user",
            Enabled:   true,
        },
        {
            Name:     "hotspot_users",
            Pipeline: PipelineOperational,
            Tier:     Tier3,
            Storage:  StorageRedis,
            Args:     []string{"/ip/hotspot/user/print"},
            Interval: 5 * time.Minute,
            RedisKey: "hotspot:users",
            KeyField: "name",
            Enabled:  true,
        },
    }
}
```

---

#### `pkg/mikrotik/spec/queue.go`

```go
package spec

func QueueSpecs() []CommandSpec {
    return []CommandSpec{
        {
            Name:        "queue_stats",
            Pipeline:    PipelineTimeSeries,
            Storage:     StorageInfluxDB,
            Args:        []string{"/queue/simple/print", "=stats="},
            UseFollow:   true,
            Measurement: "queue_stats",
            TagFields:   []string{"name"},
            ValueFields: []string{"bytes", "packets", "dropped", "rate"},
            Enabled:     true,
        },
    }
}
```

---

#### `pkg/mikrotik/spec/network.go`

Spec domain network (IP address, firewall, IP pool). Tambahkan `ip_addresses` dan `firewall_nat` (spec baru):

```go
package spec

import "time"

func NetworkSpecs() []CommandSpec {
    return []CommandSpec{
        {
            Name:     "ip_pools",
            Pipeline: PipelineOperational,
            Tier:     Tier3,
            Storage:  StorageRedis,
            Args:     []string{"/ip/pool/print"},
            Interval: 10 * time.Minute,
            RedisKey: "ip:pools",
            KeyField: "name",
            Enabled:  true,
        },
        {
            // SPEC BARU
            // Divalidasi dari routeros_7_20_8.json: /ip/address/print tersedia
            Name:     "ip_addresses",
            Pipeline: PipelineOperational,
            Tier:     Tier3,
            Storage:  StorageRedis,
            Args:     []string{"/ip/address/print"},
            Interval: 5 * time.Minute,
            RedisKey: "ip:addresses",
            KeyField: "address",
            Enabled:  true,
        },
        {
            // SPEC BARU
            // Divalidasi dari routeros_7_20_8.json: /ip/firewall/nat/print tersedia
            Name:     "firewall_nat",
            Pipeline: PipelineOperational,
            Tier:     Tier3,
            Storage:  StorageRedis,
            Args:     []string{"/ip/firewall/nat/print"},
            Interval: 10 * time.Minute,
            RedisKey: "firewall:nat",
            KeyField: ".id",
            Enabled:  true,
        },
    }
}
```

---

#### `pkg/mikrotik/spec/registry.go`

Single aggregator. Ini pengganti `DefaultISPSpecs()` dan `Tier3Specs()` yang lama:

```go
package spec

// AllSpecs mengembalikan seluruh spec yang terdaftar, dari semua domain.
// Ini adalah sumber kebenaran tunggal untuk semua collector di sistem.
func AllSpecs() []CommandSpec {
    var all []CommandSpec
    all = append(all, InterfaceSpecs()...)
    all = append(all, SystemSpecs()...)
    all = append(all, PPPSpecs()...)
    all = append(all, HotspotSpecs()...)
    all = append(all, QueueSpecs()...)
    all = append(all, NetworkSpecs()...)
    return all
}

// DefaultSpecs mengembalikan hanya spec yang Enabled == true.
// Ini yang digunakan oleh CollectorService saat StartRouter.
func DefaultSpecs() []CommandSpec {
    all := AllSpecs()
    enabled := make([]CommandSpec, 0, len(all))
    for _, s := range all {
        if s.Enabled {
            enabled = append(enabled, s)
        }
    }
    return enabled
}
```

---

#### Update `pkg/mikrotik/collector/` setelah migrasi

Setelah `pkg/mikrotik/spec/` dibuat, lakukan:

1. Hapus `pkg/mikrotik/collector/spec.go` yang lama.
2. Di semua file dalam `pkg/mikrotik/collector/`, ganti reference ke type lokal dengan import dari `pkg/mikrotik/spec`:
   ```go
   import "mikmongo/pkg/mikrotik/spec"
   // Gunakan: spec.CommandSpec, spec.PipelineTimeSeries, spec.Tier2, dst.
   ```
3. Ganti semua pemanggilan `DefaultISPSpecs()` dan `Tier3Specs()` dengan `spec.DefaultSpecs()`.
4. Pastikan `pkg/mikrotik/collector/` **tidak export** type `CommandSpec` atau constants pipeline — semua user harus import dari `pkg/mikrotik/spec/`.

---

### Tugas 2: Buat `CollectorService` di `internal/service/`

Buat file baru: `internal/service/collector_service.go`

Service ini adalah **integration layer** antara `pkg/mikrotik/collector/` dan lifecycle router. Ikuti pola registry yang ada di `internal/service/registry.go`.

**Interface:**

```go
package service

type CollectorStatus struct {
    RouterID    string
    Running     bool    // true = Supervisor aktif (tidak berarti koneksi selalu hidup)
    SpecCount   int
    StartedAt   time.Time
    LastError   error   // error terakhir yang dicatat Supervisor, nil jika sehat
}

type CollectorService interface {
    // StartRouter mendaftarkan router ke collector dan menjalankan Supervisor-nya.
    // Tidak melakukan live connection test — Supervisor yang handle reconnect.
    // Idempotent: jika sudah running, no-op, return nil.
    StartRouter(ctx context.Context, routerID string) error

    // StopRouter menghentikan Supervisor dan semua pipeline untuk satu router.
    // Idempotent: jika tidak ada entry untuk routerID ini, no-op.
    StopRouter(routerID string)

    // StopAll menghentikan semua collector. Blocking max 10 detik.
    // Dipanggil saat graceful shutdown server.
    StopAll()

    // Status mengembalikan snapshot status runtime Supervisor untuk satu router.
    // Running=false bisa berarti: belum start, sudah stop, atau koneksi belum recover.
    Status(routerID string) CollectorStatus
}
```

**Implementasi `collectorServiceImpl`:**

Dependensi:
- `mikrotikManager *mikrotikClient.Manager` — dari `pkg/mikrotik/client/`
- `redisClient *redis.Client` — dari `pkg/redis/`
- `influxClient influxdb3.Client` — sesuai nama pkg yang ada
- `logger *logger.Logger` — dari `pkg/logger/`

Internal state:
```go
type collectorServiceImpl struct {
    // ... deps ...
    mu         sync.RWMutex
    collectors map[string]*collector.Collector // key: routerID
    statuses   map[string]CollectorStatus
    wg         sync.WaitGroup
}
```

**Alur `StartRouter`:**
1. `RLock` — cek map. Jika sudah ada entry dan `Running == true`, return nil (idempotent).
2. Ambil config router dari `mikrotikManager` untuk mendapatkan host/credentials. **Jangan panggil `TestConnection`** — biarkan Supervisor yang handle koneksi awal dan retry.
3. Buat `collector.Collector` baru dengan Redis + InfluxDB backend.
4. Load specs dari `spec.DefaultSpecs()`.
5. Jalankan `collector.Start(ctx)` dalam goroutine, tambahkan ke `wg`.
6. `Lock` — simpan ke map, update `statuses` dengan `Running: true, StartedAt: time.Now()`.

**Alur `StopRouter`:**
1. Cek map — jika tidak ada entry, return (no-op).
2. Panggil `collector.Stop()` — ini cancel context Supervisor.
3. `Lock` — hapus dari map, update `statuses` dengan `Running: false`.

**`StopAll`:** Iterate map, panggil `StopRouter` untuk semua entry. Lalu `wg.Wait()` dengan `context.WithTimeout` 10 detik. Jika timeout, log `Error` tapi tetap lanjut shutdown.

**Kenapa tidak ada live connection test di `StartRouter`:**
`collector.Supervisor` sudah punya health check loop setiap 30 detik dengan exponential backoff reconnect (1s–30s, sudah ada di `pkg/mikrotik/collector/`). Jika router unreachable saat `StartRouter`, Supervisor akan retry secara otomatis tanpa perlu `StartRouter` dipanggil ulang. `CollectorStatus.LastError` mencerminkan error terbaru dari Supervisor.

**Pola concurrency:** `sync.RWMutex` untuk proteksi map. Tidak perlu `sync.Pool`.

---

### Tugas 3: Wiring di `cmd/server/main.go`

Sisipkan setelah `service.NewRegistry()` (baris ke-13 urutan inisialisasi):

```go
// Inisialisasi CollectorService
collectorSvc := service.NewCollectorService(
    mikrotikManager,  // pkg/mikrotik/client.Manager
    redisClient,
    influxClient,
    logger,
)

// Saat server boot: start collector untuk SEMUA router di DB,
// tidak peduli status DB saat ini.
// Alasan: DB status bisa stale. Supervisor yang akan handle
// koneksi awal dan retry jika router belum reachable.
routers, err := svc.Router.ListAll(ctx)
if err != nil {
    logger.Error("failed to list routers on boot", zap.Error(err))
} else {
    for _, r := range routers {
        if err := collectorSvc.StartRouter(ctx, r.ID.String()); err != nil {
            // Warn saja, jangan Fatal — server tetap harus jalan
            logger.Warn("collector not started on boot",
                zap.String("router_id", r.ID.String()),
                zap.String("router_name", r.Name),
                zap.Error(err),
            )
        }
    }
}
```

Di graceful shutdown, **sebelum** `httpServer.Shutdown`:

```go
logger.Info("stopping all collectors...")
collectorSvc.StopAll()
logger.Info("all collectors stopped")
```

Tambahkan ke `internal/service/registry.go`:

```go
type Registry struct {
    // ... semua field yang sudah ada, jangan dihapus ...
    Collector CollectorService
}
```

---

### Tugas 4: Integrasikan ke `RouterService`

Di `internal/service/router_service.go`, inject `CollectorService` sebagai dependensi. Perhatikan **perbedaan peran** antara event-driven triggers dan sync jobs:

#### 4a. Event-driven triggers (user action → collector lifecycle)

Hanya dua event yang memang harus trigger start/stop collector secara eksplisit:

| Method | Event | Aksi Collector |
|--------|-------|----------------|
| `Create()` | Router baru ditambahkan | `collectorSvc.StartRouter(ctx, routerID)` — Supervisor mulai, koneksi awal ditangani sendiri |
| `Delete()` | Router dihapus permanen | `collectorSvc.StopRouter(routerID)` — wajib stop agar tidak ada goroutine orphan |
| `SyncDevice()` | User manual trigger sync satu router | Test koneksi live → jika berhasil, `StartRouter` (idempotent jika sudah jalan) — jika gagal, `StopRouter` + log |

**Kenapa `Create()` langsung start tanpa cek status?**
Tidak ada status yang bisa dicek — router baru saja masuk DB. Supervisor akan handle jika host belum reachable.

**Kenapa `SyncDevice()` berbeda dari `SyncAllDevices()`?**
`SyncDevice()` adalah aksi eksplisit user dari UI — user bermaksud "cek dan perbaiki koneksi router ini". Wajar jika trigger restart collector. Sedangkan `SyncAllDevices()` adalah cron background job — lihat 4b.

#### 4b. `SyncAllDevices()` — baca dari Supervisor, update DB

`SyncAllDevices()` adalah cron job yang jalan setiap 5 menit. Tugasnya **bukan** mengontrol collector — Supervisor sudah handle reconnect sendiri. Tugasnya adalah **melaporkan status runtime ke DB** agar dashboard bisa menampilkan status yang tidak terlalu stale.

Ubah logika `SyncAllDevices()`:

```go
// SEBELUM (salah): cek koneksi aktif → update DB
// SESUDAH: baca status dari Supervisor → update DB

func (s *routerService) SyncAllDevices(ctx context.Context) error {
    routers, err := s.repo.ListAll(ctx)
    if err != nil {
        return err
    }

    for _, r := range routers {
        status := s.collectorSvc.Status(r.ID.String())

        // Update DB berdasarkan apa yang Supervisor laporkan,
        // bukan berdasarkan live TCP test.
        newStatus := "offline"
        if status.Running && status.LastError == nil {
            newStatus = "online"
        }

        if string(r.Status) != newStatus {
            _ = s.repo.UpdateStatus(ctx, r.ID, newStatus)
            s.logger.Info("router status updated",
                zap.String("router_id", r.ID.String()),
                zap.String("old_status", string(r.Status)),
                zap.String("new_status", newStatus),
            )
        }
    }
    return nil
}
```

**Catatan penting:** `Running=true && LastError==nil` bukan jaminan router online 100% saat itu juga — Supervisor mungkin sedang di antara dua health check. Tapi ini jauh lebih akurat dari DB status yang bisa tertinggal berjam-jam. Jika perlu akurasi tinggi, gunakan endpoint `/collector/status` dari Tugas 5 langsung di frontend.

#### Ringkasan perbedaan peran

```
Create()          → StartRouter (Supervisor mulai, retry otomatis)
Delete()          → StopRouter  (goroutine bersih)
SyncDevice()      → TestConnection live → StartRouter atau StopRouter
SyncAllDevices()  → baca Status() → update DB (tidak trigger start/stop)
Supervisor        → reconnect otomatis setiap 30 detik (sudah ada, jangan diubah)
```

**Tidak ada `IsOnline()` check dari DB** di manapun dalam alur collector. DB status adalah output, bukan input.

---

### Tugas 5: Tambahkan HTTP Endpoint untuk Cached Data

Di `internal/handler/mikrotik/`, tambahkan pola cache-first untuk data Pipeline B.

**Pola cache-first** (contoh PPP Active):

```go
// SESUDAH (cache-first, fallback ke RouterOS):
func (h *PPPHandler) GetActive(c *gin.Context) {
    routerID := c.GetString("router_id") // dari MikrotikRouterMiddleware

    redisKey := fmt.Sprintf("mikmongo:%s:ppp:active", routerID)
    cached, err := h.redisClient.HGetAll(c, redisKey)
    if err == nil && len(cached) > 0 {
        response.Success(c, parsePPPActive(cached))
        return
    }

    // Fallback: cache kosong atau Redis down
    data, err := h.pppService.GetActiveConnections(c, routerID)
    if err != nil {
        response.InternalServerError(c, err)
        return
    }
    response.Success(c, data)
}
```

**Endpoint baru untuk spec yang ditambahkan di Tugas 1:**

| Method | Path | Redis Key | Domain Struct | Fallback |
|--------|------|-----------|---------------|---------|
| `GET` | `/routers/:router_id/logs` | `mikmongo:{id}:log:entries` | `[]domain.LogEntry` | `/log/print` via service |
| `GET` | `/routers/:router_id/ip/addresses/cached` | `mikmongo:{id}:ip:addresses` | `[]domain.IPAddress` | `/ip/address/print` via service |
| `GET` | `/routers/:router_id/firewall/nat/cached` | `mikmongo:{id}:firewall:nat` | `[]domain.NATRule` | `/ip/firewall/nat/print` via service |
| `GET` | `/routers/:router_id/collector/status` | — | `service.CollectorStatus` | — (no fallback) |

Domain struct `LogEntry`, `IPAddress`, `NATRule` sudah ada di `pkg/mikrotik/domain/`. Semua endpoint ini menggunakan `MikrotikRouterMiddleware` yang sudah ada.

---

## Spesifikasi Teknis Wajib

### Import Direction (Wajib Diikuti)

```
pkg/mikrotik/spec/          ← tidak import siapapun dari pkg/mikrotik/
pkg/mikrotik/collector/     ← import pkg/mikrotik/spec/
pkg/mikrotik/client/        ← tidak import spec atau collector
internal/service/           ← import pkg/mikrotik/spec/ dan pkg/mikrotik/collector/
internal/handler/           ← tidak import pkg/mikrotik/spec/ secara langsung
```

Tidak boleh ada arah sebaliknya. Jika ada import cycle, artinya ada type yang salah tempat.

### Graceful Shutdown

- `CollectorService.StopAll()` blocking, menunggu semua goroutine via `sync.WaitGroup`.
- Timeout maksimal 10 detik (total server graceful shutdown 30 detik di `cmd/server/main.go`).

### Error Handling

- `StartRouter` gagal → `logger.Warn`, server tetap jalan.
- Redis down saat pipeline B menulis → ikuti `BatchWriter` yang ada: `MaxRetries: 3`, exponential backoff.
- `StopRouter` pada `routerID` yang tidak ada di map → no-op, tidak return error.

### Redis Key Convention

Ikuti pola `pkg/redis/` dan `ANALYSIS.md §7.2`:

```
mikmongo:{routerID}:{resource}:{subkey}

Contoh:
mikmongo:550e8400-e29b-41d4-a716:log:entries
mikmongo:550e8400-e29b-41d4-a716:ip:addresses
mikmongo:550e8400-e29b-41d4-a716:firewall:nat
mikmongo:550e8400-e29b-41d4-a716:interface:list
```

`routerID` adalah UUID string. Tidak ada key Pipeline B yang boleh ditulis tanpa prefix `routerID`.

### Logging

Gunakan `pkg/logger/` (Zap wrapper):

```go
logger.Info("collector started",
    zap.String("router_id", routerID),
    zap.Int("spec_count", len(specs)),
)
logger.Info("collector stopped",
    zap.String("router_id", routerID),
)
logger.Warn("collector start failed",
    zap.String("router_id", routerID),
    zap.Error(err),
)
```

---

## Ringkasan File yang Harus Dibuat / Dimodifikasi

| Aksi | File | Keterangan |
|------|------|------------|
| **BUAT** | `pkg/mikrotik/spec/types.go` | CommandSpec struct + type constants (bersih, tanpa ToInflux) |
| **BUAT** | `pkg/mikrotik/spec/interface.go` | InterfaceSpecs() — termasuk interface_list baru |
| **BUAT** | `pkg/mikrotik/spec/system.go` | SystemSpecs() — termasuk router_log baru |
| **BUAT** | `pkg/mikrotik/spec/ppp.go` | PPPSpecs() |
| **BUAT** | `pkg/mikrotik/spec/hotspot.go` | HotspotSpecs() |
| **BUAT** | `pkg/mikrotik/spec/queue.go` | QueueSpecs() |
| **BUAT** | `pkg/mikrotik/spec/network.go` | NetworkSpecs() — termasuk ip_addresses dan firewall_nat baru |
| **BUAT** | `pkg/mikrotik/spec/registry.go` | AllSpecs(), DefaultSpecs() |
| **HAPUS** | `pkg/mikrotik/collector/spec.go` | Digantikan oleh pkg/mikrotik/spec/ |
| **MODIFIKASI** | `pkg/mikrotik/collector/*.go` | Ganti import + reference ke spec package baru |
| **BUAT** | `internal/service/collector_service.go` | Interface + implementasi CollectorService |
| **MODIFIKASI** | `internal/service/registry.go` | Tambah field Collector CollectorService |
| **MODIFIKASI** | `internal/service/router_service.go` | Inject CollectorService; Create→StartRouter, Delete→StopRouter, SyncDevice→live test+start/stop, SyncAllDevices→baca Status()→update DB |
| **MODIFIKASI** | `cmd/server/main.go` | Init CollectorService, StartRouter semua router (bukan hanya online), StopAll on shutdown |
| **MODIFIKASI** | `internal/handler/mikrotik/log_handler.go` | Cache-first untuk endpoint /logs |
| **MODIFIKASI** | `internal/handler/mikrotik/monitor_handler.go` | Tambah endpoint /collector/status |
| **BUAT** | `internal/handler/mikrotik/cached_handler.go` | Cache-first untuk /ip/addresses/cached, /firewall/nat/cached |
| **MODIFIKASI** | `internal/router/mikrotik_routes.go` | Register 4 route baru |

---

## Yang Tidak Boleh Diubah

- Jangan ubah `cmd/collector/main.go` — tetap bisa digunakan sebagai standalone CLI. Update import-nya ke `pkg/mikrotik/spec/` saja.
- Jangan buat folder `internal/usecase/` — tidak sesuai arsitektur yang ada.
- Jangan install package baru. Semua dependency sudah tersedia.
- Jangan ubah interface `collector.Collector` yang sudah ada — hanya update import path-nya.
- Jangan hapus `BatchWriter`, `InfluxHandler`, `RedisHandler` di `pkg/mikrotik/collector/writer/` — tetap digunakan oleh collector.