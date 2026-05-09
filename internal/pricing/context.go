package pricing

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/aporicho/lovart/internal/http"
)

const (
	ModeAuto  = "auto"
	ModeFast  = "fast"
	ModeRelax = "relax"
)

const (
	unlimitedEndpoint     = "/api/canva/agent-cashier/task/query/unlimited"
	fastUnlimitedEndpoint = "/api/canva/agent-cashier/task/query/fast/unlimited"
	timeVariantEndpoint   = "/api/canva/agent-cashier/pricing/timeVariantConfig"
)

// QuoteOptions controls how quote prices are interpreted.
type QuoteOptions struct {
	Mode string
	Now  time.Time
}

// PricingContext explains how a quote price was derived.
type PricingContext struct {
	Mode           string           `json:"mode"`
	PriceSource    string           `json:"price_source"`
	ServerPrice    float64          `json:"server_price"`
	NominalPrice   float64          `json:"nominal_price"`
	EffectivePrice float64          `json:"effective_price"`
	TimeRate       float64          `json:"time_rate"`
	TimeWindow     string           `json:"time_window"`
	Unlimited      UnlimitedContext `json:"unlimited"`
	Warnings       []string         `json:"warnings,omitempty"`
}

// UnlimitedContext records the unlimited-plan status used by mode-aware pricing.
type UnlimitedContext struct {
	Checked       bool   `json:"checked"`
	Available     bool   `json:"available"`
	Enabled       bool   `json:"enabled"`
	Matched       bool   `json:"matched"`
	Endpoint      string `json:"endpoint,omitempty"`
	ExtraItem     string `json:"extra_item,omitempty"`
	RemainingDays int    `json:"remaining_days,omitempty"`
}

type unlimitedResponse struct {
	Code int             `json:"code"`
	Data unlimitedStatus `json:"data"`
}

type unlimitedStatus struct {
	Unlimited       bool            `json:"unlimited"`
	UnlimitedEnable bool            `json:"unlimitedEnable"`
	UnlimitedList   []unlimitedItem `json:"unlimited_list"`
}

type unlimitedItem struct {
	ModelDisplayName string   `json:"model_display_name"`
	Status           int      `json:"status"`
	RemainingDays    int      `json:"remaining_days"`
	ExtraItem        *string  `json:"extraItem"`
	AliasList        []string `json:"alias_list"`
}

type timeVariantResponse struct {
	Code int               `json:"code"`
	Data timeVariantConfig `json:"data"`
}

type timeVariantConfig struct {
	OffPeakRate      string `json:"offPeakRate"`
	PeakRate         string `json:"peakRate"`
	OffPeakStartTime string `json:"offPeakStartTime"`
	OffPeakEndTime   string `json:"offPeakEndTime"`
	PeakStartTime    string `json:"peakStartTime"`
	PeakEndTime      string `json:"peakEndTime"`
	PeakEnable       bool   `json:"peakEnable"`
	OffPeakEnable    bool   `json:"offPeakEnable"`
}

type requestFacts struct {
	quality    string
	resolution string
}

// NormalizeMode returns a canonical mode name accepted by pricing/generation.
func NormalizeMode(mode string) (string, error) {
	mode = strings.ToLower(strings.TrimSpace(mode))
	if mode == "" {
		mode = ModeAuto
	}
	switch mode {
	case ModeAuto, ModeFast, ModeRelax:
		return mode, nil
	default:
		return "", fmt.Errorf("mode must be one of auto, fast, relax")
	}
}

// QuoteWithOptions fetches a live quote and derives the effective price for a mode.
func QuoteWithOptions(ctx context.Context, client *http.Client, model string, body map[string]any, opts QuoteOptions) (*QuoteResult, error) {
	mode, err := NormalizeMode(opts.Mode)
	if err != nil {
		return nil, fmt.Errorf("pricing: %w", err)
	}

	result, err := baseQuote(ctx, client, model, body)
	if err != nil {
		return nil, err
	}

	now := opts.Now
	if now.IsZero() {
		now = time.Now()
	}

	pc := basePricingContext(mode, result)
	if mode == ModeAuto {
		result.PricingContext = pc
		return result, nil
	}

	endpoint := modeUnlimitedEndpoint(mode)
	pc.Unlimited.Checked = true
	pc.Unlimited.Endpoint = endpoint

	var unlimited *unlimitedStatus
	if status, err := fetchUnlimitedStatus(ctx, client, endpoint); err != nil {
		pc.Warnings = append(pc.Warnings, err.Error())
	} else {
		unlimited = status
	}

	var timeVariant *timeVariantConfig
	if cfg, err := fetchTimeVariantConfig(ctx, client); err != nil {
		pc.Warnings = append(pc.Warnings, err.Error())
	} else {
		timeVariant = cfg
	}

	applyModePricing(pc, result, model, body, unlimited, timeVariant, now)
	result.Price = pc.EffectivePrice
	result.PricingContext = pc
	return result, nil
}

