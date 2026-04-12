// pool/pool.go - Connection Pool untuk 3 Pipeline
package pool

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/Butterfly-Student/go-ros/client"
	"github.com/Butterfly-Student/go-ros"
)

// PoolType identifies which pipeline this pool serves
type PoolType string

const (
	PoolTimeSeries  PoolType = "time_series"  // Pipeline A - agresif
	PoolOperational PoolType = "operational"  // Pipeline B - follow=yes
	PoolOnDemand    PoolType = "on_demand"    // Pipeline C - direct
)

// Config untuk pool
 type Config struct {
	Type        PoolType
	MaxConns    int           // Maximum connections
	MaxCommands int           // Max commands per connection (RouterOS limit ~25)
	IdleTimeout time.Duration // Connection idle timeout
	RetryDelay  time.Duration // Retry delay on failure
}

// DefaultConfigs untuk masing-masing pipeline
func DefaultConfig(poolType PoolType) Config {
	switch poolType {
	case PoolTimeSeries:
		// Pipeline A: Banyak koneksi untuk concurrent metrics
		return Config{
			Type:        PoolTimeSeries,
			MaxConns:    5,
			MaxCommands: 20,
			IdleTimeout: 30 * time.Second,
			RetryDelay:  5 * time.Second,
		}
	case PoolOperational:
		// Pipeline B: Moderate untuk follow=yes listeners
		return Config{
			Type:        PoolOperational,
			MaxConns:    5,
			MaxCommands: 10, // Lebih sedikit karena follow=yes memakan 1 slot permanen
			IdleTimeout: 60 * time.Second,
			RetryDelay:  5 * time.Second,
		}
	case PoolOnDemand:
		// Pipeline C: Minimal, on-demand only
		return Config{
			Type:        PoolOnDemand,
			MaxConns:    2,
			MaxCommands: 25,
			IdleTimeout: 10 * time.Second,
			RetryDelay:  3 * time.Second,
		}
	default:
		return Config{
			Type:        poolType,
			MaxConns:    3,
			MaxCommands: 20,
			IdleTimeout: 30 * time.Second,
			RetryDelay:  5 * time.Second,
		}
	}
}

// PoolConn adalah wrapper connection dengan metadata
type PoolConn struct {
	ID        string
	Client    *mikrotik.Client
	RawClient *client.Client
	InUse     bool
	CommandCount int
	LastUsed  time.Time
	mu        sync.RWMutex
}

// IsHealthy checks if connection is still alive
func (pc *PoolConn) IsHealthy() bool {
	pc.mu.RLock()
	defer pc.mu.RUnlock()
	
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	
	// Try a simple ping
	_, err := pc.RawClient.RunContext(ctx, "/system/identity/print")
	return err == nil
}

// CanAcceptCommand checks if connection can accept more commands
func (pc *PoolConn) CanAcceptCommand(maxCommands int) bool {
	pc.mu.RLock()
	defer pc.mu.RUnlock()
	return pc.CommandCount < maxCommands && !pc.InUse
}

// MarkInUse marks connection as in-use
func (pc *PoolConn) MarkInUse() {
	pc.mu.Lock()
	defer pc.mu.Unlock()
	pc.InUse = true
	pc.LastUsed = time.Now()
}

// MarkAvailable marks connection as available
func (pc *PoolConn) MarkAvailable() {
	pc.mu.Lock()
	defer pc.mu.Unlock()
	pc.InUse = false
	pc.LastUsed = time.Now()
}

// IncrementCommand increases command count
func (pc *PoolConn) IncrementCommand() {
	pc.mu.Lock()
	defer pc.mu.Unlock()
	pc.CommandCount++
}

// ConnPool manages connections untuk satu pipeline
type ConnPool struct {
	config    Config
	routerCfg client.Config
	conns     []*PoolConn
	mu        sync.RWMutex
	closed    bool
}

