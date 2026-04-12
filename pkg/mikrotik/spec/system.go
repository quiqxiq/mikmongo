package spec

func SystemSpecs() []CommandSpec {
	return []CommandSpec{
		{
			Name:        "system_resource",
			Pipeline:    PipelineTimeSeries,
			Storage:     StorageInfluxDB,
			Args:        []string{"/system/resource/monitor", "=interval=2s", "=once="},
			UseFollow:   true,
			Measurement: "system_resource",
			TagFields:   []string{"router_id"},
			ValueFields: []string{"cpu-load", "free-memory", "total-memory", "uptime", "cpu-count", "version"},
			Enabled:     true,
		},
		// NEW spec validated against routeros_7.20.8.json
		{
			Name:         "router_log",
			Pipeline:     PipelineOperational,
			Tier:         Tier2,
			Storage:      StorageRedisStream,
			Args:         []string{"/log/print", "=follow="},
			UseFollow:    true,
			RedisKey:     "log:entries",
			StreamMaxLen: 1000,
			Enabled:      true,
		},
	}
}
