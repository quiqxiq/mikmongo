// pipeline/operational/tier3_collector.go - Pipeline B Tier 3: Run() + ticker → Redis
package operational

import (
	"context"
	"log"
	"sync"
	"time"

	"github.com/Butterfly-Student/go-ros/collector/pool"
	"github.com/Butterfly-Student/go-ros/collector/writer"
	mtspec "github.com/Butterfly-Student/go-ros/spec"
)

// Tier3Collector handles static data collection dengan ticker
type Tier3Collector struct {
	routerID    string
	pool        *pool.ConnPool
	specs       []mtspec.CommandSpec
	batchWriter *writer.BatchWriter
	
	// Control
	stopCh      chan struct{}
	wg          sync.WaitGroup
}

// NewTier3Collector creates new Tier 3 collector
func NewTier3Collector(routerID string, p *pool.ConnPool, specs []mtspec.CommandSpec, bw *writer.BatchWriter) *Tier3Collector {
	return &Tier3Collector{
		routerID:    routerID,
		pool:        p,
		specs:       specs,
		batchWriter: bw,
		stopCh:      make(chan struct{}),
	}
}

// Start starts the collector
func (c *Tier3Collector) Start() {
	log.Printf("[Tier3Collector %s] Starting with %d specs", c.routerID, len(c.specs))
	
	// Start tickers untuk setiap spec
	for _, spec := range c.specs {
		c.wg.Add(1)
		go c.runTicker(spec)
	}
}

// Stop stops the collector
func (c *Tier3Collector) Stop() {
	close(c.stopCh)
	c.wg.Wait()
	log.Printf("[Tier3Collector %s] Stopped", c.routerID)
}

// runTicker runs ticker untuk satu spec
func (c *Tier3Collector) runTicker(spec mtspec.CommandSpec) {
	defer c.wg.Done()
	
	// Initial fetch
	c.fetchAndCache(spec)
	
	// Setup ticker dengan interval dari spec
	interval := spec.Interval
	if interval < 10*time.Second {
		interval = 10 * time.Second // Minimum 10s
	}
	
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	
	log.Printf("[Tier3Collector %s] Ticker started for %s (interval: %v)", 
		c.routerID, spec.Name, interval)
	
	for {
		select {
		case <-c.stopCh:
			return
		case <-ticker.C:
			c.fetchAndCache(spec)
		}
	}
}

// fetchAndCache fetches data dari RouterOS dan cache ke Redis
func (c *Tier3Collector) fetchAndCache(spec mtspec.CommandSpec) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	
	// Acquire connection dari pool (Tier 3 pakai 1 slot)
	conn, err := c.pool.Acquire()
	if err != nil {
		log.Printf("[Tier3Collector %s] Failed to acquire connection: %v", c.routerID, err)
		return
	}
	
	// Run command (tidak pakai follow=yes)
	results, err := conn.Client.RunRaw(ctx, spec.Args)
	conn.RawClient.Close()
	
	if err != nil {
		log.Printf("[Tier3Collector %s] Fetch error for %s: %v", c.routerID, spec.Name, err)
		return
	}
	
	// Cache setiap row ke Redis dengan TTL
	for _, data := range results {
		keyField := spec.KeyField
		if keyField == "" {
			continue
		}
		
		keyValue := data[keyField]
		if keyValue == "" {
			continue
		}
		
		// Send ke BatchWriter
		item := writer.WriteItem{
			Mode:     writer.ModeRedis,
			RouterID: c.routerID,
			Key:      spec.RedisKey,
			Field:    keyValue,
			Value:    data,
			TTL:      spec.TTL,
		}
		
		c.batchWriter.Write(item)
	}
	
	log.Printf("[Tier3Collector %s] Cached %d items for %s", c.routerID, len(results), spec.Name)
}
