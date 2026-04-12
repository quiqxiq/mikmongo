package mikrotik

import (
	"encoding/json"
	"fmt"

	"github.com/gin-gonic/gin"
	goredis "github.com/redis/go-redis/v9"
	"mikmongo/pkg/response"
)

// CachedHandler provides cache-first reads from Redis hashes
// populated by the collector pipeline.
type CachedHandler struct {
	rdb *goredis.Client
}

// NewCachedHandler creates a new CachedHandler.
func NewCachedHandler(rdb *goredis.Client) *CachedHandler {
	return &CachedHandler{rdb: rdb}
}

// GetCachedPPPActive returns cached PPP active users from Redis.
func (h *CachedHandler) GetCachedPPPActive(c *gin.Context) {
	h.getHashEntries(c, "ppp:active")
}

// GetCachedHotspotActive returns cached hotspot active users from Redis.
func (h *CachedHandler) GetCachedHotspotActive(c *gin.Context) {
	h.getHashEntries(c, "hotspot:active")
}

// GetCachedPPPSecrets returns cached PPP secrets from Redis.
func (h *CachedHandler) GetCachedPPPSecrets(c *gin.Context) {
	h.getHashEntries(c, "ppp:secrets")
}

// GetCachedQueueStats returns cached queue statistics from Redis.
func (h *CachedHandler) GetCachedQueueStats(c *gin.Context) {
	h.getHashEntries(c, "queue:stats")
}

// getHashEntries reads all entries from a Redis hash and returns them
// as a JSON array. Each hash value is expected to be a JSON string.
func (h *CachedHandler) getHashEntries(c *gin.Context, suffix string) {
	routerID := c.Param("router_id")
	if routerID == "" {
		response.BadRequest(c, "missing router_id")
		return
	}

	key := fmt.Sprintf("%s:%s", routerID, suffix)

	result, err := h.rdb.HGetAll(c.Request.Context(), key).Result()
	if err != nil {
		response.InternalServerError(c, fmt.Sprintf("failed to read cache: %v", err))
		return
	}

	entries := make([]json.RawMessage, 0, len(result))
	for _, v := range result {
		// Each value is a JSON string; keep it as raw JSON to avoid
		// unnecessary marshal/unmarshal round-trips.
		raw := json.RawMessage(v)
		if json.Valid(raw) {
			entries = append(entries, raw)
		}
	}

	response.OK(c, entries)
}
