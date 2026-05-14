package jobs

import (
	"context"
	"fmt"
	"math"
	"os"

	"github.com/aporicho/lovart/internal/config"
	"github.com/aporicho/lovart/internal/http"
	"github.com/aporicho/lovart/internal/pricing"
	"github.com/schollz/progressbar/v3"
)

// QuoteSummary holds the aggregated batch pricing result.
type QuoteSummary struct {
	TotalJobs       int        `json:"total_jobs"`
	TotalPrice      float64    `json:"total_price"`
	Balance         float64    `json:"balance"`
	BalanceAfter    float64    `json:"balance_after"`
	CanAfford       bool       `json:"can_afford"`
	SignatureGroups int        `json:"signature_groups"`
	QuotedRequests  int        `json:"quoted_requests"`
	CacheHits       int        `json:"cache_hits"`
	Jobs            []JobQuote `json:"jobs"`
}

// JobQuote is the pricing result for a single job.
type JobQuote struct {
	JobID          string                  `json:"job_id"`
	Title          string                  `json:"title,omitempty"`
	Model          string                  `json:"model"`
	Outputs        int                     `json:"outputs"`
	ActualOutputs  int                     `json:"actual_outputs,omitempty"`
	APICalls       int                     `json:"api_calls"`
	Price          float64                 `json:"price"`
	Cached         bool                    `json:"cached"`
	NormalizedBody map[string]any          `json:"normalized_body,omitempty"`
	PriceDetail    *pricing.PriceDetail    `json:"price_detail,omitempty"`
	PricingContext *pricing.PricingContext `json:"pricing_context,omitempty"`
}

// QuoteJobs runs batch pricing for all jobs in a JSONL file.
// A visual progress bar is shown on stderr. Pass noProgress=true to suppress.
func QuoteJobs(ctx context.Context, client *http.Client, jobsFile string, noProgress bool) (*QuoteSummary, error) {
	jobs, validation, err := PrepareJobsFile(jobsFile)
	if err != nil {
		return nil, fmt.Errorf("jobs pricing: %w", err)
	}
	if validation != nil {
		return nil, validation
	}
	return QuotePreparedJobs(ctx, client, jobs, noProgress)
}

