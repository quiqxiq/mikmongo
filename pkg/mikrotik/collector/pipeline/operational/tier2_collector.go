// pipeline/operational/tier2_collector.go - Pipeline B Tier 2: follow=yes → Redis
package operational

import (
	"context"
	"log"
	"sync"
	"time"

	"github.com/Butterfly-Student/go-ros/collector/pool"
	"github.com/Butterfly-Student/go-ros/collector/writer"
	"github.com/Butterfly-Student/go-ros/spec"
)

// Tier2Collector handles event-driven state collection (follow=yes)
type Tier2Collector struct {
	routerID    string
	pool        *pool.ConnPool
	specs       []spec.CommandSpec
	batchWriter *writer.BatchWriter
	
	// Fan-in channel
	stateCh     chan StateEvent
	
	// Control
	stopCh      chan struct{}
	wg          sync.WaitGroup
}

// StateEvent adalah state change event
type StateEvent struct {
	RouterID  string
	Spec      spec.CommandSpec
	Key       string
	Data      map[string]string
	Timestamp time.Time
	IsDelete  bool // true jika item dihapus
}

// NewTier2Collector creates new Tier 2 collector
func NewTier2Collector(routerID string, p *pool.ConnPool, specs []spec.CommandSpec, bw *writer.BatchWriter) *Tier2Collector {
	return &Tier2Collector{
		routerID:    routerID,
		pool:        p,
		specs:       specs,
		batchWriter: bw,
		stateCh:     make(chan StateEvent, 500),
		stopCh:      make(chan struct{}),
	}
}

// Start starts the collector
func (c *Tier2Collector) Start() {
	log.Printf("[Tier2Collector %s] Starting with %d specs", c.routerID, len(c.specs))
	
	// Start fan-in processor
	c.wg.Add(1)
	go c.processStates()
	
	// Start collectors untuk setiap spec
	for _, spec := range c.specs {
		c.wg.Add(1)
		go c.collectSpec(spec)
	}
}

// Stop stops the collector
func (c *Tier2Collector) Stop() {
	close(c.stopCh)
	c.wg.Wait()
	close(c.stateCh)
	log.Printf("[Tier2Collector %s] Stopped", c.routerID)
}

// collectSpec collects state untuk satu spec dengan follow=yes
func (c *Tier2Collector) collectSpec(spec spec.CommandSpec) {
	defer c.wg.Done()
	
	// Acquire connection dari pool
	conn, err := c.pool.Acquire()
	if err != nil {
		log.Printf("[Tier2Collector %s] Failed to acquire connection: %v", c.routerID, err)
		return
	}
	defer c.pool.Release(conn)
	
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	
	// Create result channel
	resultCh := make(chan map[string]string, 100)
	
	// Start listening
	stopListen, err := conn.Client.ListenRaw(ctx, spec.Args, resultCh)
	if err != nil {
		log.Printf("[Tier2Collector %s] Failed to start listener for %s: %v", c.routerID, spec.Name, err)
		return
	}
	defer stopListen()
	
	conn.IncrementCommand()
	log.Printf("[Tier2Collector %s] Started collecting %s", c.routerID, spec.Name)
	
	for {
		select {
		case <-c.stopCh:
			return
		case <-ctx.Done():
			return
		case data := <-resultCh:
			if data == nil {
				continue
			}
			
			// Detect delete (empty value untuk key field)
			keyField := spec.KeyField
			keyValue := data[keyField]
			isDelete := false
			
			// Jika data hanya berisi key field tanpa value lain, itu adalah delete
			if len(data) == 1 && keyValue != "" {
				isDelete = true
			}
			
			// Send ke fan-in channel
			c.stateCh <- StateEvent{
				RouterID:  c.routerID,
				Spec:      spec,
				Key:       keyValue,
				Data:      data,
				Timestamp: time.Now(),
				IsDelete:  isDelete,
			}
		}
	}
}

// processStates adalah fan-in processor untuk state changes
func (c *Tier2Collector) processStates() {
	defer c.wg.Done()
	
	for {
		select {
		case <-c.stopCh:
			return
		case event, ok := <-c.stateCh:
			if !ok {
				return
			}
			
			// Convert ke WriteItem dan kirim ke BatchWriter
			item := c.eventToWriteItem(event)
			c.batchWriter.Write(item)
			
			if event.IsDelete {
				log.Printf("[Tier2Collector %s] Delete detected: %s/%s", 
					c.routerID, event.Spec.RedisKey, event.Key)
			}
		}
	}
}

// eventToWriteItem converts StateEvent ke WriteItem
func (c *Tier2Collector) eventToWriteItem(event StateEvent) writer.WriteItem {
	spec := event.Spec
	
	return writer.WriteItem{
		Mode:     writer.ModeRedis,
		RouterID: event.RouterID,
		Key:      spec.RedisKey,
		Field:    event.Key,
		Value:    event.Data,
		// Tier 2 tidak pakai TTL karena real-time state
	}
}
