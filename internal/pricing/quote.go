package pricing

import (
	"context"
	"fmt"

	"github.com/aporicho/lovart/internal/http"
)

// QuoteResult contains the pricing response from the Lovart pricing API.
type QuoteResult struct {
	Price          float64         `json:"price"`
	Balance        float64         `json:"balance"`
	PriceDetail    PriceDetail     `json:"price_detail"`
	NormalizedBody map[string]any  `json:"normalized_body,omitempty"`
	PricingContext *PricingContext `json:"pricing_context,omitempty"`
}

// PriceDetail breaks down how the price is calculated.
type PriceDetail struct {
	UnitPrice     float64 `json:"unit_price"`
	UnitCount     int     `json:"unit_count"`
	UnitName      string  `json:"unit_name"`
	TotalPrice    float64 `json:"total_price"`
	SearchKey     string  `json:"search_key"`
	GeneratorName string  `json:"generator_name"`

	InputImageCount                 int            `json:"input_image_count"`
	InputImageCountField            string         `json:"input_image_count_field"`
	InputImageCountUnitPrice        float64        `json:"input_image_count_unit_price"`
	InputImageCountSurchargeEnabled bool           `json:"input_image_count_surcharge_enabled"`
	ImageCountSurcharge             float64        `json:"image_count_surcharge"`
	PriceBeforeSurcharge            float64        `json:"price_before_surcharge"`
	InputArgs                       map[string]any `json:"input_args,omitempty"`
}

// quoteResponse mirrors the Lovart LGW pricing API response envelope.
type quoteResponse struct {
	Code int `json:"code"`
	Data struct {
		Price       float64     `json:"price"`
		Balance     float64     `json:"balance"`
		PriceDetail PriceDetail `json:"price_detail"`
	} `json:"data"`
}

// Quote fetches a live credit quote from the Lovart pricing API.
// The body should contain the generation parameters (prompt, size, quality, n, etc.).
func Quote(ctx context.Context, client *http.Client, model string, body map[string]any) (*QuoteResult, error) {
	return QuoteWithOptions(ctx, client, model, body, QuoteOptions{Mode: ModeAuto})
}

func baseQuote(ctx context.Context, client *http.Client, model string, body map[string]any) (*QuoteResult, error) {
	path := "/v1/generator/pricing"

	reqBody := map[string]any{
		"generator_name": model,
	}
	if len(body) > 0 {
		reqBody["input_args"] = body
	}

	var resp quoteResponse
	if err := client.PostJSON(ctx, http.LGWBase, path, reqBody, &resp); err != nil {
		return nil, fmt.Errorf("pricing: quote: %w", err)
	}

	if resp.Code != 0 {
		return nil, fmt.Errorf("pricing: quote returned code %d", resp.Code)
	}

	return &QuoteResult{
		Price:       resp.Data.Price,
		Balance:     resp.Data.Balance,
		PriceDetail: resp.Data.PriceDetail,
	}, nil
}

// Balance returns the user's current credit balance via a minimal pricing request.
func Balance(ctx context.Context, client *http.Client) (float64, error) {
	result, err := Quote(ctx, client, "openai/gpt-image-2", map[string]any{
		"prompt":  "",
		"quality": "low",
		"size":    "1024*1024",
		"n":       1,
	})
	if err != nil {
		return 0, fmt.Errorf("pricing: balance: %w", err)
	}
	return result.Balance, nil
}
