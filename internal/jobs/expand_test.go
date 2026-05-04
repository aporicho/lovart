package jobs

import (
	"testing"

	"github.com/aporicho/lovart/internal/config"
)

func TestExpand_GPT_WithinLimit(t *testing.T) {
	body := map[string]any{"prompt": "test", "quality": "low", "size": "1024*1024"}
	subs, err := Expand("openai/gpt-image-2", 5, body)
	if err != nil {
		t.Fatal(err)
	}
	if len(subs) != 1 {
		t.Fatalf("expected 1 sub-request, got %d", len(subs))
	}
	if subs[0].N != 5 {
		t.Errorf("expected N=5, got %d", subs[0].N)
	}
	if subs[0].Body["n"] != 5 {
		t.Errorf("body[n] = %v, want 5", subs[0].Body["n"])
	}
}

func TestExpand_GPT_OverLimit(t *testing.T) {
	body := map[string]any{"prompt": "test", "quality": "high", "size": "2048*2048"}
	subs, err := Expand("openai/gpt-image-2", 25, body)
	if err != nil {
		t.Fatal(err)
	}
	if len(subs) != 3 {
		t.Fatalf("expected 3 sub-requests, got %d", len(subs))
	}
	if subs[0].N != 10 {
		t.Errorf("sub[0].N = %d, want 10", subs[0].N)
	}
	if subs[1].N != 10 {
		t.Errorf("sub[1].N = %d, want 10", subs[1].N)
	}
	if subs[2].N != 5 {
		t.Errorf("sub[2].N = %d, want 5", subs[2].N)
	}
}

func TestExpand_Midjourney(t *testing.T) {
	body := map[string]any{"prompt": "test", "aspect_ratio": "1:1"}
	subs, err := Expand("youchuan/midjourney", 10, body)
	if err != nil {
		t.Fatal(err)
	}
	if len(subs) != 3 {
		t.Fatalf("expected 3 sub-requests (ceil(10/4)), got %d", len(subs))
	}
}

func TestExpand_SingleImage(t *testing.T) {
	body := map[string]any{"prompt": "test", "aspect_ratio": "1:1"}
	subs, err := Expand("vertex/nano-banana-2", 5, body)
	if err != nil {
		t.Fatal(err)
	}
	if len(subs) != 5 {
		t.Fatalf("expected 5 sub-requests, got %d", len(subs))
	}
}

func TestExpand_Zero(t *testing.T) {
	subs, err := Expand("openai/gpt-image-2", 0, map[string]any{})
	if err != nil {
		t.Fatal(err)
	}
	if len(subs) != 0 {
		t.Errorf("expected 0 sub-requests, got %d", len(subs))
	}
}

func TestCostSignature_SameModelDifferentPrompt(t *testing.T) {
	a := JobLine{
		JobID: "001", Model: "openai/gpt-image-2", Outputs: 4,
		Body: map[string]any{"prompt": "a red cube", "quality": "high", "size": "2048*2048"},
	}
	b := JobLine{
		JobID: "002", Model: "openai/gpt-image-2", Outputs: 4,
		Body: map[string]any{"prompt": "a blue sphere", "quality": "high", "size": "2048*2048"},
	}
	if CostSignature(a) != CostSignature(b) {
		t.Error("same params + different prompt should have same signature")
	}
}

func TestCostSignature_SameParamsSameSignature(t *testing.T) {
	a := JobLine{
		JobID: "001", Model: "openai/gpt-image-2", Outputs: 4,
		Body: map[string]any{"prompt": "same", "quality": "high", "size": "2048*2048"},
	}
	b := JobLine{
		JobID: "002", Model: "openai/gpt-image-2", Outputs: 4,
		Body: map[string]any{"prompt": "same", "quality": "high", "size": "2048*2048"},
	}
	if CostSignature(a) != CostSignature(b) {
		t.Error("identical params should have same signature")
	}
}

func TestCostSignature_DifferentQuality(t *testing.T) {
	a := JobLine{
		JobID: "001", Model: "openai/gpt-image-2", Outputs: 1,
		Body: map[string]any{"prompt": "test", "quality": "low", "size": "1024*1024"},
	}
	b := JobLine{
		JobID: "002", Model: "openai/gpt-image-2", Outputs: 1,
		Body: map[string]any{"prompt": "test", "quality": "high", "size": "1024*1024"},
	}
	if CostSignature(a) == CostSignature(b) {
		t.Error("different quality should have different signature")
	}
}

func TestCostSignature_DifferentOutputs(t *testing.T) {
	a := JobLine{
		JobID: "001", Model: "openai/gpt-image-2", Outputs: 4,
		Body: map[string]any{"prompt": "test", "quality": "high", "size": "1024*1024"},
	}
	b := JobLine{
		JobID: "002", Model: "openai/gpt-image-2", Outputs: 10,
		Body: map[string]any{"prompt": "test", "quality": "high", "size": "1024*1024"},
	}
	if CostSignature(a) == CostSignature(b) {
		t.Error("different outputs should have different signature")
	}
}

// Ensure config package is used.
var _ = config.OutputCapability
