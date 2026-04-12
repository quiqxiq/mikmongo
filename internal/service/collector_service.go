package service

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/Butterfly-Student/go-ros/collector"
	"github.com/Butterfly-Student/go-ros/collector/writer"
	"github.com/Butterfly-Student/go-ros/spec"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// RouterCollectorStatus represents the status of a single router's collector.
type RouterCollectorStatus struct {
	RouterID        string                 `json:"router_id"`
	SpecCount       int                    `json:"spec_count"`
	StartedAt       time.Time              `json:"started_at"`
	Uptime          string                 `json:"uptime"`
	Stats           map[string]interface{} `json:"stats,omitempty"`
	LastError       string                 `json:"last_error,omitempty"`
	LastErrorAt     *time.Time             `json:"last_error_at,omitempty"`
}

// CollectorService is the application-layer wrapper that ties
// the collector.Manager to RouterService for lifecycle management.
type CollectorService struct {
	routerSvc    *RouterService
	redisAddr    string
	influxURL    string
	influxToken  string
	influxOrg    string
	influxBucket string
	manager      *collector.Manager
	logger       *zap.Logger

	mu           sync.RWMutex
	startedAt    map[string]time.Time
	lastErrors   map[string]errorRecord
}

type errorRecord struct {
	message string
	at      time.Time
}

// NewCollectorService creates a new CollectorService.
func NewCollectorService(
	routerSvc *RouterService,
	redisAddr string,
	influxURL string,
	influxToken string,
	influxOrg string,
	influxBucket string,
	logger *zap.Logger,
) *CollectorService {
	return &CollectorService{
		routerSvc:    routerSvc,
		redisAddr:    redisAddr,
		influxURL:    influxURL,
		influxToken:  influxToken,
		influxOrg:    influxOrg,
		influxBucket: influxBucket,
		logger:       logger,
		startedAt:    make(map[string]time.Time),
		lastErrors:   make(map[string]errorRecord),
	}
}

// Start fetches all active routers, creates the collector.Manager,
// registers each router with DefaultSpecs, and starts collecting.
func (s *CollectorService) Start(ctx context.Context) error {
	cfg := collector.ManagerConfig{
		InfluxConfig: writer.InfluxConfig{
			URL:    s.influxURL,
			Token:  s.influxToken,
			Org:    s.influxOrg,
			Bucket: s.influxBucket,
		},
		RedisConfig: writer.RedisConfig{
			Addr:   s.redisAddr,
			DB:     0,
			Prefix: "mikrotik",
		},
	}

	mgr, err := collector.NewManager(cfg)
	if err != nil {
		return fmt.Errorf("failed to create collector manager: %w", err)
	}
	s.manager = mgr

	// Fetch all active routers
	devices, err := s.routerSvc.routerRepo.GetActive(ctx)
	if err != nil {
		s.manager.StopAll()
		return fmt.Errorf("failed to fetch active routers: %w", err)
	}

	allSpecs := spec.DefaultSpecs()
	tsSpecs := spec.FilterByPipeline(allSpecs, spec.PipelineTimeSeries)
	opSpecs := spec.FilterByPipeline(allSpecs, spec.PipelineOperational)

	for _, device := range devices {
		deviceUUID, parseErr := uuid.Parse(device.ID)
		if parseErr != nil {
			s.logger.Warn("invalid router UUID, skipping",
				zap.String("router_id", device.ID),
				zap.Error(parseErr),
			)
			continue
		}

		routerCfg, cfgErr := s.routerSvc.getRouterConfig(ctx, deviceUUID)
		if cfgErr != nil {
			s.logger.Warn("failed to get router config, skipping",
				zap.String("router_id", device.ID),
				zap.Error(cfgErr),
			)
			s.recordError(device.ID, cfgErr)
			continue
		}

		if addErr := s.manager.AddRouter(device.ID, routerCfg, tsSpecs, opSpecs); addErr != nil {
			s.logger.Warn("failed to add router to collector",
				zap.String("router_id", device.ID),
				zap.Error(addErr),
			)
			s.recordError(device.ID, addErr)
			continue
		}

		s.mu.Lock()
		s.startedAt[device.ID] = time.Now()
		s.mu.Unlock()
	}

	s.manager.StartAll()
	s.logger.Info("collector service started", zap.Int("router_count", len(devices)))
	return nil
}

