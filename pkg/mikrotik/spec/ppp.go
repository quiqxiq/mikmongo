package spec

import "time"

func PPPSpecs() []CommandSpec {
	return []CommandSpec{
		{
			Name:      "ppp_active",
			Pipeline:  PipelineOperational,
			Tier:      Tier2,
			Storage:   StorageRedis,
			Args:      []string{"/ppp/active/print", "=follow="},
			UseFollow: true,
			RedisKey:  "ppp:active",
			KeyField:  "name",
			Enabled:   true,
		},
		{
			Name:     "ppp_secrets",
			Pipeline: PipelineOperational,
			Tier:     Tier3,
			Storage:  StorageRedis,
			Args:     []string{"/ppp/secret/print"},
			Interval: 5 * time.Minute,
			RedisKey: "ppp:secrets",
			KeyField: "name",
			TTL:      10 * time.Minute,
			Enabled:  true,
		},
		{
			Name:     "ppp_profiles",
			Pipeline: PipelineOperational,
			Tier:     Tier3,
			Storage:  StorageRedis,
			Args:     []string{"/ppp/profile/print"},
			Interval: 5 * time.Minute,
			RedisKey: "ppp:profiles",
			KeyField: "name",
			TTL:      10 * time.Minute,
			Enabled:  true,
		},
	}
}
