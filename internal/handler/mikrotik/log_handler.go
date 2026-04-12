package mikrotik

import (
	"fmt"
	"strconv"

	"github.com/gin-gonic/gin"
	goredis "github.com/redis/go-redis/v9"
	"mikmongo/pkg/response"
)

const (
	defaultLogCount = 50
	maxLogCount     = 500
)

// LogHandler reads router logs from Redis Streams.
type LogHandler struct {
	rdb *goredis.Client
}

// NewLogHandler creates a new LogHandler.
func NewLogHandler(rdb *goredis.Client) *LogHandler {
	return &LogHandler{rdb: rdb}
}

// LogEntry represents a single router log entry.
type LogEntry struct {
	ID        string            `json:"id"`
	Timestamp string            `json:"timestamp,omitempty"`
	Message   string            `json:"message,omitempty"`
	Fields    map[string]string `json:"fields,omitempty"`
}

// GetRouterLogs reads the latest log entries from a Redis Stream
// using XREVRANGE (newest first).
//
// Query params:
//   - count: number of entries to return (default 50, max 500)
func (h *LogHandler) GetRouterLogs(c *gin.Context) {
	routerID := c.Param("router_id")
	if routerID == "" {
		response.BadRequest(c, "missing router_id")
		return
	}

	count := defaultLogCount
	if countStr := c.Query("count"); countStr != "" {
		parsed, err := strconv.Atoi(countStr)
		if err != nil || parsed < 1 {
			response.BadRequest(c, "count must be a positive integer")
			return
		}
		if parsed > maxLogCount {
			parsed = maxLogCount
		}
		count = parsed
	}

	key := fmt.Sprintf("%s:router:log", routerID)

	messages, err := h.rdb.XRevRangeN(c.Request.Context(), key, "+", "-", int64(count)).Result()
	if err != nil {
		response.InternalServerError(c, fmt.Sprintf("failed to read log stream: %v", err))
		return
	}

	entries := make([]LogEntry, 0, len(messages))
	for _, msg := range messages {
		entry := LogEntry{
			ID:     msg.ID,
			Fields: make(map[string]string, len(msg.Values)),
		}
		for k, v := range msg.Values {
			str, _ := v.(string)
			switch k {
			case "timestamp", "time":
				entry.Timestamp = str
			case "message", "msg":
				entry.Message = str
			default:
				entry.Fields[k] = str
			}
		}
		entries = append(entries, entry)
	}

	response.OK(c, entries)
}
