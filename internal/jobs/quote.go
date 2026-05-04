package jobs

import (
	"context"
	"fmt"
	"io"
	"math"

	"github.com/aporicho/lovart/internal/config"
	"github.com/aporicho/lovart/internal/http"
	"github.com/aporicho/lovart/internal/pricing"
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
	JobID         string               `json:"job_id"`
	Title         string               `json:"title,omitempty"`
	Model         string               `json:"model"`
	Outputs       int                  `json:"outputs"`
	ActualOutputs int                  `json:"actual_outputs,omitempty"`
	APICalls      int                  `json:"api_calls"`
	Price         float64              `json:"price"`
	Cached        bool                 `json:"cached"`
	PriceDetail   *pricing.PriceDetail `json:"price_detail,omitempty"`
}

// QuoteJobs runs batch pricing for all jobs in a JSONL file.
// Progress is written to progress (can be nil). Results are returned in the summary.
func QuoteJobs(ctx context.Context, client *http.Client, jobsFile string, progress io.Writer) (*QuoteSummary, error) {
	jobs, err := ParseJobsFile(jobsFile)
	if err != nil {
		return nil, fmt.Errorf("jobs quote: %w", err)
	}

	if len(jobs) == 0 {
		return &QuoteSummary{}, nil
	}

	cache := NewQuoteCache()
	summary := &QuoteSummary{
		TotalJobs: len(jobs),
	}
	var balance float64

	totalJobs := len(jobs)

	for i, job := range jobs {
		sig := CostSignature(job)

		var result *pricing.QuoteResult
		var cached bool

		if r, ok := cache.Get(sig); ok {
			result = r
			cached = true
			summary.CacheHits++
		} else {
			subs, _ := Expand(job.Model, job.Outputs, job.Body)
			apiCalls := len(subs)

			repBody := job.Body
			if len(subs) > 0 {
				repBody = subs[0].Body
			}

			r, err := pricing.Quote(ctx, client, job.Model, repBody)
			if err != nil {
				return nil, fmt.Errorf("jobs quote: job %q: %w", job.JobID, err)
			}
			result = r
			summary.QuotedRequests++

			if i == 0 {
				balance = result.Balance
			}

			jobPrice := computeJobPrice(result, apiCalls, job.Model, job.Outputs)
			result = &pricing.QuoteResult{
				Price:       jobPrice,
				Balance:     result.Balance,
				PriceDetail: result.PriceDetail,
			}

			cache.Set(sig, result)
		}

		subs, _ := Expand(job.Model, job.Outputs, job.Body)
		apiCalls := len(subs)
		actualOutputs := computeActualOutputs(job.Model, job.Outputs, subs)

		summary.Jobs = append(summary.Jobs, JobQuote{
			JobID:         job.JobID,
			Title:         job.Title,
			Model:         job.Model,
			Outputs:       job.Outputs,
			ActualOutputs: actualOutputs,
			APICalls:      apiCalls,
			Price:         result.Price,
			Cached:        cached,
			PriceDetail:   &result.PriceDetail,
		})
		summary.TotalPrice += result.Price

		// Progress to stderr.
		if progress != nil {
			status := "quote"
			if cached {
				status = "cache"
			}
			fmt.Fprintf(progress, "\r  报价中 [%d/%d] %s (%s)  cached: %d | quoted: %d",
				i+1, totalJobs, status, job.Model, summary.CacheHits, summary.QuotedRequests)
		}
	}

	if progress != nil {
		fmt.Fprintf(progress, "\n")
		fmt.Fprintf(progress, "  报价完成: %d 个 job, %d 个签名组, %d 次 API 调用, %d 次缓存命中, 总价 %.0f 积分\n",
			totalJobs, cache.Size(), summary.QuotedRequests, summary.CacheHits, summary.TotalPrice)
	}

	summary.Balance = balance
	summary.BalanceAfter = balance - summary.TotalPrice
	summary.CanAfford = summary.BalanceAfter >= 0
	summary.SignatureGroups = cache.Size()

	return summary, nil
}

// computeJobPrice calculates the total price for a job given the representative
// unit price and the number of API calls needed.
func computeJobPrice(quote *pricing.QuoteResult, apiCalls int, model string, outputs int) float64 {
	cap := config.OutputCapability(model)

	if cap.IsFixedBatch {
		batchCount := int(math.Ceil(float64(outputs) / float64(cap.BatchSize)))
		return float64(batchCount) * quote.PriceDetail.UnitPrice
	}

	if cap.MultiField != "" {
		if apiCalls <= 1 {
			return quote.Price
		}
		return quote.Price
	}

	return float64(outputs) * quote.PriceDetail.UnitPrice
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
