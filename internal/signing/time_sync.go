package signing

import (
	"encoding/json"
	"fmt"
	"io"
	nethttp "net/http"
	"time"
)

const timeSyncURL = "https://www.lovart.ai/api/www/lovart/time/utc/timestamp"

// SyncTime fetches the Lovart server time via HTTP and computes the offset
// between server and local clock in milliseconds.
//
// Returns the offset to add to local timestamps: timestamp = local_ms + offset_ms.
func SyncTime() (int64, error) {
	before := time.Now().UnixMilli()

	resp, err := nethttp.Get(fmt.Sprintf("%s?_t=%d", timeSyncURL, before))
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
		return 0, fmt.Errorf("signing: parse time sync response: %w", err)
	}

	serverTS := result.Data.Timestamp
	if serverTS == 0 {
		return 0, fmt.Errorf("signing: time sync returned zero timestamp")
	}

	// offset = server_time - (local_before + rtt/2)
	offset := serverTS - (before + rtt/2)
	return offset, nil
}

// TimestampNowMS returns the current Unix millisecond timestamp adjusted by the given offset.
func TimestampNowMS(offsetMS int64) string {
	return fmt.Sprintf("%d", time.Now().UnixMilli()+offsetMS)
}
