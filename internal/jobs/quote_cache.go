package jobs

import "github.com/aporicho/lovart/internal/pricing"

// QuoteCache stores pricing results keyed by cost signature.
type QuoteCache struct {
	entries map[string]*pricing.QuoteResult
}

// NewQuoteCache creates an empty pricing cache.
func NewQuoteCache() *QuoteCache {
	return &QuoteCache{
		entries: make(map[string]*pricing.QuoteResult),
	}
}

// Get returns a cached quote if available.
func (c *QuoteCache) Get(signature string) (*pricing.QuoteResult, bool) {
	r, ok := c.entries[signature]
	return r, ok
}

// Set stores a quote result.
func (c *QuoteCache) Set(signature string, result *pricing.QuoteResult) {
	c.entries[signature] = result
}

// Size returns the number of cached entries.
func (c *QuoteCache) Size() int {
	return len(c.entries)
}
