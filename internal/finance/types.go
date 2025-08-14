package finance

import (
	"time"
)

// yahooChartResp mirrors Yahoo v8 chart response (trimmed to needed fields)
type yahooChartResp struct {
	Chart struct {
		Result []struct {
			Meta struct {
				GmtOffset int    `json:"gmtoffset"`
				Timezone  string `json:"timezone"`
			} `json:"meta"`
			Timestamp  []int64 `json:"timestamp"`
			Indicators struct {
				Quote []struct {
					Close []float64 `json:"close"`
				} `json:"quote"`
			} `json:"indicators"`
		} `json:"result"`
		Error any `json:"error"`
	} `json:"chart"`
}

// yahooSparkResp mirrors Yahoo v7 spark fallback (trimmed)
type yahooSparkResp struct {
	Spark struct {
		Result []struct {
			Symbol   string `json:"symbol"`
			Response []struct {
				Timestamp []int64   `json:"timestamp"`
				Close     []float64 `json:"close"`
			} `json:"response"`
		} `json:"result"`
		Error any `json:"error"`
	} `json:"spark"`
}

// Chart image cache entry
type chartCacheEntry struct {
	createdAt time.Time
	image     []byte
}

const chartCacheTTL = 60 * time.Second
