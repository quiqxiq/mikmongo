// pipeline/ondemand/runner.go - Pipeline C: On-demand Run()
package ondemand

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/Butterfly-Student/go-ros/client"
	"github.com/Butterfly-Student/go-ros/collector/pool"
	"github.com/Butterfly-Student/go-ros/collector/writer"
)

// Runner handles on-demand operations (read/write langsung ke router)
type Runner struct {
	routerID     string
	pool         *pool.ConnPool
	redisHandler *writer.RedisHandler // Untuk cache invalidation
}

// NewRunner creates new on-demand runner
func NewRunner(routerID string, p *pool.ConnPool, redisHandler *writer.RedisHandler) *Runner {
	return &Runner{
		routerID:     routerID,
		pool:         p,
		redisHandler: redisHandler,
	}
}

// Run executes read command dan return results langsung
// Digunakan untuk: ping, log read, diagnostics
func (r *Runner) Run(ctx context.Context, args []string) ([]map[string]string, error) {
	// Acquire connection dari pool
	conn, err := r.pool.Acquire()
	if err != nil {
		return nil, fmt.Errorf("failed to acquire connection: %w", err)
	}
	defer r.pool.Release(conn)
	
	// Set timeout
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	
	// Execute command
	results, err := conn.Client.RunRaw(ctx, args)
	if err != nil {
		return nil, fmt.Errorf("command failed: %w", err)
	}
	
	log.Printf("[OnDemandRunner %s] Executed: %v, results: %d", r.routerID, args[0], len(results))
	return results, nil
}

// Write executes write command dan invalidate cache
// Digunakan untuk: add/edit/delete PPP secrets, profiles, dll
func (r *Runner) Write(ctx context.Context, args []string, invalidateKeys []string) error {
	// Acquire connection dari pool
	conn, err := r.pool.Acquire()
	if err != nil {
		return fmt.Errorf("failed to acquire connection: %w", err)
	}
	defer r.pool.Release(conn)
	
	// Set timeout
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	
	// Execute write command
	_, err = conn.Client.RunRaw(ctx, args)
	if err != nil {
		return fmt.Errorf("write failed: %w", err)
	}
	
	// Invalidate cache keys
	if r.redisHandler != nil && len(invalidateKeys) > 0 {
		if err := r.redisHandler.Invalidate(ctx, r.routerID, invalidateKeys); err != nil {
			log.Printf("[OnDemandRunner %s] Cache invalidation warning: %v", r.routerID, err)
			// Don't fail the write if invalidation fails
		}
		log.Printf("[OnDemandRunner %s] Invalidated keys: %v", r.routerID, invalidateKeys)
	}
	
	log.Printf("[OnDemandRunner %s] Write executed: %v", r.routerID, args[0])
	return nil
}

// Ping tests connectivity ke router
func (r *Runner) Ping(ctx context.Context) error {
	conn, err := r.pool.Acquire()
	if err != nil {
		return err
	}
	defer r.pool.Release(conn)
	
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	
	_, err = conn.Client.RunRaw(ctx, []string{"/ping", "=address=127.0.0.1", "=count=1"})
	return err
}

// GetRouterConfig returns router configuration
func (r *Runner) GetRouterConfig() client.Config {
	// This would return the router config untuk reconnect purposes
	return client.Config{}
}
