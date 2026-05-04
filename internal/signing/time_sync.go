package signing

import (
	"encoding/json"
	"fmt"
	"io"
	nethttp "net/http"
	"time"
)

const timeSyncURL = "https://www.lovart.ai/api/www/lovart/time/utc/timestamp"

// SyncTime fetches the Lovart server time and computes the offset between
// server and local clock in milliseconds. The cookie header is required by Lovart.
//
// Returns the offset to add to local timestamps: timestamp = local_ms + offset_ms.
func SyncTime(cookie string) (int64, error) {
	before := time.Now().UnixMilli()

	req, err := nethttp.NewRequest("GET", fmt.Sprintf("%s?_t=%d", timeSyncURL, before), nil)
	if err != nil {
		return 0, fmt.Errorf("signing: time sync request: %w", err)
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36")
	req.Header.Set("Accept", "application/json, text/plain, */*")
	req.Header.Set("Origin", "https://www.lovart.ai")
	req.Header.Set("Referer", "https://www.lovart.ai/canvas")
	if cookie != "" {
		req.Header.Set("Cookie", cookie)
	}

	resp, err := nethttp.DefaultClient.Do(req)
	if err != nil {
		return 0, fmt.Errorf("signing: time sync request: %w", err)
	}
	defer resp.Body.Close()

	after := time.Now().UnixMilli()
	rtt := after - before

	data, err := io.ReadAll(io.LimitReader(resp.Body, 4096))
	if err != nil {
		return 0, fmt.Errorf("signing: read time sync response: %w", err)
	}

	var result struct {
		Data struct {
			Timestamp int64 `json:"timestamp"`
		} `json:"data"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return 0, fmt.Errorf("signing: parse time sync response: %w (body: %.200s)", err, string(data))
	}

	serverTS := result.Data.Timestamp
	if serverTS == 0 {
		return 0, fmt.Errorf("signing: time sync returned zero timestamp (body: %.200s)", string(data))
	}

	// offset = server_time - (local_before + rtt/2)
	offset := serverTS - (before + rtt/2)
	return offset, nil
}

// TimestampNowMS returns the current Unix millisecond timestamp adjusted by the given offset.
func TimestampNowMS(offsetMS int64) string {
	return fmt.Sprintf("%d", time.Now().UnixMilli()+offsetMS)
}