// QuotePreparedJobs runs batch pricing for already parsed and validated jobs.
func QuotePreparedJobs(ctx context.Context, client *http.Client, jobs []JobLine, noProgress bool) (*QuoteSummary, error) {
	if len(jobs) == 0 {
		return &QuoteSummary{}, nil
	}

	totalJobs := len(jobs)

	var bar *progressbar.ProgressBar
	if !noProgress {
		bar = progressbar.NewOptions(totalJobs,
			progressbar.OptionSetWriter(os.Stderr),
			progressbar.OptionSetDescription("报价中..."),
			progressbar.OptionSetTheme(progressbar.ThemeDefault),
			progressbar.OptionFullWidth(),
			progressbar.OptionShowCount(),
			progressbar.OptionSetPredictTime(true),
			progressbar.OptionShowElapsedTimeOnFinish(),
			progressbar.OptionOnCompletion(func() {
				fmt.Fprint(os.Stderr, "\n")
			}),
		)
	}

	cache := NewQuoteCache()
	summary := &QuoteSummary{
		TotalJobs: totalJobs,
	}
	var balance float64

	for i, job := range jobs {
		normalizedJob, err := normalizeJobLine(job)
		if err != nil {
			return nil, fmt.Errorf("jobs pricing: job %q: normalize request defaults: %w", job.JobID, err)
		}
		sig := CostSignature(normalizedJob)

		var result *pricing.QuoteResult
		var cached bool

		if r, ok := cache.Get(sig); ok {
			result = r
			cached = true
			summary.CacheHits++
		} else {
			subs, _ := Expand(normalizedJob.Model, normalizedJob.Outputs, normalizedJob.Body)
			apiCalls := len(subs)

			repBody := normalizedJob.Body
			if len(subs) > 0 {
				repBody = subs[0].Body
			}

			r, err := pricing.QuoteWithOptions(ctx, client, normalizedJob.Model, repBody, pricing.QuoteOptions{Mode: normalizedJob.Mode})
			if err != nil {
				return nil, fmt.Errorf("jobs pricing: job %q: %w", job.JobID, err)
			}
			if len(r.NormalizedBody) == 0 {
				r.NormalizedBody = repBody
			}
			result = r
			summary.QuotedRequests++

			if i == 0 {
				balance = result.Balance
			}

			jobPrice := computeJobPrice(result, apiCalls, job.Model, job.Outputs)
			priceContext := scalePricingContext(result.PricingContext, result.Price, jobPrice)
			result = &pricing.QuoteResult{
				Price:          jobPrice,
				Balance:        result.Balance,
				PriceDetail:    result.PriceDetail,
				NormalizedBody: result.NormalizedBody,
				PricingContext: priceContext,
			}

			cache.Set(sig, result)
		}

		subs, _ := Expand(normalizedJob.Model, normalizedJob.Outputs, normalizedJob.Body)
		apiCalls := len(subs)
		actualOutputs := computeActualOutputs(normalizedJob.Model, normalizedJob.Outputs, subs)

		summary.Jobs = append(summary.Jobs, JobQuote{
			JobID:          job.JobID,
			Title:          job.Title,
			Model:          job.Model,
			Outputs:        job.Outputs,
			ActualOutputs:  actualOutputs,
			APICalls:       apiCalls,
			Price:          result.Price,
			Cached:         cached,
			NormalizedBody: result.NormalizedBody,
			PriceDetail:    &result.PriceDetail,
			PricingContext: result.PricingContext,
		})
		summary.TotalPrice += result.Price

		if bar != nil {
			status := " quote"
			if cached {
				status = " cache"
			}
			bar.Describe(fmt.Sprintf("%s (%d/%d) %d cached %d quoted%s",
				job.Model, i+1, totalJobs, summary.CacheHits, summary.QuotedRequests, status))
			bar.Add(1)
		}
	}

	if bar != nil {
		bar.Finish()
		fmt.Fprintf(os.Stderr, "  报价完成: %d 个 job, %d 个签名组, %d 次 API 调用, %d 次缓存命中, 总价 %.0f 积分\n",
			totalJobs, cache.Size(), summary.QuotedRequests, summary.CacheHits, summary.TotalPrice)
	}

	summary.Balance = balance
	summary.BalanceAfter = balance - summary.TotalPrice
	summary.CanAfford = summary.BalanceAfter >= 0
	summary.SignatureGroups = cache.Size()

	return summary, nil
}

// computeJobPrice calculates the total price for a job.
func computeJobPrice(quote *pricing.QuoteResult, apiCalls int, model string, outputs int) float64 {
	cap := config.OutputCapability(model)

	if cap.IsFixedBatch {
		batchCount := int(math.Ceil(float64(outputs) / float64(cap.BatchSize)))
		return float64(batchCount) * quote.Price
	}

	if cap.MultiField != "" {
		if apiCalls <= 1 {
			return quote.Price
		}
		return quote.Price
	}

	return float64(outputs) * quote.Price
}

func scalePricingContext(pc *pricing.PricingContext, basePrice, totalPrice float64) *pricing.PricingContext {
	if pc == nil {
		return nil
	}
	scaled := *pc
	if basePrice > 0 {
		factor := totalPrice / basePrice
		scaled.ServerPrice *= factor
		scaled.NominalPrice *= factor
	}
	scaled.EffectivePrice = totalPrice
	return &scaled
}

// computeActualOutputs returns the actual number of images that will be produced.
func computeActualOutputs(model string, outputs int, subs []SubRequest) int {
	cap := config.OutputCapability(model)
	if cap.IsFixedBatch {
		return len(subs) * cap.BatchSize
	}
	n := 0
	for _, s := range subs {
		n += s.N
	}
	return n
}
