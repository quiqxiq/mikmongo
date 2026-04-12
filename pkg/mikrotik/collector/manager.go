// manager.go - Main Manager untuk mengelola semua router collectors
package collector

import (
	"log"
	"sync"

	"github.com/Butterfly-Student/go-ros/client"
	"github.com/Butterfly-Student/go-ros/collector/pipeline/ondemand"
	"github.com/Butterfly-Student/go-ros/collector/pool"
	"github.com/Butterfly-Student/go-ros/collector/writer"
	"github.com/Butterfly-Student/go-ros/spec"
)

// Config untuk Manager
type ManagerConfig struct {
	InfluxConfig writer.InfluxConfig
	RedisConfig  writer.RedisConfig
	BatchConfig  writer.Config
}

// Manager manages all router supervisors dan on-demand runners
type Manager struct {
	config           ManagerConfig
	supervisors      map[string]*Supervisor
	runners          map[string]*ondemand.Runner
	onDemandPools    map[string]*pool.ConnPool
	batchWriter      *writer.BatchWriter
	influxHandler    *writer.InfluxHandler
	redisHandler     *writer.RedisHandler
	mu               sync.RWMutex
}

// NewManager creates new manager
func NewManager(cfg ManagerConfig) (*Manager, error) {
	// Create handlers
	influxHandler, err := writer.NewInfluxHandler(cfg.InfluxConfig)
	if err != nil {
		return nil, err
	}

	redisHandler, err := writer.NewRedisHandler(cfg.RedisConfig)
	if err != nil {
		influxHandler.Close()
		return nil, err
	}

	// Create shared batch writer
	batchWriter := writer.NewBatchWriter(cfg.BatchConfig, influxHandler, redisHandler)

	return &Manager{
		config:        cfg,
		supervisors:   make(map[string]*Supervisor),
		runners:       make(map[string]*ondemand.Runner),
		onDemandPools: make(map[string]*pool.ConnPool),
		batchWriter:   batchWriter,
		influxHandler: influxHandler,
		redisHandler:  redisHandler,
	}, nil
}

// AddRouter menambahkan router baru untuk di-manage
func (m *Manager) AddRouter(
	routerID string,
	routerCfg client.Config,
	timeSeriesSpecs []spec.CommandSpec,
	operationalSpecs []spec.CommandSpec,
) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.supervisors[routerID]; exists {
		return nil // Already exists
	}

	// Create supervisor untuk Pipeline A & B
	supervisor, err := NewSupervisor(
		routerID,
		routerCfg,
		timeSeriesSpecs,
		operationalSpecs,
		m.batchWriter,
		DefaultSupervisorConfig(),
	)
	if err != nil {
		return err
	}

	// Create on-demand pool untuk Pipeline C
	onDemandPool, err := pool.NewConnPool(routerCfg, pool.DefaultConfig(pool.PoolOnDemand))
	if err != nil {
		supervisor.Stop()
		return err
	}

	// Create runner untuk Pipeline C
	runner := ondemand.NewRunner(routerID, onDemandPool, m.redisHandler)

	m.supervisors[routerID] = supervisor
	m.runners[routerID] = runner
	m.onDemandPools[routerID] = onDemandPool

	log.Printf("[Manager] Added router: %s", routerID)
	return nil
}

// StartRouter starts collection untuk satu router
func (m *Manager) StartRouter(routerID string) error {
	m.mu.RLock()
	supervisor, exists := m.supervisors[routerID]
	m.mu.RUnlock()

	if !exists {
		return nil
	}

	return supervisor.Start()
}

// StartAll starts all routers
func (m *Manager) StartAll() {
	m.mu.RLock()
	defer m.mu.RUnlock()

	m.batchWriter.Start()

	for id, supervisor := range m.supervisors {
		if err := supervisor.Start(); err != nil {
			log.Printf("[Manager] Failed to start %s: %v", id, err)
		}
	}

	log.Println("[Manager] Started all routers")
}

// StopRouter stops collection untuk satu router
func (m *Manager) StopRouter(routerID string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if supervisor, exists := m.supervisors[routerID]; exists {
		supervisor.Stop()
	}

	if pool, exists := m.onDemandPools[routerID]; exists {
		pool.Close()
	}

	log.Printf("[Manager] Stopped router: %s", routerID)
}

// StopAll stops semua routers
func (m *Manager) StopAll() {
	m.mu.Lock()
	defer m.mu.Unlock()

	for id := range m.supervisors {
		m.supervisors[id].Stop()
	}

	for id := range m.onDemandPools {
		m.onDemandPools[id].Close()
	}

	m.batchWriter.Stop()
	m.influxHandler.Close()
	m.redisHandler.Close()

	log.Println("[Manager] Stopped all routers")
}

// RemoveRouter removes router dari manager
func (m *Manager) RemoveRouter(routerID string) {
	m.StopRouter(routerID)

	m.mu.Lock()
	defer m.mu.Unlock()

	delete(m.supervisors, routerID)
	delete(m.runners, routerID)
	delete(m.onDemandPools, routerID)

	log.Printf("[Manager] Removed router: %s", routerID)
}

// GetRunner returns on-demand runner untuk router
func (m *Manager) GetRunner(routerID string) *ondemand.Runner {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.runners[routerID]
}

// GetRedisHandler returns redis handler untuk queries
func (m *Manager) GetRedisHandler() *writer.RedisHandler {
	return m.redisHandler
}

// GetStats returns manager statistics
func (m *Manager) GetStats() map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()

	stats := map[string]interface{}{
		"router_count": len(m.supervisors),
		"routers":      make(map[string]interface{}),
	}

	routerStats := make(map[string]interface{})
	for id, supervisor := range m.supervisors {
		routerStats[id] = supervisor.GetStats()
	}
	stats["routers"] = routerStats

	return stats
}
