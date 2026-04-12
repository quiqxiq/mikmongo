package spec

import "time"

func NetworkSpecs() []CommandSpec {
	return []CommandSpec{
		{
			Name:     "ip_pools",
			Pipeline: PipelineOperational,
			Tier:     Tier3,
			Storage:  StorageRedis,
			Args:     []string{"/ip/pool/print"},
			Interval: 10 * time.Minute,
			RedisKey: "ip:pools",
			KeyField: "name",
			TTL:      20 * time.Minute,
			Enabled:  true,
		},
		// NEW spec validated against routeros_7.20.8.json
		{
			Name:     "ip_addresses",
			Pipeline: PipelineOperational,
			Tier:     Tier3,
			Storage:  StorageRedis,
			Args:     []string{"/ip/address/print"},
			Interval: 5 * time.Minute,
			RedisKey: "ip:addresses",
			KeyField: "address",
			TTL:      10 * time.Minute,
			Enabled:  true,
		},
		// NEW spec validated against routeros_7.20.8.json
		{
			Name:     "firewall_nat",
			Pipeline: PipelineOperational,
			Tier:     Tier3,
			Storage:  StorageRedis,
			Args:     []string{"/ip/firewall/nat/print"},
			Interval: 10 * time.Minute,
			RedisKey: "firewall:nat",
			KeyField: ".id",
			TTL:      20 * time.Minute,
			Enabled:  true,
		},
	}
}