// NewConnPool creates new connection pool
func NewConnPool(routerCfg client.Config, config Config) (*ConnPool, error) {
	pool := &ConnPool{
		config:    config,
		routerCfg: routerCfg,
		conns:     make([]*PoolConn, 0, config.MaxConns),
	}
	
	// Initialize minimum connections
	for i := 0; i < config.MaxConns; i++ {
		conn, err := pool.createConnection(i)
		if err != nil {
			return nil, fmt.Errorf("failed to create connection %d: %w", i, err)
		}
		pool.conns = append(pool.conns, conn)
	}
	
	log.Printf("[Pool %s] Created with %d connections", config.Type, len(pool.conns))
	return pool, nil
}

// createConnection creates a new pool connection
func (p *ConnPool) createConnection(index int) (*PoolConn, error) {
	c, err := client.New(p.routerCfg)
	if err != nil {
		return nil, err
	}
	
	return &PoolConn{
		ID:        fmt.Sprintf("%s-conn-%d", p.config.Type, index),
		Client:    mikrotik.NewClientFromConnection(c),
		RawClient: c,
		InUse:     false,
		LastUsed:  time.Now(),
	}, nil
}

// Acquire gets an available connection
func (p *ConnPool) Acquire() (*PoolConn, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	
	if p.closed {
		return nil, fmt.Errorf("pool is closed")
	}
	
	// Find available connection
	for _, conn := range p.conns {
		if !conn.InUse && conn.CanAcceptCommand(p.config.MaxCommands) {
			conn.MarkInUse()
			return conn, nil
		}
	}
	
	// All connections busy - this shouldn't happen if sized correctly
	return nil, fmt.Errorf("no available connections in pool")
}

// Release returns connection to pool
func (p *ConnPool) Release(conn *PoolConn) {
	conn.MarkAvailable()
}

// RecycleConnection creates new connection to replace unhealthy one
func (p *ConnPool) RecycleConnection(conn *PoolConn) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	
	// Close old connection
	conn.RawClient.Close()
	
	// Find and replace
	for i, c := range p.conns {
		if c.ID == conn.ID {
			newConn, err := p.createConnection(i)
			if err != nil {
				return err
			}
			p.conns[i] = newConn
			log.Printf("[Pool %s] Recycled connection %s", p.config.Type, conn.ID)
			return nil
		}
	}
	
	return fmt.Errorf("connection not found in pool")
}

// HealthCheck checks all connections and recycles unhealthy ones
func (p *ConnPool) HealthCheck() {
	p.mu.RLock()
	conns := make([]*PoolConn, len(p.conns))
	copy(conns, p.conns)
	p.mu.RUnlock()
	
	for _, conn := range conns {
		if !conn.IsHealthy() {
			log.Printf("[Pool %s] Unhealthy connection detected: %s", p.config.Type, conn.ID)
			if err := p.RecycleConnection(conn); err != nil {
				log.Printf("[Pool %s] Failed to recycle: %v", p.config.Type, err)
			}
		}
	}
}

// GetStats returns pool statistics
func (p *ConnPool) GetStats() PoolStats {
	p.mu.RLock()
	defer p.mu.RUnlock()
	
	stats := PoolStats{
		Type:      p.config.Type,
		Total:     len(p.conns),
		MaxConns:  p.config.MaxConns,
	}
	
	for _, conn := range p.conns {
		if conn.InUse {
			stats.InUse++
		} else {
			stats.Available++
		}
		stats.TotalCommands += conn.CommandCount
	}
	
	return stats
}

// Close closes all connections
func (p *ConnPool) Close() {
	p.mu.Lock()
	defer p.mu.Unlock()
	
	p.closed = true
	for _, conn := range p.conns {
		conn.RawClient.Close()
	}
	
	log.Printf("[Pool %s] Closed %d connections", p.config.Type, len(p.conns))
}

// PoolStats contains pool statistics
type PoolStats struct {
	Type          PoolType
	Total         int
	InUse         int
	Available     int
	MaxConns      int
	TotalCommands int
}

// String returns formatted stats
func (s PoolStats) String() string {
	return fmt.Sprintf("[%s] Total: %d, InUse: %d, Available: %d", 
		s.Type, s.Total, s.InUse, s.Available)
}
