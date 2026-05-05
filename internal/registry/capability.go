package registry

// Fixed batch models: one API call produces this many artifacts.
var fixedBatchModels = map[string]int{
	"youchuan/midjourney": 4,
}

// OutputCapability returns the output capability for a model.
func OutputCapability(model string) OutputCap {
	reg, err := Load()
	if err != nil {
		return defaultOutputCap(model)
	}
	return reg.OutputCapability(model)
}

// OutputCapability returns the output capability for a model from a loaded registry.
func (r *Registry) OutputCapability(model string) OutputCap {
	cap := defaultOutputCap(model)
	if cap.IsFixedBatch {
		return cap
	}
	record, ok := r.Model(model)
	if !ok {
		return cap
	}
	props, _ := record.RequestSchema["properties"].(map[string]any)
	for _, name := range []string{"n", "max_images", "num_images"} {
		prop := schemaMap(props[name])
		if prop == nil {
			continue
		}
		cap.MultiField = name
		if max, ok := number(prop["maximum"]); ok && max > 0 {
			cap.MaxOutputs = int(max)
		}
		return cap
	}
	return cap
}

func defaultOutputCap(model string) OutputCap {
	cap := OutputCap{BatchSize: 1, MaxOutputs: 1}
	if batch, ok := fixedBatchModels[model]; ok {
		cap.IsFixedBatch = true
		cap.BatchSize = batch
	}
	return cap
}
