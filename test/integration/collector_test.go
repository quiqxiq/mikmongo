//go:build integration

package integration

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/Butterfly-Student/go-ros/client"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	goredis "github.com/redis/go-redis/v9"
	"go.uber.org/zap"

	"mikmongo/internal/model"
	"mikmongo/internal/repository"
	"mikmongo/internal/service"
	"mikmongo/pkg/redis"
	"mikmongo/utils"

	handlerMikrotik "mikmongo/internal/handler/mikrotik"
)

const (
	// RouterOS is accessed via Docker port mapping on localhost
	routerOSHost = "localhost"
	routerOSPort = 8728
	routerOSUser = "admin"
	routerOSPass = "admin"

	redisMonitorAddr = "localhost:6380"
	influxURL        = "http://localhost:8181"
	redisMainAddr    = "localhost:6379"

	waitForCollection = 15 * time.Second
	dialTimeout       = 3 * time.Second

	// 32-byte key for test encryption
	testEncKey = "integration-test-key-32bytes!!"
)

// ---------------------------------------------------------------------------
// TestMain: skip everything when infrastructure is unreachable
// ---------------------------------------------------------------------------

func TestMain(m *testing.M) {
	if !isReachable(fmt.Sprintf("%s:%d", routerOSHost, routerOSPort)) {
		fmt.Println("SKIP: RouterOS not reachable at", routerOSHost)
		os.Exit(0)
	}
	if !isReachable(redisMonitorAddr) {
		fmt.Println("SKIP: Redis (monitor) not reachable at", redisMonitorAddr)
		os.Exit(0)
	}
	if !isReachable("localhost:8181") {
		fmt.Println("SKIP: InfluxDB not reachable at", influxURL)
		os.Exit(0)
	}

	os.Exit(m.Run())
}

