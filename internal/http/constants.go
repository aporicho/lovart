package http

// Base URLs for Lovart APIs.
const (
	WWWBase  = "https://www.lovart.ai"
	LGWBase   = "https://lgw.lovart.ai"
	CanvaBase = "https://www.lovart.ai/api/canva"
)

// Default headers sent with every Lovart request.
var defaultHeaders = map[string]string{
	"Content-Type": "application/json",
	"Accept":       "application/json, text/plain, */*",
	"Origin":       WWWBase,
	"Referer":      WWWBase + "/canvas",
	"User-Agent":   "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/146.0.0.0 Safari/537.36",
}
