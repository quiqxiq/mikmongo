package spec

func AllSpecs() []CommandSpec {
	groups := [][]CommandSpec{
		InterfaceSpecs(),
		SystemSpecs(),
		PPPSpecs(),
		HotspotSpecs(),
		QueueSpecs(),
		NetworkSpecs(),
	}

	total := 0
	for _, g := range groups {
		total += len(g)
	}

	all := make([]CommandSpec, 0, total)
	for _, g := range groups {
		all = append(all, g...)
	}
	return all
}

func DefaultSpecs() []CommandSpec {
	all := AllSpecs()
	out := make([]CommandSpec, 0, len(all))
	for _, s := range all {
		if s.Enabled {
			out = append(out, s)
		}
	}
	return out
}

// FilterByPipeline returns enabled specs for the given pipeline.
func FilterByPipeline(specs []CommandSpec, pipeline PipelineType) []CommandSpec {
	var result []CommandSpec
	for _, s := range specs {
		if s.Pipeline == pipeline && s.Enabled {
			result = append(result, s)
		}
	}
	return result
}

// FilterByTier returns enabled operational specs for the given tier.
func FilterByTier(specs []CommandSpec, tier Tier) []CommandSpec {
	var result []CommandSpec
	for _, s := range specs {
		if s.Pipeline == PipelineOperational && s.Tier == tier && s.Enabled {
			result = append(result, s)
		}
	}
	return result
}