// Stop performs a graceful shutdown of all collectors.
func (s *CollectorService) Stop() {
	if s.manager == nil {
		return
	}
	s.manager.StopAll()
	s.logger.Info("collector service stopped")
}

// RegisterRouter registers a single router for collection.
// Called when a new router is created or activated.
func (s *CollectorService) RegisterRouter(ctx context.Context, routerID uuid.UUID) error {
	if s.manager == nil {
		return fmt.Errorf("collector manager not initialized")
	}

	routerCfg, err := s.routerSvc.getRouterConfig(ctx, routerID)
	if err != nil {
		return fmt.Errorf("failed to get router config: %w", err)
	}

	allSpecs := spec.DefaultSpecs()
	tsSpecs := spec.FilterByPipeline(allSpecs, spec.PipelineTimeSeries)
	opSpecs := spec.FilterByPipeline(allSpecs, spec.PipelineOperational)

	if err := s.manager.AddRouter(routerID.String(), routerCfg, tsSpecs, opSpecs); err != nil {
		return fmt.Errorf("failed to add router: %w", err)
	}

	if err := s.manager.StartRouter(routerID.String()); err != nil {
		return fmt.Errorf("failed to start router collector: %w", err)
	}

	s.mu.Lock()
	s.startedAt[routerID.String()] = time.Now()
	s.mu.Unlock()

	s.logger.Info("registered router for collection", zap.String("router_id", routerID.String()))
	return nil
}

// UnregisterRouter stops collecting for a router and removes it.
// Called when a router is deleted or deactivated.
func (s *CollectorService) UnregisterRouter(routerID uuid.UUID) {
	if s.manager == nil {
		return
	}

	s.manager.RemoveRouter(routerID.String())

	s.mu.Lock()
	delete(s.startedAt, routerID.String())
	delete(s.lastErrors, routerID.String())
	s.mu.Unlock()

	s.logger.Info("unregistered router from collection", zap.String("router_id", routerID.String()))
}

// Status returns the status of all collectors keyed by router ID.
func (s *CollectorService) Status() map[string]RouterCollectorStatus {
	if s.manager == nil {
		return nil
	}

	managerStats := s.manager.GetStats()
	routerStatsRaw, _ := managerStats["routers"].(map[string]interface{})

	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make(map[string]RouterCollectorStatus, len(routerStatsRaw))
	now := time.Now()

	for routerID, raw := range routerStatsRaw {
		status := RouterCollectorStatus{
			RouterID: routerID,
		}

		if stats, ok := raw.(map[string]interface{}); ok {
			status.Stats = stats
			if tsCount, ok := stats["time_series_specs"].(int); ok {
				status.SpecCount += tsCount
			}
			if opCount, ok := stats["operational_specs"].(int); ok {
				status.SpecCount += opCount
			}
		}

		if started, ok := s.startedAt[routerID]; ok {
			status.StartedAt = started
			status.Uptime = now.Sub(started).Truncate(time.Second).String()
		}

		if rec, ok := s.lastErrors[routerID]; ok {
			status.LastError = rec.message
			status.LastErrorAt = &rec.at
		}

		result[routerID] = status
	}

	return result
}

// GetCachedData reads cached operational data from Redis for a router.
func (s *CollectorService) GetCachedData(ctx context.Context, routerID string, redisKey string) (map[string]string, error) {
	if s.manager == nil {
		return nil, fmt.Errorf("collector manager not initialized")
	}

	handler := s.manager.GetRedisHandler()
	if handler == nil {
		return nil, fmt.Errorf("redis handler not available")
	}

	data, err := handler.Get(ctx, routerID, redisKey, "")
	if err != nil {
		return nil, fmt.Errorf("failed to read cached data: %w", err)
	}

	return data, nil
}

func (s *CollectorService) recordError(routerID string, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.lastErrors[routerID] = errorRecord{
		message: err.Error(),
		at:      time.Now(),
	}
}
