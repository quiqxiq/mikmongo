package spec

func QueueSpecs() []CommandSpec {
	return []CommandSpec{
		{
			Name:        "queue_stats",
			Pipeline:    PipelineTimeSeries,
			Storage:     StorageInfluxDB,
			Args:        []string{"/queue/simple/print", "=stats="},
			UseFollow:   true,
			Measurement: "queue_stats",
			TagFields:   []string{"name"},
			ValueFields: []string{"bytes", "packets", "dropped", "rate"},
			Enabled:     true,
		},
	}
}
