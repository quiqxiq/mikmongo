// writer/influx_handler.go - InfluxDB Batch Handler
package writer

import (
	"context"
	"fmt"
	"strings"
	"time"

	influxdb2 "github.com/influxdata/influxdb-client-go/v2"
	"github.com/influxdata/influxdb-client-go/v2/api"
	"github.com/influxdata/influxdb-client-go/v2/api/write"
)

// InfluxHandler writes batches to InfluxDB
type InfluxHandler struct {
	client   influxdb2.Client
	writeAPI api.WriteAPIBlocking
	org      string
	bucket   string
}

// InfluxConfig untuk InfluxDB connection
type InfluxConfig struct {
	URL    string
	Token  string
	Org    string
	Bucket string
}

// NewInfluxHandler creates new InfluxDB handler
func NewInfluxHandler(config InfluxConfig) (*InfluxHandler, error) {
	client := influxdb2.NewClient(config.URL, config.Token)
	
	// Test connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	
	if _, err := client.Health(ctx); err != nil {
		return nil, fmt.Errorf("influxdb health check failed: %w", err)
	}
	
	writeAPI := client.WriteAPIBlocking(config.Org, config.Bucket)
	
	return &InfluxHandler{
		client:   client,
		writeAPI: writeAPI,
		org:      config.Org,
		bucket:   config.Bucket,
	}, nil
}

// Close closes InfluxDB client
func (h *InfluxHandler) Close() {
	h.client.Close()
}

// WriteBatch writes batch of items to InfluxDB
func (h *InfluxHandler) WriteBatch(ctx context.Context, items []WriteItem) error {
	points := make([]*write.Point, 0, len(items))
	
	for _, item := range items {
		point := h.itemToPoint(item)
		if point != nil {
			points = append(points, point)
		}
	}
	
	if len(points) == 0 {
		return nil
	}
	
	return h.writeAPI.WritePoint(ctx, points...)
}

// itemToPoint converts WriteItem to InfluxDB point
func (h *InfluxHandler) itemToPoint(item WriteItem) *write.Point {
	// Build tags
	tags := make(map[string]string)
	tags["router"] = item.RouterID
	
	for k, v := range item.Tags {
		tags[k] = v
	}
	
	// Build fields - convert string values to appropriate types
	fields := make(map[string]interface{})
	for k, v := range item.Fields {
		fields[k] = convertValue(v)
	}
	
	// Create point
	return influxdb2.NewPoint(
		item.Measurement,
		tags,
		fields,
		item.Timestamp,
	)
}

// convertValue converts string value to numeric jika memungkinkan
func convertValue(v interface{}) interface{} {
	switch val := v.(type) {
	case string:
		// Coba parse sebagai int
		if i, err := parseInt(val); err == nil {
			return i
		}
		// Coba parse sebagai float
		if f, err := parseFloat(val); err == nil {
			return f
		}
		// Return as string
		return val
	default:
		return val
	}
}

// parseInt mencoba parse string sebagai int
func parseInt(s string) (int64, error) {
	var result int64
	_, err := fmt.Sscanf(s, "%d", &result)
	return result, err
}

// parseFloat mencoba parse string sebagai float
func parseFloat(s string) (float64, error) {
	var result float64
	_, err := fmt.Sscanf(s, "%f", &result)
	return result, err
}

// parseLineProtocol parses RouterOS output untuk jadi line protocol
// Contoh: rx-byte=12345 tx-byte=67890 -> fields
func ParseLineProtocol(data map[string]string, tagFields, valueFields []string) (tags, fields map[string]string) {
	tags = make(map[string]string)
	fields = make(map[string]string)
	
	for k, v := range data {
		// Check if it's a tag field
		isTag := false
		for _, tf := range tagFields {
			if strings.EqualFold(k, tf) {
				tags[k] = v
				isTag = true
				break
			}
		}
		
		// Check if it's a value field
		if !isTag {
			for _, vf := range valueFields {
				if strings.EqualFold(k, vf) {
					fields[k] = v
					break
				}
			}
		}
	}
	
	return tags, fields
}
