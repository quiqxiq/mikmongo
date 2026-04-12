package spec

import "time"

func HotspotSpecs() []CommandSpec {
	return []CommandSpec{
		{
			Name:      "hotspot_active",
			Pipeline:  PipelineOperational,
			Tier:      Tier2,
			Storage:   StorageRedis,
			Args:      []string{"/ip/hotspot/active/print", "=follow="},
			UseFollow: true,
			RedisKey:  "hotspot:active",
			KeyField:  "user",
			Enabled:   true,
		},
		{
			Name:     "hotspot_users",
			Pipeline: PipelineOperational,
			Tier:     Tier3,
			Storage:  StorageRedis,
			Args:     []string{"/ip/hotspot/user/print"},
			Interval: 5 * time.Minute,
			RedisKey: "hotspot:users",
			KeyField: "name",
			TTL:      10 * time.Minute,
			Enabled:  true,
		},
	}
}
