package pricing

import "testing"

func TestUnlimitedExtraMatchesQualityAndResolution(t *testing.T) {
	extra := stringPtr("low 2K")
	if !unlimitedExtraMatches(extra, requestFacts{quality: "low", resolution: "1k"}) {
		t.Fatalf("expected low 2K to match low/1k")
	}
	if !unlimitedExtraMatches(extra, requestFacts{quality: "low", resolution: "2k"}) {
		t.Fatalf("expected low 2K to match low/2k")
	}
	if unlimitedExtraMatches(extra, requestFacts{quality: "medium", resolution: "2k"}) {
		t.Fatalf("expected low 2K not to match medium/2k")
	}
	if unlimitedExtraMatches(extra, requestFacts{quality: "low", resolution: "4k"}) {
		t.Fatalf("expected low 2K not to match low/4k")
	}
	if unlimitedExtraMatches(extra, requestFacts{quality: "low"}) {
		t.Fatalf("expected low 2K not to match missing resolution")
	}
	if unlimitedExtraMatches(extra, requestFacts{resolution: "2k"}) {
		t.Fatalf("expected low 2K not to match missing quality")
	}
	extra4K := stringPtr("4K")
	for _, resolution := range []string{"512", "1k", "2k", "4k"} {
		if !unlimitedExtraMatches(extra4K, requestFacts{resolution: resolution}) {
			t.Fatalf("expected 4K to match %s", resolution)
		}
	}
	if !unlimitedExtraMatches(nil, requestFacts{quality: "high", resolution: "4k"}) {
		t.Fatalf("expected empty extra item to match")
	}
}

func TestRequestFactsMergeServerInputArgsWithNormalizedBody(t *testing.T) {
	facts := requestFactsFor(
		PriceDetail{InputArgs: map[string]any{"aspect_ratio": "16:9"}},
		map[string]any{"resolution": "1K", "quality": "low"},
	)
	if facts.resolution != "1k" || facts.quality != "low" {
		t.Fatalf("facts = %#v, want normalized body fallback", facts)
	}
}
