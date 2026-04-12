package spec

import "time"

// PipelineType menentukan pipeline mana yang handle spec ini.
type PipelineType string

const (
	PipelineTimeSeries  PipelineType = "time_series"
	PipelineOperational PipelineType = "operational"
	PipelineOnDemand    PipelineType = "on_demand"
)

// Tier dalam Pipeline Operational.
type Tier int

const (
	Tier2 Tier = 2
	Tier3 Tier = 3
)

// StorageType menentukan storage backend.
type StorageType string

const (
	StorageInfluxDB    StorageType = "influxdb"
	StorageRedis       StorageType = "redis"
	StorageRedisStream StorageType = "redis_stream"
	StorageNone        StorageType = "none"
)

// CommandSpec mendefinisikan satu command untuk collector.
type CommandSpec struct {
	Name     string
	Pipeline PipelineType
	Tier     Tier
	Storage  StorageType

	Args      []string
	UseFollow bool
	Interval  time.Duration

	Measurement string
	TagFields   []string
	ValueFields []string

	RedisKey string
	KeyField string
	TTL      time.Duration

	// StreamMaxLen caps the Redis Stream length when Storage == StorageRedisStream
	// (used as XADD ... MAXLEN ~ N).
	StreamMaxLen int64

	Enabled bool
}

func (s CommandSpec) IsTimeSeries() bool  { return s.Pipeline == PipelineTimeSeries }
func (s CommandSpec) IsOperational() bool { return s.Pipeline == PipelineOperational }
func (s CommandSpec) IsOnDemand() bool    { return s.Pipeline == PipelineOnDemand }
func (s CommandSpec) IsTier2() bool       { return s.Tier == Tier2 }
func (s CommandSpec) IsTier3() bool       { return s.Tier == Tier3 }