func isReachable(addr string) bool {
	conn, err := net.DialTimeout("tcp", addr, dialTimeout)
	if err != nil {
		return false
	}
	conn.Close()
	return true
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// Compile-time check that fakeRouterRepo satisfies the interface.
var _ repository.RouterDeviceRepository = (*fakeRouterRepo)(nil)

type fakeRouterRepo struct {
	device model.MikrotikRouter
}

func newFakeRouterRepo(device model.MikrotikRouter) *fakeRouterRepo {
	return &fakeRouterRepo{device: device}
}

func (r *fakeRouterRepo) Create(_ context.Context, _ *model.MikrotikRouter) error { return nil }
func (r *fakeRouterRepo) GetByID(_ context.Context, _ uuid.UUID) (*model.MikrotikRouter, error) {
	return &r.device, nil
}
func (r *fakeRouterRepo) GetActive(_ context.Context) ([]model.MikrotikRouter, error) {
	return []model.MikrotikRouter{r.device}, nil
}
func (r *fakeRouterRepo) Update(_ context.Context, _ *model.MikrotikRouter) error { return nil }
func (r *fakeRouterRepo) UpdateLastSync(_ context.Context, _ uuid.UUID) error {
	return nil
}
func (r *fakeRouterRepo) Delete(_ context.Context, _ uuid.UUID) error { return nil }
func (r *fakeRouterRepo) List(_ context.Context, _, _ int) ([]model.MikrotikRouter, error) {
	return []model.MikrotikRouter{r.device}, nil
}

func newMonitorRedisClient(t *testing.T) *goredis.Client {
	t.Helper()
	rdb := goredis.NewClient(&goredis.Options{Addr: redisMonitorAddr})
	ctx, cancel := context.WithTimeout(context.Background(), dialTimeout)
	defer cancel()
	if err := rdb.Ping(ctx).Err(); err != nil {
		t.Fatalf("redis monitor ping failed: %v", err)
	}
	return rdb
}

func testRouterDevice() model.MikrotikRouter {
	// Encrypt the password with our test key
	padded := make([]byte, 32)
	copy(padded, []byte(testEncKey))
	encPass, err := utils.Encrypt(padded, routerOSPass)
	if err != nil {
		panic(fmt.Sprintf("failed to encrypt test password: %v", err))
	}

	return model.MikrotikRouter{
		ID:                "00000000-0000-0000-0000-000000000001",
		Name:              "integration-test-router",
		Address:           routerOSHost,
		APIPort:           routerOSPort,
		Username:          routerOSUser,
		PasswordEncrypted: encPass,
		IsActive:          true,
		Status:            "online",
	}
}

func newCollectorService(t *testing.T) *service.CollectorService {
	t.Helper()
	logger := zap.NewNop()
	device := testRouterDevice()

	redisClient := redis.NewClient(redis.Options{Host: "localhost", Port: 6379})
	routerSvc := service.NewRouterService(
		newFakeRouterRepo(device),
		testEncKey,
		redisClient,
		logger,
	)

	return service.NewCollectorService(
		routerSvc,
		redisMonitorAddr,
		influxURL,
		"", // token
		"", // org
		"mikmongo-test",
		logger,
	)
}

// ---------------------------------------------------------------------------
// Test 1: RouterOS direct connectivity
// ---------------------------------------------------------------------------

func TestRouterOSConnection(t *testing.T) {
	cfg := client.Config{
		Host:     routerOSHost,
		Port:     routerOSPort,
		Username: routerOSUser,
		Password: routerOSPass,
		Timeout:  10 * time.Second,
	}

	c, err := client.New(cfg)
	if err != nil {
		t.Fatalf("failed to connect to RouterOS: %v", err)
	}
	defer c.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	results, err := c.RunRaw(ctx, []string{"/system/resource/print"})
	if err != nil {
		t.Fatalf("RunRaw /system/resource/print failed: %v", err)
	}

	if len(results) == 0 {
		t.Fatal("expected at least one result from /system/resource/print")
	}

	if _, ok := results[0]["platform"]; !ok {
		t.Errorf("expected 'platform' field in system resource, got keys: %v", mapKeys(results[0]))
	}

	t.Logf("RouterOS platform: %s, version: %s", results[0]["platform"], results[0]["version"])
}

// ---------------------------------------------------------------------------
// Test 2: CollectorService Start/Stop
// ---------------------------------------------------------------------------

func TestCollectorServiceStartStop(t *testing.T) {
	collectorSvc := newCollectorService(t)

	ctx := context.Background()
	if err := collectorSvc.Start(ctx); err != nil {
		t.Fatalf("CollectorService.Start failed: %v", err)
	}

	t.Log("collector started, waiting for data collection...")
	time.Sleep(waitForCollection)

	status := collectorSvc.Status()
	if len(status) == 0 {
		t.Error("expected at least 1 router in collector status, got 0")
	}
	for id, s := range status {
		t.Logf("router %s: specs=%d uptime=%s", id, s.SpecCount, s.Uptime)
	}

	collectorSvc.Stop()
	t.Log("collector stopped cleanly")
}

// ---------------------------------------------------------------------------
// Test 3: Cached data in Redis after collection
// ---------------------------------------------------------------------------

func TestCachedDataAfterCollection(t *testing.T) {
	collectorSvc := newCollectorService(t)

	ctx := context.Background()
	if err := collectorSvc.Start(ctx); err != nil {
		t.Fatalf("CollectorService.Start failed: %v", err)
	}
	defer collectorSvc.Stop()

	t.Log("waiting for collection to populate Redis...")
	time.Sleep(waitForCollection)

	rdb := newMonitorRedisClient(t)
	defer rdb.Close()

	routerID := "00000000-0000-0000-0000-000000000001"

	// Check various key patterns the collector may write
	keysToCheck := []string{
		fmt.Sprintf("%s:ppp:active", routerID),
		fmt.Sprintf("%s:interface:traffic", routerID),
		fmt.Sprintf("%s:hotspot:active", routerID),
		fmt.Sprintf("%s:queue:stats", routerID),
		fmt.Sprintf("%s:router:log", routerID),
	}

	foundKeys := 0
	for _, key := range keysToCheck {
		result, hErr := rdb.HGetAll(ctx, key).Result()
		if hErr == nil && len(result) > 0 {
			t.Logf("found hash key %s with %d entries", key, len(result))
			foundKeys++
			continue
		}

		xlen, xErr := rdb.XLen(ctx, key).Result()
		if xErr == nil && xlen > 0 {
			t.Logf("found stream key %s with %d entries", key, xlen)
			foundKeys++
			continue
		}
	}

	if foundKeys == 0 {
		// Scan for any keys with the routerID prefix
		keys, _, scanErr := rdb.Scan(ctx, 0, fmt.Sprintf("*%s*", routerID), 100).Result()
		if scanErr != nil {
			t.Logf("scan error: %v", scanErr)
		}
		if len(keys) > 0 {
			t.Logf("found %d keys matching routerID pattern: %v", len(keys), keys)
			foundKeys = len(keys)
		}

		// Also try mikrotik-prefixed keys
		keys2, _, scanErr2 := rdb.Scan(ctx, 0, "mikrotik*", 100).Result()
		if scanErr2 == nil && len(keys2) > 0 {
			t.Logf("found %d keys with 'mikrotik' prefix: %v", len(keys2), keys2)
			foundKeys += len(keys2)
		}
	}

	if foundKeys == 0 {
		t.Error("expected at least some cached data in Redis after collection, found none")
	}
}

// ---------------------------------------------------------------------------
// Test 4: Handler endpoints with real Redis
// ---------------------------------------------------------------------------

func TestCollectorHandlerEndpoints(t *testing.T) {
	collectorSvc := newCollectorService(t)

	ctx := context.Background()
	if err := collectorSvc.Start(ctx); err != nil {
		t.Fatalf("CollectorService.Start failed: %v", err)
	}
	defer collectorSvc.Stop()

	t.Log("waiting for collection...")
	time.Sleep(waitForCollection)

	gin.SetMode(gin.TestMode)
	rdb := newMonitorRedisClient(t)
	defer rdb.Close()

	cachedHandler := handlerMikrotik.NewCachedHandler(rdb)
	logHandler := handlerMikrotik.NewLogHandler(rdb)
	statusHandler := &handlerMikrotik.CollectorStatusHandler{
		CollectorSvc: collectorSvc,
	}

	router := gin.New()
	router.GET("/cached/ppp-active/:router_id", cachedHandler.GetCachedPPPActive)
	router.GET("/cached/hotspot-active/:router_id", cachedHandler.GetCachedHotspotActive)
	router.GET("/cached/ppp-secrets/:router_id", cachedHandler.GetCachedPPPSecrets)
	router.GET("/cached/queue-stats/:router_id", cachedHandler.GetCachedQueueStats)
	router.GET("/logs/:router_id", logHandler.GetRouterLogs)
	router.GET("/collector/status", statusHandler.GetStatus)

	routerID := "00000000-0000-0000-0000-000000000001"

	t.Run("cached/ppp-active", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodGet, fmt.Sprintf("/cached/ppp-active/%s", routerID), nil)
		router.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("expected 200, got %d: %s", w.Code, w.Body.String())
		}
	})

	t.Run("logs", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodGet, fmt.Sprintf("/logs/%s", routerID), nil)
		router.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("expected 200, got %d: %s", w.Code, w.Body.String())
		}
	})

	t.Run("collector/status", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodGet, "/collector/status", nil)
		router.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("expected 200, got %d: %s", w.Code, w.Body.String())
		}

		var body map[string]interface{}
		if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
			t.Fatalf("failed to parse status body: %v", err)
		}

		data, ok := body["data"]
		if !ok {
			t.Error("expected 'data' field in collector status response")
		} else {
			statusMap, isMap := data.(map[string]interface{})
			if !isMap {
				t.Logf("status data type: %T, value: %v", data, data)
			} else if len(statusMap) == 0 {
				t.Error("expected at least 1 router in status data")
			} else {
				t.Logf("collector status contains %d routers", len(statusMap))
			}
		}
	})
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func mapKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}