func basePricingContext(mode string, result *QuoteResult) *PricingContext {
	nominal := nominalPrice(result)
	return &PricingContext{
		Mode:           mode,
		PriceSource:    "server",
		ServerPrice:    result.Price,
		NominalPrice:   nominal,
		EffectivePrice: result.Price,
		TimeRate:       1,
		TimeWindow:     "unknown",
	}
}

func fetchUnlimitedStatus(ctx context.Context, client *http.Client, endpoint string) (*unlimitedStatus, error) {
	var resp unlimitedResponse
	if err := client.GetJSON(ctx, http.WWWBase, endpoint, &resp); err != nil {
		return nil, fmt.Errorf("pricing: unlimited status: %w", err)
	}
	if resp.Code != 0 {
		return nil, fmt.Errorf("pricing: unlimited status returned code %d", resp.Code)
	}
	return &resp.Data, nil
}

func fetchTimeVariantConfig(ctx context.Context, client *http.Client) (*timeVariantConfig, error) {
	var resp timeVariantResponse
	if err := client.GetJSON(ctx, http.WWWBase, timeVariantEndpoint, &resp); err != nil {
		return nil, fmt.Errorf("pricing: time variant config: %w", err)
	}
	if resp.Code != 0 {
		return nil, fmt.Errorf("pricing: time variant config returned code %d", resp.Code)
	}
	return &resp.Data, nil
}

func applyModePricing(pc *PricingContext, result *QuoteResult, model string, body map[string]any, status *unlimitedStatus, cfg *timeVariantConfig, now time.Time) {
	pc.NominalPrice = nominalPrice(result)
	pc.EffectivePrice = result.Price
	pc.PriceSource = "server"
	pc.TimeRate = 1
	pc.TimeWindow = "unknown"

	if cfg != nil {
		pc.TimeRate, pc.TimeWindow = activeTimeRate(*cfg, now)
	}
	if status != nil {
		pc.Unlimited.Available = status.Unlimited
		pc.Unlimited.Enabled = status.UnlimitedEnable
	}

	if item, ok := matchingUnlimited(model, requestFactsFor(result.PriceDetail, body), status); ok {
		pc.Unlimited.Matched = true
		pc.Unlimited.RemainingDays = item.RemainingDays
		if item.ExtraItem != nil {
			pc.Unlimited.ExtraItem = strings.TrimSpace(*item.ExtraItem)
		}
		pc.PriceSource = "unlimited"
		pc.EffectivePrice = 0
		return
	}

	if cfg == nil {
		return
	}

	pc.EffectivePrice = pc.NominalPrice * pc.TimeRate
	if pc.TimeRate == 1 {
		pc.PriceSource = "nominal"
	} else {
		pc.PriceSource = "time_variant"
	}
}

func modeUnlimitedEndpoint(mode string) string {
	if mode == ModeFast {
		return fastUnlimitedEndpoint
	}
	return unlimitedEndpoint
}

func nominalPrice(result *QuoteResult) float64 {
	if result == nil {
		return 0
	}
	detail := result.PriceDetail
	if detail.TotalPrice > 0 {
		return detail.TotalPrice
	}
	if detail.UnitPrice > 0 && detail.UnitCount > 0 {
		return detail.UnitPrice * float64(detail.UnitCount)
	}
	if detail.PriceBeforeSurcharge > 0 || detail.ImageCountSurcharge > 0 {
		return detail.PriceBeforeSurcharge + detail.ImageCountSurcharge
	}
	return result.Price
}

func matchingUnlimited(model string, facts requestFacts, status *unlimitedStatus) (unlimitedItem, bool) {
	if status == nil || !status.UnlimitedEnable {
		return unlimitedItem{}, false
	}
	for _, item := range status.UnlimitedList {
		if item.Status != 1 {
			continue
		}
		if !modelMatches(item, model) {
			continue
		}
		if !unlimitedExtraMatches(item.ExtraItem, facts) {
			continue
		}
		return item, true
	}
	return unlimitedItem{}, false
}

