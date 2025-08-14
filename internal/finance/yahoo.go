package finance

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"sync"

	"github.com/vicanso/go-charts/v2"
)

type yahooChartResp struct {
	Chart struct {
		Result []struct {
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

// spark (v7) fallback schema
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

type chartCacheEntry struct {
	createdAt time.Time
	image     []byte
}

var (
	chartCache   = map[string]chartCacheEntry{}
	chartCacheMu sync.Mutex
)

const chartCacheTTL = 60 * time.Second

// fetch5mSeries fetches 5m timestamps and close prices for a single symbol and window range.
func fetch5mSeries(symbol string, rangeParam string) ([]int64, []float64, error) {
	hosts := []string{"query1.finance.yahoo.com", "query2.finance.yahoo.com"}
	backoffs := []time.Duration{200 * time.Millisecond, 500 * time.Millisecond, 1 * time.Second}
	var yc yahooChartResp
	var lastErr error
	for attempt := 0; attempt < len(backoffs)+1; attempt++ {
		for _, host := range hosts {
			url := fmt.Sprintf("https://%s/v8/finance/chart/%s?range=%s&interval=5m&includePrePost=true&events=div,splits", host, symbol, rangeParam)
			req, _ := http.NewRequest("GET", url, nil)
			req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.4 Safari/605.1.15")
			req.Header.Set("Accept", "application/json, text/javascript, */*; q=0.01")
			req.Header.Set("Accept-Language", "en-US,en;q=0.9")
			req.Header.Set("Referer", fmt.Sprintf("https://finance.yahoo.com/quote/%s/chart", strings.ToUpper(symbol)))
			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				lastErr = err
				continue
			}
			body, readErr := io.ReadAll(resp.Body)
			resp.Body.Close()
			if readErr != nil {
				lastErr = fmt.Errorf("failed to read yahoo response: %w", readErr)
				continue
			}
			if resp.StatusCode == http.StatusTooManyRequests || strings.HasPrefix(string(body), "Edge: Too Many Requests") {
				lastErr = fmt.Errorf("yahoo %s returned 429: Edge: Too Many Requests", host)
				continue
			}
			if resp.StatusCode != http.StatusOK {
				preview := string(body)
				if len(preview) > 120 {
					preview = preview[:120]
				}
				lastErr = fmt.Errorf("yahoo %s returned %d: %s", host, resp.StatusCode, preview)
				continue
			}
			if strings.HasPrefix(string(body), "<") || strings.HasPrefix(string(body), "Edge:") {
				preview := string(body)
				if len(preview) > 120 {
					preview = preview[:120]
				}
				lastErr = fmt.Errorf("yahoo returned non-json body: %s", preview)
				continue
			}
			if err := json.Unmarshal(body, &yc); err != nil {
				preview := string(body)
				if len(preview) > 120 {
					preview = preview[:120]
				}
				lastErr = fmt.Errorf("failed to parse yahoo json: %v; body: %s", err, preview)
				continue
			}
			lastErr = nil
			break
		}
		if lastErr == nil {
			break
		}
		if attempt < len(backoffs) {
			time.Sleep(backoffs[attempt])
		}
	}
	if lastErr != nil {
		// Spark fallback
		var sp yahooSparkResp
		for attempt := 0; attempt < len(backoffs)+1 && lastErr != nil; attempt++ {
			for _, host := range hosts {
				url := fmt.Sprintf("https://%s/v7/finance/spark?symbols=%s&range=%s&interval=5m", host, strings.ToUpper(symbol), rangeParam)
				req, _ := http.NewRequest("GET", url, nil)
				req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.4 Safari/605.1.15")
				req.Header.Set("Accept", "application/json, text/javascript, */*; q=0.01")
				req.Header.Set("Accept-Language", "en-US,en;q=0.9")
				req.Header.Set("Referer", fmt.Sprintf("https://finance.yahoo.com/quote/%s/chart", strings.ToUpper(symbol)))
				resp, err := http.DefaultClient.Do(req)
				if err != nil {
					lastErr = err
					continue
				}
				body, readErr := io.ReadAll(resp.Body)
				resp.Body.Close()
				if readErr != nil {
					lastErr = fmt.Errorf("failed to read yahoo spark response: %w", readErr)
					continue
				}
				if resp.StatusCode == http.StatusTooManyRequests || strings.HasPrefix(string(body), "Edge: Too Many Requests") {
					lastErr = fmt.Errorf("yahoo %s returned 429 on spark", host)
					continue
				}
				if resp.StatusCode != http.StatusOK {
					preview := string(body)
					if len(preview) > 120 {
						preview = preview[:120]
					}
					lastErr = fmt.Errorf("yahoo %s spark returned %d: %s", host, resp.StatusCode, preview)
					continue
				}
				if strings.HasPrefix(string(body), "<") {
					lastErr = errors.New("yahoo spark returned non-json body")
					continue
				}
				if err := json.Unmarshal(body, &sp); err != nil {
					lastErr = fmt.Errorf("failed to parse yahoo spark json: %v", err)
					continue
				}
				if len(sp.Spark.Result) > 0 && len(sp.Spark.Result[0].Response) > 0 {
					ts := sp.Spark.Result[0].Response[0].Timestamp
					cl := sp.Spark.Result[0].Response[0].Close
					return ts, cl, nil
				}
			}
			if attempt < len(backoffs) {
				time.Sleep(backoffs[attempt])
			}
		}
		if lastErr != nil {
			return nil, nil, lastErr
		}
	}
	if len(yc.Chart.Result) == 0 || len(yc.Chart.Result[0].Indicators.Quote) == 0 {
		return nil, nil, errors.New("no data")
	}
	return yc.Chart.Result[0].Timestamp, yc.Chart.Result[0].Indicators.Quote[0].Close, nil
}

// Make5mChart generates a 5-minute chart for the given symbol and time window.
// window accepts: "1d" (default), "1w", "1m". It maps to Yahoo ranges of 1d, 5d, 1mo respectively.
func Make5mChart(symbol string, window ...string) ([]byte, error) {
	// Normalize window
	w := "1d"
	if len(window) > 0 && window[0] != "" {
		lw := strings.ToLower(strings.TrimSpace(window[0]))
		switch lw {
		case "1d", "day", "1day":
			w = "1d"
		case "1w", "1wk", "week", "1week":
			w = "1w"
		case "1m", "1mo", "month", "1month":
			w = "1m"
		default:
			w = "1d"
		}
	}

	// Map window to Yahoo range param
	rangeParam := "1d"
	switch w {
	case "1d":
		rangeParam = "1d"
	case "1w":
		rangeParam = "5d"
	case "1m":
		rangeParam = "1mo"
	}
	// Serve from cache if recent
	cacheKey := strings.ToUpper(symbol) + "|" + w
	chartCacheMu.Lock()
	if entry, ok := chartCache[cacheKey]; ok {
		if time.Since(entry.createdAt) < chartCacheTTL {
			img := make([]byte, len(entry.image))
			copy(img, entry.image)
			chartCacheMu.Unlock()
			return img, nil
		}
	}
	chartCacheMu.Unlock()

	// Try multiple Yahoo hosts and retry with backoff on transient errors/rate limits
	hosts := []string{"query1.finance.yahoo.com", "query2.finance.yahoo.com"}
	backoffs := []time.Duration{200 * time.Millisecond, 500 * time.Millisecond, 1 * time.Second}
	var yc yahooChartResp
	var lastErr error

	for attempt := 0; attempt < len(backoffs)+1; attempt++ {
		for _, host := range hosts {
			url := fmt.Sprintf("https://%s/v8/finance/chart/%s?range=%s&interval=5m&includePrePost=true&events=div,splits", host, symbol, rangeParam)
			req, _ := http.NewRequest("GET", url, nil)
			// Use browser-like headers to reduce edge 429s
			req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.4 Safari/605.1.15")
			req.Header.Set("Accept", "application/json, text/javascript, */*; q=0.01")
			req.Header.Set("Accept-Language", "en-US,en;q=0.9")
			req.Header.Set("Referer", fmt.Sprintf("https://finance.yahoo.com/quote/%s/chart", strings.ToUpper(symbol)))

			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				lastErr = err
				continue
			}
			body, readErr := io.ReadAll(resp.Body)
			resp.Body.Close()
			if readErr != nil {
				lastErr = fmt.Errorf("failed to read yahoo response: %w", readErr)
				continue
			}
			// Detect rate limiting early
			if resp.StatusCode == http.StatusTooManyRequests || strings.HasPrefix(string(body), "Edge: Too Many Requests") {
				lastErr = fmt.Errorf("yahoo %s returned 429: Edge: Too Many Requests", host)
				continue
			}
			if resp.StatusCode != http.StatusOK {
				preview := string(body)
				if len(preview) > 120 {
					preview = preview[:120]
				}
				lastErr = fmt.Errorf("yahoo %s returned %d: %s", host, resp.StatusCode, preview)
				continue
			}
			// Guard against HTML/text
			if strings.HasPrefix(string(body), "<") || strings.HasPrefix(string(body), "Edge:") {
				preview := string(body)
				if len(preview) > 120 {
					preview = preview[:120]
				}
				lastErr = fmt.Errorf("yahoo returned non-json body: %s", preview)
				continue
			}
			if err := json.Unmarshal(body, &yc); err != nil {
				preview := string(body)
				if len(preview) > 120 {
					preview = preview[:120]
				}
				lastErr = fmt.Errorf("failed to parse yahoo json: %v; body: %s", err, preview)
				continue
			}
			// Success
			lastErr = nil
			break
		}
		if lastErr == nil {
			break
		}
		if attempt < len(backoffs) {
			time.Sleep(backoffs[attempt])
		}
	}
	if lastErr != nil {
		// Fallback to v7 spark endpoint
		spURLHosts := hosts
		var sp yahooSparkResp
		for attempt := 0; attempt < len(backoffs)+1 && lastErr != nil; attempt++ {
			for _, host := range spURLHosts {
				url := fmt.Sprintf("https://%s/v7/finance/spark?symbols=%s&range=%s&interval=5m", host, strings.ToUpper(symbol), rangeParam)
				req, _ := http.NewRequest("GET", url, nil)
				req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.4 Safari/605.1.15")
				req.Header.Set("Accept", "application/json, text/javascript, */*; q=0.01")
				req.Header.Set("Accept-Language", "en-US,en;q=0.9")
				req.Header.Set("Referer", fmt.Sprintf("https://finance.yahoo.com/quote/%s/chart", strings.ToUpper(symbol)))
				resp, err := http.DefaultClient.Do(req)
				if err != nil {
					lastErr = err
					continue
				}
				body, readErr := io.ReadAll(resp.Body)
				resp.Body.Close()
				if readErr != nil {
					lastErr = fmt.Errorf("failed to read yahoo spark response: %w", readErr)
					continue
				}
				if resp.StatusCode == http.StatusTooManyRequests || strings.HasPrefix(string(body), "Edge: Too Many Requests") {
					lastErr = fmt.Errorf("yahoo %s returned 429 on spark", host)
					continue
				}
				if resp.StatusCode != http.StatusOK {
					preview := string(body)
					if len(preview) > 120 {
						preview = preview[:120]
					}
					lastErr = fmt.Errorf("yahoo %s spark returned %d: %s", host, resp.StatusCode, preview)
					continue
				}
				if strings.HasPrefix(string(body), "<") {
					lastErr = errors.New("yahoo spark returned non-json body")
					continue
				}
				if err := json.Unmarshal(body, &sp); err != nil {
					lastErr = fmt.Errorf("failed to parse yahoo spark json: %v", err)
					continue
				}
				if len(sp.Spark.Result) > 0 && len(sp.Spark.Result[0].Response) > 0 {
					// Convert spark result to chart result structure
					yc.Chart.Result = []struct {
						Timestamp  []int64 `json:"timestamp"`
						Indicators struct {
							Quote []struct {
								Close []float64 `json:"close"`
							} `json:"quote"`
						} `json:"indicators"`
					}{
						{
							Timestamp: sp.Spark.Result[0].Response[0].Timestamp,
							Indicators: struct {
								Quote []struct {
									Close []float64 `json:"close"`
								} `json:"quote"`
							}{
								Quote: []struct {
									Close []float64 `json:"close"`
								}{
									{Close: sp.Spark.Result[0].Response[0].Close},
								},
							},
						},
					}
					lastErr = nil
					break
				}
			}
			if lastErr == nil {
				break
			}
			if attempt < len(backoffs) {
				time.Sleep(backoffs[attempt])
			}
		}
		if lastErr != nil {
			return nil, lastErr
		}
	}
	if len(yc.Chart.Result) == 0 || len(yc.Chart.Result[0].Indicators.Quote) == 0 {
		return nil, errors.New("no data")
	}

	t := yc.Chart.Result[0].Timestamp
	c := yc.Chart.Result[0].Indicators.Quote[0].Close
	if len(t) == 0 || len(c) == 0 {
		return nil, errors.New("empty bars")
	}

	// Prepare data for chart
	var xAxisData []string
	var yAxisData []float64
	var xAllLabels []string
	var yMin, yMax float64
	firstPoint := true

	// Build labels with local time and compute y-range
	for i := range t {
		if i >= len(c) || c[i] == 0 {
			continue
		}
		tt := time.Unix(t[i], 0).In(time.Local)
		// Richer labels to allow auto-thinning while keeping hour+date context
		switch w {
		case "1d":
			xAllLabels = append(xAllLabels, tt.Format("15:04"))
		case "1w":
			xAllLabels = append(xAllLabels, tt.Format("Jan 02 15:04"))
		default: // 1m
			xAllLabels = append(xAllLabels, tt.Format("Jan 02 15:04"))
		}
		xAxisData = append(xAxisData, tt.Format("15:04"))
		price := c[i]
		yAxisData = append(yAxisData, price)
		if firstPoint {
			yMin, yMax = price, price
			firstPoint = false
		} else {
			if price < yMin {
				yMin = price
			}
			if price > yMax {
				yMax = price
			}
		}
	}

	if len(xAxisData) == 0 {
		return nil, errors.New("no valid data points")
	}

	// Determine y-axis bounds with padding
	if len(yAxisData) > 1 {
		pad := (yMax - yMin) * 0.05
		if pad < yMax*0.002 {
			pad = yMax * 0.002
		}
		yMin = yMin - pad
		if yMin < 0 {
			yMin = 0
		}
		yMax = yMax + pad
	}

	// X-axis split recommendation based on window
	split := 8
	switch w {
	case "1d":
		split = 8
	case "1w":
		split = 7
	case "1m":
		split = 10
	}

	// Create line chart with improved axes
	painter, err := charts.LineRender([][]float64{yAxisData},
		charts.TitleTextOptionFunc(strings.ToUpper(symbol)+" • 5m • "+strings.ToUpper(w)),
		charts.XAxisOptionFunc(charts.XAxisOption{Data: xAllLabels, BoundaryGap: charts.FalseFlag(), SplitNumber: split}),
		charts.YAxisOptionFunc(charts.YAxisOption{Min: &yMin, Max: &yMax, DivideCount: 5}),
		charts.ThemeOptionFunc(charts.ThemeLight),
	)
	if err != nil {
		return nil, err
	}

	// Get the chart as PNG bytes
	imgBytes, err := painter.Bytes()
	if err != nil {
		return nil, err
	}

	// Save to cache
	chartCacheMu.Lock()
	chartCache[strings.ToUpper(symbol)] = chartCacheEntry{createdAt: time.Now(), image: imgBytes}
	chartCacheMu.Unlock()

	return imgBytes, nil
}

// MakeMulti5mChart renders multiple symbols in one chart with legends and two y-axes if needed.
// window: "1d" | "1w" | "1m" (same mapping as Make5mChart)
func MakeMulti5mChart(symbols []string, window ...string) ([]byte, error) {
	if len(symbols) == 0 {
		return nil, errors.New("no symbols provided")
	}
	// Normalize window and range mapping
	w := "1d"
	if len(window) > 0 && window[0] != "" {
		lw := strings.ToLower(strings.TrimSpace(window[0]))
		switch lw {
		case "1d", "day", "1day":
			w = "1d"
		case "1w", "1wk", "week", "1week":
			w = "1w"
		case "1m", "1mo", "month", "1month":
			w = "1m"
		}
	}
	rangeParam := "1d"
	switch w {
	case "1d":
		rangeParam = "1d"
	case "1w":
		rangeParam = "5d"
	case "1m":
		rangeParam = "1mo"
	}

	// Fetch all series (sequentially to avoid rate limits)
	type seriesData struct {
		sym string
		ts  []int64
		cl  []float64
	}
	fetched := make([]seriesData, 0, len(symbols))
	for _, s := range symbols {
		s = strings.TrimSpace(s)
		if s == "" {
			continue
		}
		ts, cl, err := fetch5mSeries(s, rangeParam)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", s, err)
		}
		fetched = append(fetched, seriesData{sym: strings.ToUpper(s), ts: ts, cl: cl})
		time.Sleep(120 * time.Millisecond) // gentle spacing to reduce 429s
	}
	if len(fetched) == 0 {
		return nil, errors.New("no series fetched")
	}

	// Determine the reference timeline: pick the series with the most points
	ref := fetched[0]
	for _, sd := range fetched[1:] {
		if len(sd.ts) > len(ref.ts) {
			ref = sd
		}
	}

	// Build x labels based on ref timeline
	xLabels := make([]string, 0, len(ref.ts))
	for _, ts := range ref.ts {
		tt := time.Unix(ts, 0).In(time.Local)
		if w == "1d" {
			xLabels = append(xLabels, tt.Format("15:04"))
		} else {
			xLabels = append(xLabels, tt.Format("Jan 02 15:04"))
		}
	}

	// Align each series to ref timeline by index (Yahoo returns aligned bars most of the time)
	// For simple alignment, we will truncate to the min length among series
	minLen := len(ref.ts)
	for _, sd := range fetched {
		if len(sd.cl) < minLen {
			minLen = len(sd.cl)
		}
	}
	if minLen < 2 {
		return nil, errors.New("not enough data points")
	}
	xLabels = xLabels[len(xLabels)-minLen:]

	// Prepare values and either normalize all to percent change when >2, or keep raw with dual axes when 1-2
	values := make([][]float64, 0, len(fetched))
	names := make([]string, 0, len(fetched))
	normalized := len(fetched) > 2
	// y-range collectors
	var leftMin, leftMax, rightMin, rightMax *float64
	var commonMin, commonMax *float64

	for i, sd := range fetched {
		clOrig := sd.cl[len(sd.cl)-minLen:]
		var cl []float64
		if normalized {
			// percent change from first non-zero bar
			base := 0.0
			for _, v := range clOrig {
				if v != 0 {
					base = v
					break
				}
			}
			if base == 0 {
				// fallback: use first value even if zero to avoid div-by-zero (will remain zeros)
				base = clOrig[0]
				if base == 0 {
					base = 1
				}
			}
			cl = make([]float64, len(clOrig))
			for j, v := range clOrig {
				cl[j] = (v/base - 1.0) * 100.0
			}
			// track common min/max
			for _, v := range cl {
				if commonMin == nil || v < *commonMin {
					vv := v
					commonMin = &vv
				}
				if commonMax == nil || v > *commonMax {
					vv := v
					commonMax = &vv
				}
			}
		} else {
			cl = clOrig
			// track per-side ranges
			mn, mx := cl[0], cl[0]
			for _, v := range cl[1:] {
				if v < mn {
					mn = v
				}
				if v > mx {
					mx = v
				}
			}
			pad := (mx - mn) * 0.05
			if pad < mx*0.002 {
				pad = mx * 0.002
			}
			if i%2 == 0 {
				vmin, vmax := mn-pad, mx+pad
				if leftMin == nil || vmin < *leftMin {
					vv := vmin
					leftMin = &vv
				}
				if leftMax == nil || vmax > *leftMax {
					vv := vmax
					leftMax = &vv
				}
			} else {
				vmin, vmax := mn-pad, mx+pad
				if rightMin == nil || vmin < *rightMin {
					vv := vmin
					rightMin = &vv
				}
				if rightMax == nil || vmax > *rightMax {
					vv := vmax
					rightMax = &vv
				}
			}
		}
		values = append(values, cl)
		names = append(names, sd.sym)
	}

	// Build series list
	seriesList := charts.NewSeriesListDataFromValues(values, charts.ChartTypeLine)
	for i := range seriesList {
		seriesList[i].Name = names[i]
		if normalized {
			seriesList[i].AxisIndex = 0
		} else {
			seriesList[i].AxisIndex = i % 2 // 0 left, 1 right
		}
	}

	// Split recommendation
	split := 8
	switch w {
	case "1d":
		split = 8
	case "1w":
		split = 7
	case "1m":
		split = 10
	}

	// Render
	var painter *charts.Painter
	var err error
	if normalized {
		// unified percent axis
		// pad common range slightly
		if commonMin != nil && commonMax != nil {
			pad := (*commonMax - *commonMin) * 0.05
			vmin := *commonMin - pad
			vmax := *commonMax + pad
			painter, err = charts.Render(charts.ChartOption{SeriesList: seriesList},
				charts.TitleTextOptionFunc("Multi • 5m • "+strings.ToUpper(w), strings.Join(names, ", ")+" • normalized %"),
				charts.XAxisOptionFunc(charts.XAxisOption{Data: xLabels, BoundaryGap: charts.FalseFlag(), SplitNumber: split}),
				charts.YAxisOptionFunc(charts.YAxisOption{Min: &vmin, Max: &vmax, DivideCount: 5}),
				charts.LegendOptionFunc(charts.LegendOption{Data: names}),
				charts.ThemeOptionFunc(charts.ThemeLight),
			)
		} else {
			painter, err = charts.Render(charts.ChartOption{SeriesList: seriesList},
				charts.TitleTextOptionFunc("Multi • 5m • "+strings.ToUpper(w), strings.Join(names, ", ")+" • normalized %"),
				charts.XAxisOptionFunc(charts.XAxisOption{Data: xLabels, BoundaryGap: charts.FalseFlag(), SplitNumber: split}),
				charts.YAxisOptionFunc(charts.YAxisOption{DivideCount: 5}),
				charts.LegendOptionFunc(charts.LegendOption{Data: names}),
				charts.ThemeOptionFunc(charts.ThemeLight),
			)
		}
	} else {
		painter, err = charts.Render(charts.ChartOption{SeriesList: seriesList},
			charts.TitleTextOptionFunc("Multi • 5m • "+strings.ToUpper(w), strings.Join(names, ", ")),
			charts.XAxisOptionFunc(charts.XAxisOption{Data: xLabels, BoundaryGap: charts.FalseFlag(), SplitNumber: split}),
			charts.YAxisOptionFunc(
				charts.YAxisOption{Min: leftMin, Max: leftMax, DivideCount: 5},
				charts.YAxisOption{Min: rightMin, Max: rightMax, DivideCount: 5, Position: charts.PositionRight},
			),
			charts.LegendOptionFunc(charts.LegendOption{Data: names}),
			charts.ThemeOptionFunc(charts.ThemeLight),
		)
	}
	if err != nil {
		return nil, err
	}
	img, err := painter.Bytes()
	if err != nil {
		return nil, err
	}
	return img, nil
}
