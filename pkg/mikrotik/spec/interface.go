package spec

import "time"

func InterfaceSpecs() []CommandSpec {
	return []CommandSpec{
		{
			Name:        "interface_traffic",
			Pipeline:    PipelineTimeSeries,
			Storage:     StorageInfluxDB,
			Args:        []string{"/interface/monitor-traffic", "=interface=all", "=.proplist=name,rx-bits-per-second,tx-bits-per-second,rx-packets-per-second,tx-packets-per-second,rx-drops,tx-drops"},
			UseFollow:   true,
			Measurement: "interface_traffic",
			TagFields:   []string{"name"},
			ValueFields: []string{"rx-bits-per-second", "tx-bits-per-second", "rx-packets-per-second", "tx-packets-per-second", "rx-drops", "tx-drops"},
			Enabled:     true,
		},
		{
			Name:      "interface_status",
			Pipeline:  PipelineOperational,
			Tier:      Tier2,
			Storage:   StorageRedis,
			Args:      []string{"/interface/print", "=follow="},
			UseFollow: true,
			RedisKey:  "interface:status",
			KeyField:  "name",
			Enabled:   true,
		},
		// NEW spec validated against routeros_7.20.8.json
		{
			Name:     "interface_list",
			Pipeline: PipelineOperational,
			Tier:     Tier3,
			Storage:  StorageRedis,
			Args:     []string{"/interface/print"},
			Interval: 5 * time.Minute,
			RedisKey: "interface:list",
			KeyField: "name",
			TTL:      10 * time.Minute,
			Enabled:  true,
		},
	}
}