func modelMatches(item unlimitedItem, model string) bool {
	if strings.EqualFold(strings.TrimSpace(item.ModelDisplayName), strings.TrimSpace(model)) {
		return true
	}
	for _, alias := range item.AliasList {
		if strings.EqualFold(strings.TrimSpace(alias), strings.TrimSpace(model)) {
			return true
		}
	}
	return false
}

func unlimitedExtraMatches(extra *string, facts requestFacts) bool {
	if extra == nil || strings.TrimSpace(*extra) == "" {
		return true
	}
	extraText := normalizeText(*extra)

	if facts.quality != "" {
		extraQuality := qualityFromText(extraText)
		if extraQuality != "" && extraQuality != facts.quality {
			return false
		}
	}

	if facts.resolution != "" {
		extraResolution := resolutionFromText(extraText)
		if extraResolution != "" && extraResolution != facts.resolution {
			return false
		}
	}

	return true
}

func requestFactsFor(detail PriceDetail, body map[string]any) requestFacts {
	args := body
	if len(detail.InputArgs) > 0 {
		args = detail.InputArgs
	}
	return requestFacts{
		quality:    qualityFromText(valueString(args["quality"])),
		resolution: resolutionFromArgs(args),
	}
}

func resolutionFromArgs(args map[string]any) string {
	for _, key := range []string{"resolution", "size"} {
		if value, ok := args[key]; ok {
			if resolution := resolutionFromText(valueString(value)); resolution != "" {
				return resolution
			}
		}
	}
	return ""
}

func qualityFromText(value string) string {
	text := normalizeText(value)
	for _, token := range []string{"low", "medium", "high"} {
		if strings.Contains(text, token) {
			return token
		}
	}
	return ""
}

func resolutionFromText(value string) string {
	text := normalizeText(value)
	if strings.Contains(text, "4k") {
		return "4k"
	}
	if strings.Contains(text, "2k") {
		return "2k"
	}
	if strings.Contains(text, "1k") {
		return "1k"
	}
	if strings.Contains(text, "3840") || strings.Contains(text, "2160") {
		return "4k"
	}
	if strings.Contains(text, "2048") || strings.Contains(text, "1152") {
		return "2k"
	}
	if strings.Contains(text, "1024") || strings.Contains(text, "1536") {
		return "1k"
	}
	return ""
}

func normalizeText(value string) string {
	text := strings.ToLower(strings.TrimSpace(value))
	text = strings.ReplaceAll(text, " ", "")
	text = strings.ReplaceAll(text, "_", "")
	text = strings.ReplaceAll(text, "-", "")
	text = strings.ReplaceAll(text, "x", "*")
	text = strings.ReplaceAll(text, "×", "*")
	return text
}

func valueString(value any) string {
	switch v := value.(type) {
	case string:
		return v
	case fmt.Stringer:
		return v.String()
	case nil:
		return ""
	default:
		return fmt.Sprint(v)
	}
}

func activeTimeRate(cfg timeVariantConfig, now time.Time) (float64, string) {
	ms := millisecondsSinceDayStart(now)
	if cfg.PeakEnable && inTimeRange(ms, parseInt64(cfg.PeakStartTime), parseInt64(cfg.PeakEndTime)) {
		return parseRate(cfg.PeakRate, 1), "peak"
	}
	if cfg.OffPeakEnable && inTimeRange(ms, parseInt64(cfg.OffPeakStartTime), parseInt64(cfg.OffPeakEndTime)) {
		return parseRate(cfg.OffPeakRate, 1), "off_peak"
	}
	return 1, "standard"
}

func millisecondsSinceDayStart(now time.Time) int64 {
	return int64(now.Hour())*60*60*1000 +
		int64(now.Minute())*60*1000 +
		int64(now.Second())*1000 +
		int64(now.Nanosecond())/int64(time.Millisecond)
}

func inTimeRange(ms, start, end int64) bool {
	if start == end {
		return false
	}
	if start < end {
		return ms >= start && ms < end
	}
	return ms >= start || ms < end
}

func parseRate(value string, fallback float64) float64 {
	rate, err := strconv.ParseFloat(strings.TrimSpace(value), 64)
	if err != nil || rate <= 0 {
		return fallback
	}
	return rate
}

func parseInt64(value string) int64 {
	parsed, _ := strconv.ParseInt(strings.TrimSpace(value), 10, 64)
	return parsed
}
