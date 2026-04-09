// supervisor.go - CollectorSupervisor untuk monitoring dan auto-restart
package collector

import (
	"log"
	"sync"
	"time"

	"github.com/Butterfly-Student/go-ros/client"
	"mikmongo/internal/collector/pipeline/operational"
	"mikmongo/internal/collector/pipeline/time_series"
	"mikmongo/internal/collector/pool"
	"mikmongo/internal/collector/writer"
)

// Supervisor monitors collectors dan auto-restart jika failure
type Supervisor struct {
	routerID          string
	routerCfg         client.Config
	
	// Pools
	timeSeriesPool    *pool.ConnPool
	operationalPool   *pool.ConnPool
	
	// Writers
	batchWriter       *writer.BatchWriter
	
	// Specs
	timeSeriesSpecs   []CommandSpec
	operationalSpecs  []CommandSpec
	
	// Collectors
	tsCollector       *time_series.Collector
	tier2Collector    *operational.Tier2Collector
	tier3Collector    *operational.Tier3Collector
	
	// Control
	stopCh            chan struct{}
	wg                sync.WaitGroup
	mu                sync.RWMutex
	
	// Config
	healthCheckInterval time.Duration
	restartDelay        time.Duration
}

// SupervisorConfig untuk supervisor configuration
type SupervisorConfig struct {
	HealthCheckInterval time.Duration
	RestartDelay        time.Duration
}

// DefaultSupervisorConfig returns default config
func DefaultSupervisorConfig() SupervisorConfig {
	return SupervisorConfig{
		HealthCheckInterval: 30 * time.Second,
		RestartDelay:        5 * time.Second,
	}
}

// NewSupervisor creates new supervisor
func NewSupervisor(
	routerID string,
	routerCfg client.Config,
	timeSeriesSpecs, operationalSpecs []CommandSpec,
	batchWriter *writer.BatchWriter,
	config SupervisorConfig,
) (*Supervisor, error) {
	
	// Create pools
	tsPool, err := pool.NewConnPool(routerCfg, pool.DefaultConfig(pool.PoolTimeSeries))
	if err != nil {
		return nil, err
	}
	
	opPool, err := pool.NewConnPool(routerCfg, pool.DefaultConfig(pool.PoolOperational))
	if err != nil {
		tsPool.Close()
		return nil, err
	}
	
	return &Supervisor{
		routerID:            routerID,
		routerCfg:           routerCfg,
		timeSeriesPool:      tsPool,
		operationalPool:     opPool,
		batchWriter:         batchWriter,
		timeSeriesSpecs:     timeSeriesSpecs,
		operationalSpecs:    operationalSpecs,
		stopCh:              make(chan struct{}),
		healthCheckInterval: config.HealthCheckInterval,
		restartDelay:        config.RestartDelay,
	}, nil
}

// Start starts supervisor dan all collectors
func (s *Supervisor) Start() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	log.Printf("[Supervisor %s] Starting...", s.routerID)
	
	// Start time-series collector (Pipeline A)
	if len(s.timeSeriesSpecs) > 0 {
		s.tsCollector = time_series.NewCollector(
			s.routerID,
			s.timeSeriesPool,
			s.timeSeriesSpecs,
			s.batchWriter,
		)
		s.tsCollector.Start()
	}
	
	// Filter operational specs by tier
	tier2Specs := FilterByTier(s.operationalSpecs, Tier2)
	tier3Specs := FilterByTier(s.operationalSpecs, Tier3)
	
	// Start Tier 2 collector (follow=yes)
	if len(tier2Specs) > 0 {
		s.tier2Collector = operational.NewTier2Collector(
			s.routerID,
			s.operationalPool,
			tier2Specs,
			s.batchWriter,
		)
		s.tier2Collector.Start()
	}
	
	// Start Tier 3 collector (ticker)
	if len(tier3Specs) > 0 {
		s.tier3Collector = operational.NewTier3Collector(
			s.routerID,
			s.operationalPool,
			tier3Specs,
			s.batchWriter,
		)
		s.tier3Collector.Start()
	}
	
	// Start health check monitor
	s.wg.Add(1)
	go s.healthCheckLoop()
	
	log.Printf("[Supervisor %s] Started all collectors", s.routerID)
	return nil
}

