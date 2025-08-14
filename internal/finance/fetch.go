package finance

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

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
					ts, cl = filterNonNegative(ts, cl)
					ts, cl = filterIQR(ts, cl, 1.5, 20)
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
	ts := yc.Chart.Result[0].Timestamp
	cl := yc.Chart.Result[0].Indicators.Quote[0].Close
	ts, cl = filterNonNegative(ts, cl)
	ts, cl = filterIQR(ts, cl, 1.5, 20)
	return ts, cl, nil
}

// fetchSeries fetches timestamps and close prices for a single symbol using the given interval and range.
func fetchSeries(symbol string, interval string, rangeParam string) ([]int64, []float64, error) {
	hosts := []string{"query1.finance.yahoo.com", "query2.finance.yahoo.com"}
	backoffs := []time.Duration{200 * time.Millisecond, 500 * time.Millisecond, 1 * time.Second}
	var yc yahooChartResp
	var lastErr error
	for attempt := 0; attempt < len(backoffs)+1; attempt++ {
		for _, host := range hosts {
			url := fmt.Sprintf("https://%s/v8/finance/chart/%s?range=%s&interval=%s&includePrePost=true&events=div,splits", host, symbol, rangeParam, interval)
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
				url := fmt.Sprintf("https://%s/v7/finance/spark?symbols=%s&range=%s&interval=%s", host, strings.ToUpper(symbol), rangeParam, interval)
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
					ts, cl = filterNonNegative(ts, cl)
					ts, cl = filterIQR(ts, cl, 1.5, 20)
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
	ts := yc.Chart.Result[0].Timestamp
	cl := yc.Chart.Result[0].Indicators.Quote[0].Close
	ts, cl = filterNonNegative(ts, cl)
	ts, cl = filterIQR(ts, cl, 1.5, 20)
	return ts, cl, nil
}
