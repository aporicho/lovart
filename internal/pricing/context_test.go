package pricing

import (
	"math"
	"testing"
	"time"
)

func TestApplyModePricingMatchesRelaxUnlimited(t *testing.T) {
	result := &QuoteResult{
		Price: 42,
		PriceDetail: PriceDetail{
			TotalPrice: 12,
			InputArgs: map[string]any{
				"quality": "medium",
				"size":    "2048*1152",
			},
		},
	}
	pc := basePricingContext(ModeRelax, result)
	pc.Unlimited.Checked = true
	pc.Unlimited.Endpoint = unlimitedEndpoint
	status := &unlimitedStatus{
		Unlimited:       true,
		UnlimitedEnable: true,
		UnlimitedList: []unlimitedItem{{
			Status:        1,
			RemainingDays: 28,
			ExtraItem:     stringPtr("medium 2K"),
			AliasList:     []string{"openai/gpt-image-2"},
		}},
	}
	cfg := peakTimeVariantConfig()

	applyModePricing(pc, result, "openai/gpt-image-2", nil, status, &cfg, peakTime())

	if pc.PriceSource != "unlimited" {
		t.Fatalf("price_source = %q, want unlimited", pc.PriceSource)
	}
	if pc.EffectivePrice != 0 {
		t.Fatalf("effective_price = %v, want 0", pc.EffectivePrice)
	}
	if !pc.Unlimited.Matched {
		t.Fatalf("unlimited matched = false, want true")
	}
	if !pc.Unlimited.Available || !pc.Unlimited.Enabled {
		t.Fatalf("unlimited availability = %v/%v, want true/true", pc.Unlimited.Available, pc.Unlimited.Enabled)
	}
	if pc.Unlimited.ExtraItem != "medium 2K" {
		t.Fatalf("extra_item = %q, want medium 2K", pc.Unlimited.ExtraItem)
	}
	if pc.TimeRate != 1.4 || pc.TimeWindow != "peak" {
		t.Fatalf("time context = %v/%q, want 1.4/peak", pc.TimeRate, pc.TimeWindow)
	}
}

func TestApplyModePricingAppliesTimeRateWhenUnlimitedDoesNotMatch(t *testing.T) {
	result := &QuoteResult{
		Price: 42,
		PriceDetail: PriceDetail{
			TotalPrice: 12,
			InputArgs: map[string]any{
				"quality": "medium",
				"size":    "2048*1152",
			},
		},
	}
	pc := basePricingContext(ModeFast, result)
	pc.Unlimited.Checked = true
	pc.Unlimited.Endpoint = fastUnlimitedEndpoint
	status := &unlimitedStatus{
		Unlimited:       true,
		UnlimitedEnable: true,
		UnlimitedList: []unlimitedItem{{
			Status:        1,
			RemainingDays: 28,
			ExtraItem:     stringPtr("low 2K"),
			AliasList:     []string{"openai/gpt-image-2"},
		}},
	}
	cfg := peakTimeVariantConfig()

	applyModePricing(pc, result, "openai/gpt-image-2", nil, status, &cfg, peakTime())

	if pc.PriceSource != "time_variant" {
		t.Fatalf("price_source = %q, want time_variant", pc.PriceSource)
	}
	if pc.Unlimited.Matched {
		t.Fatalf("unlimited matched = true, want false")
	}
	if diff := math.Abs(pc.EffectivePrice - 16.8); diff > 0.0001 {
		t.Fatalf("effective_price = %v, want 16.8", pc.EffectivePrice)
	}
}

func peakTimeVariantConfig() timeVariantConfig {
	return timeVariantConfig{
		PeakRate:         "1.4",
		OffPeakRate:      "0.5",
		PeakStartTime:    "25200000",
		PeakEndTime:      "32400000",
		OffPeakStartTime: "50400000",
		OffPeakEndTime:   "68400000",
		PeakEnable:       true,
		OffPeakEnable:    true,
	}
}

func peakTime() time.Time {
	return time.Date(2026, 5, 10, 8, 0, 0, 0, time.FixedZone("CST", 8*60*60))
}

func stringPtr(value string) *string {
	return &value
}
