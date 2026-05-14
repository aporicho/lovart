package registry

import "testing"

func TestNormalizeRequestFillsDefaultsWithoutMutatingInput(t *testing.T) {
	setupRegistryMetadata(t)

	reg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	body := map[string]any{
		"prompt": "test",
		"options": map[string]any{
			"seed": float64(7),
		},
	}
	normalized, err := reg.NormalizeRequest("openai/gpt-image-2", body)
	if err != nil {
		t.Fatalf("NormalizeRequest: %v", err)
	}
	if normalized["resolution"] != "1K" {
		t.Fatalf("resolution default = %#v, want 1K", normalized["resolution"])
	}
	if _, ok := body["resolution"]; ok {
		t.Fatalf("NormalizeRequest mutated input body: %#v", body)
	}
	options := normalized["options"].(map[string]any)
	options["seed"] = float64(8)
	if body["options"].(map[string]any)["seed"] != float64(7) {
		t.Fatalf("NormalizeRequest did not deep-copy nested values")
	}
}
