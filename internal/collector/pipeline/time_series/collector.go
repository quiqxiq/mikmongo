// pipeline/time_series/collector.go - Pipeline A: Time-series → InfluxDB
package time_series

import (
	"context"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"mikmongo/internal/collector"
	"mikmongo/internal/collector/pool"
	"mikmongo/internal/collector/writer"
)

// Collector handles time-series metrics collection
type Collector struct {
	routerID    string
	pool        *pool.ConnPool
	specs       []collector.CommandSpec
	batchWriter *writer.BatchWriter
	
	// Fan-in channel untuk semua metrics
	metricsCh   chan MetricEvent
	
	// Control
	stopCh      chan struct{}
	wg          sync.WaitGroup
}

// MetricEvent adalah event dari RouterOS
type MetricEvent struct {
	RouterID    string
	Spec        collector.CommandSpec
	Timestamp   time.Time
	Data        map[string]string
}

// NewCollector creates new time-series collector
func NewCollector(routerID string, p *pool.ConnPool, specs []collector.CommandSpec, bw *writer.BatchWriter) *Collector {
	return &Collector{
		routerID:    routerID,
		pool:        p,
		specs:       specs,
		batchWriter: bw,
		metricsCh:   make(chan MetricEvent, 1000),
		stopCh:      make(chan struct{}),
	}
}

// Start starts the collector
func (c *Collector) Start() {
	log.Printf("[TimeSeriesCollector %s] Starting with %d specs", c.routerID, len(c.specs))
	
	// Start fan-in processor
	c.wg.Add(1)
	go c.processMetrics()
	
	// Start collectors untuk setiap spec
	for _, spec := range c.specs {
		c.wg.Add(1)
		go c.collectSpec(spec)
	}
}

// Stop stops the collector
func (c *Collector) Stop() {
	close(c.stopCh)
	c.wg.Wait()
	close(c.metricsCh)
	log.Printf("[TimeSeriesCollector %s] Stopped", c.routerID)
}

// collectSpec collects data untuk satu spec
func (c *Collector) collectSpec(spec collector.CommandSpec) {
	defer c.wg.Done()
	
	// Acquire connection dari pool
	conn, err := c.pool.Acquire()
	if err != nil {
		log.Printf("[TimeSeriesCollector %s] Failed to acquire connection: %v", c.routerID, err)
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
		log.Printf("[TimeSeriesCollector %s] Failed to start listener for %s: %v", c.routerID, spec.Name, err)
		return
	}
	defer stopListen()
	
	conn.IncrementCommand()
	log.Printf("[TimeSeriesCollector %s] Started collecting %s", c.routerID, spec.Name)
	
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
			
			// Send ke fan-in channel
			c.metricsCh <- MetricEvent{
				RouterID:  c.routerID,
				Spec:      spec,
				Timestamp: time.Now(),
				Data:      data,
			}
		}
	}
}

// processMetrics adalah fan-in processor
func (c *Collector) processMetrics() {
	defer c.wg.Done()
	
	for {
		select {
		case <-c.stopCh:
			return
		case event, ok := <-c.metricsCh:
			if !ok {
				return
			}
			
			// Convert ke WriteItem dan kirim ke BatchWriter
			item := c.eventToWriteItem(event)
			c.batchWriter.Write(item)
		}
	}
}

// eventToWriteItem converts MetricEvent ke WriteItem
func (c *Collector) eventToWriteItem(event MetricEvent) writer.WriteItem {
	spec := event.Spec
	
	// Parse tags dan fields dari data
	tags, fields := c.parseData(event.Data, spec.TagFields, spec.ValueFields)
	
	return writer.WriteItem{
		Mode:        writer.ModeInfluxDB,
		RouterID:    event.RouterID,
		Timestamp:   event.Timestamp,
		Measurement: spec.Measurement,
		Tags:        tags,
		Fields:      fields,
	}
}

// parseData extracts tags dan fields dari RouterOS data
func (c *Collector) parseData(data map[string]string, tagFields, valueFields []string) (tags, fields map[string]interface{}) {
	tags = make(map[string]interface{})
	fields = make(map[string]interface{})
	
	// Extract tags
	for _, tf := range tagFields {
		if v, ok := data[tf]; ok {
			tags[tf] = v
		}
	}
	
	// Extract fields
	for _, vf := range valueFields {
		if v, ok := data[vf]; ok {
			// Convert ke numeric jika memungkinkan
			fields[vf] = convertToNumeric(v)
		}
	}
	
	return tags, fields
}

// convertToNumeric mencoba convert string ke numeric
func convertToNumeric(s string) interface{} {
	// Coba int dulu
	var intVal int64
	if _, err := fmt.Sscanf(s, "%d", &intVal); err == nil {
		return intVal
	}
	
	// Coba float
	var floatVal float64
	if _, err := fmt.Sscanf(s, "%f", &floatVal); err == nil {
		return floatVal
	}
	
	// Return as string
	return s
}

// GetStats returns collector stats
func (c *Collector) GetStats() map[string]interface{} {
	pendingInflux, _ := c.batchWriter.GetStats()
	return map[string]interface{}{
		"router_id":      c.routerID,
		"specs_count":    len(c.specs),
		"pending_influx": pendingInflux,
	}
}

// formatFieldName normalizes field name untuk InfluxDB
func formatFieldName(name string) string {
	// Replace hyphen dengan underscore
	return strings.ReplaceAll(name, "-", "_")
}
