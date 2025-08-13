package finance

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

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

func Make5mChart(symbol string) ([]byte, error) {
	url := fmt.Sprintf("https://query1.finance.yahoo.com/v8/finance/chart/%s?range=2d&interval=5m&includePrePost=true", symbol)
	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("User-Agent", "curl/8")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var yc yahooChartResp
	if err := json.NewDecoder(resp.Body).Decode(&yc); err != nil {
		return nil, err
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

	for i := range t {
		if i >= len(c) || c[i] == 0 {
			continue
		}
		xAxisData = append(xAxisData, time.Unix(t[i], 0).Format("15:04"))
		yAxisData = append(yAxisData, c[i])
	}

	if len(xAxisData) == 0 {
		return nil, errors.New("no valid data points")
	}

	// Create line chart
	painter, err := charts.LineRender([][]float64{yAxisData},
		charts.TitleTextOptionFunc(strings.ToUpper(symbol)+" • 5m"),
		charts.XAxisDataOptionFunc(xAxisData),
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

	return imgBytes, nil
}