// Stop stops supervisor dan all collectors
func (s *Supervisor) Stop() {
	close(s.stopCh)
	s.wg.Wait()
	
	s.mu.Lock()
	defer s.mu.Unlock()
	
	// Stop collectors
	if s.tsCollector != nil {
		s.tsCollector.Stop()
	}
	if s.tier2Collector != nil {
		s.tier2Collector.Stop()
	}
	if s.tier3Collector != nil {
		s.tier3Collector.Stop()
	}
	
	// Close pools
	s.timeSeriesPool.Close()
	s.operationalPool.Close()
	
	log.Printf("[Supervisor %s] Stopped", s.routerID)
}

// healthCheckLoop monitors pool health dan restart jika perlu
func (s *Supervisor) healthCheckLoop() {
	defer s.wg.Done()
	
	ticker := time.NewTicker(s.healthCheckInterval)
	defer ticker.Stop()
	
	for {
		select {
		case <-s.stopCh:
			return
		case <-ticker.C:
			s.performHealthCheck()
		}
	}
}

// performHealthCheck checks pool health
func (s *Supervisor) performHealthCheck() {
	// Check time-series pool
	tsStats := s.timeSeriesPool.GetStats()
	if tsStats.InUse == tsStats.Total {
		log.Printf("[Supervisor %s] TimeSeries pool saturated: %s", s.routerID, tsStats.String())
	}
	
	// Check operational pool
	opStats := s.operationalPool.GetStats()
	if opStats.InUse == opStats.Total {
		log.Printf("[Supervisor %s] Operational pool saturated: %s", s.routerID, opStats.String())
	}
	
	// Health check connections
	s.timeSeriesPool.HealthCheck()
	s.operationalPool.HealthCheck()
}

// RestartCollector restarts a specific collector (public untuk manual restart)
func (s *Supervisor) RestartCollector(pipeline string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	log.Printf("[Supervisor %s] Restarting %s collector...", s.routerID, pipeline)
	
	switch pipeline {
	case "time_series":
		if s.tsCollector != nil {
			s.tsCollector.Stop()
		}
		// Recycle pool dan create new collector
		s.timeSeriesPool.Close()
		newPool, err := pool.NewConnPool(s.routerCfg, pool.DefaultConfig(pool.PoolTimeSeries))
		if err != nil {
			return err
		}
		s.timeSeriesPool = newPool
		s.tsCollector = time_series.NewCollector(s.routerID, newPool, s.timeSeriesSpecs, s.batchWriter)
		s.tsCollector.Start()
		
	case "operational":
		if s.tier2Collector != nil {
			s.tier2Collector.Stop()
		}
		if s.tier3Collector != nil {
			s.tier3Collector.Stop()
		}
		s.operationalPool.Close()
		newPool, err := pool.NewConnPool(s.routerCfg, pool.DefaultConfig(pool.PoolOperational))
		if err != nil {
			return err
		}
		s.operationalPool = newPool
		
		tier2Specs := FilterByTier(s.operationalSpecs, Tier2)
		tier3Specs := FilterByTier(s.operationalSpecs, Tier3)
		
		if len(tier2Specs) > 0 {
			s.tier2Collector = operational.NewTier2Collector(s.routerID, newPool, tier2Specs, s.batchWriter)
			s.tier2Collector.Start()
		}
		if len(tier3Specs) > 0 {
			s.tier3Collector = operational.NewTier3Collector(s.routerID, newPool, tier3Specs, s.batchWriter)
			s.tier3Collector.Start()
		}
	}
	
	return nil
}

// GetStats returns supervisor stats
func (s *Supervisor) GetStats() map[string]interface{} {
	s.mu.RLock()
	defer s.mu.RUnlock()
	
	return map[string]interface{}{
		"router_id":             s.routerID,
		"time_series_pool":      s.timeSeriesPool.GetStats(),
		"operational_pool":      s.operationalPool.GetStats(),
		"time_series_specs":     len(s.timeSeriesSpecs),
		"operational_specs":     len(s.operationalSpecs),
	}
}
