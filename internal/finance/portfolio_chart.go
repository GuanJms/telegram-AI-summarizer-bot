package finance

import (
	"fmt"
	"strings"

	"github.com/vicanso/go-charts/v2"
)

// MakePortfolioChart generates a chart showing portfolio performance with statistics
func MakePortfolioChart(symbols []string, window string) ([]byte, error) {
	if len(symbols) == 0 {
		return nil, fmt.Errorf("no symbols provided")
	}

	// Create cache key
	cacheKey := fmt.Sprintf("portfolio-%s-%s", strings.Join(symbols, ","), window)
	if img, found := cacheGet(cacheKey); found {
		return img, nil
	}

	// Fetch asset data
	assets, err := fetchPortfolioAssets(symbols, window)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch assets: %w", err)
	}

	// Align timestamps across all assets
	timestamps, alignedPrices, err := alignTimestamps(assets)
	if err != nil {
		return nil, fmt.Errorf("failed to align timestamps: %w", err)
	}

	// Calculate equal weighted portfolio
	portfolio, err := calculateEqualWeightedPortfolio(timestamps, alignedPrices, 100.0)
	if err != nil {
		return nil, fmt.Errorf("failed to calculate portfolio: %w", err)
	}

	// Calculate statistics
	stats, err := calculatePortfolioStats(portfolio)
	if err != nil {
		return nil, fmt.Errorf("failed to calculate stats: %w", err)
	}

	// Convert timestamps to Eastern Time for display
	easternLoc := getEasternTime()
	var xLabels []string
	var values []float64

	for i, ts := range portfolio.Timestamps {
		easternTime := ts.In(easternLoc)

		// Format labels based on data range
		var label string
		if len(portfolio.Timestamps) <= 10 {
			label = easternTime.Format("Jan 02")
		} else if len(portfolio.Timestamps) <= 60 {
			label = easternTime.Format("Jan 02")
		} else {
			label = easternTime.Format("Jan '06")
		}

		xLabels = append(xLabels, label)
		values = append(values, portfolio.Values[i])
	}

	// Calculate Y-axis range with padding
	minVal, maxVal := portfolio.Values[0], portfolio.Values[0]
	for _, val := range portfolio.Values {
		if val < minVal {
			minVal = val
		}
		if val > maxVal {
			maxVal = val
		}
	}

	padding := (maxVal - minVal) * 0.05
	if padding == 0 {
		padding = maxVal * 0.05
	}
	yMin := minVal - padding
	yMax := maxVal + padding

	// Create title with statistics
	title := fmt.Sprintf("Equal Weighted Portfolio (%s)", strings.Join(symbols, ", "))
	subtitle := fmt.Sprintf("Return: %.2f%% | Sharpe: %.2f | Vol: %.2f%% | MaxDD: %.2f%%",
		stats.TotalReturn, stats.SharpeRatio, stats.Volatility, stats.MaxDrawdown)

	// Determine split number for x-axis based on data points
	splitNum := 6
	if len(xLabels) <= 30 {
		splitNum = len(xLabels) / 3
		if splitNum < 3 {
			splitNum = 3
		}
	}

	// Combine title and subtitle
	fullTitle := title + "\n" + subtitle

	p, err := charts.LineRender(
		[][]float64{values},
		charts.TitleTextOptionFunc(fullTitle),
		charts.XAxisOptionFunc(charts.XAxisOption{
			Data:        xLabels,
			SplitNumber: splitNum,
			BoundaryGap: charts.FalseFlag(),
		}),
		charts.YAxisOptionFunc(charts.YAxisOption{
			Min:         &yMin,
			Max:         &yMax,
			DivideCount: 5,
		}),
		charts.ThemeOptionFunc(charts.ThemeLight),
	)

	if err != nil {
		return nil, fmt.Errorf("failed to render chart: %w", err)
	}

	buf, err := p.Bytes()
	if err != nil {
		return nil, fmt.Errorf("failed to generate chart bytes: %w", err)
	}

	// Cache the result
	cacheSet(cacheKey, buf)

	return buf, nil
}

// MakeWeightedPortfolioChart generates a chart showing weighted portfolio performance with statistics
func MakeWeightedPortfolioChart(symbols []string, weights []float64, window string) ([]byte, error) {
	if len(symbols) == 0 {
		return nil, fmt.Errorf("no symbols provided")
	}

	if len(symbols) != len(weights) {
		return nil, fmt.Errorf("symbols and weights length mismatch")
	}

	// Create cache key
	weightStrs := make([]string, len(weights))
	for i, w := range weights {
		weightStrs[i] = fmt.Sprintf("%.3f", w)
	}
	cacheKey := fmt.Sprintf("wport-%s-%s-%s", strings.Join(symbols, ","), strings.Join(weightStrs, ","), window)
	if img, found := cacheGet(cacheKey); found {
		return img, nil
	}

	// Create portfolio config
	config, err := createPortfolioConfig(symbols, weights, 100.0)
	if err != nil {
		return nil, fmt.Errorf("failed to create portfolio config: %w", err)
	}

	// Fetch asset data
	assets, err := fetchPortfolioAssets(symbols, window)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch assets: %w", err)
	}

	// Align timestamps across all assets
	timestamps, alignedPrices, err := alignTimestamps(assets)
	if err != nil {
		return nil, fmt.Errorf("failed to align timestamps: %w", err)
	}

	// Calculate weighted portfolio
	portfolio, err := calculateWeightedPortfolio(timestamps, alignedPrices, config)
	if err != nil {
		return nil, fmt.Errorf("failed to calculate portfolio: %w", err)
	}

	// Calculate statistics
	stats, err := calculatePortfolioStats(portfolio)
	if err != nil {
		return nil, fmt.Errorf("failed to calculate stats: %w", err)
	}

	// Convert timestamps to Eastern Time for display
	easternLoc := getEasternTime()
	var xLabels []string
	var values []float64

	for i, ts := range portfolio.Timestamps {
		easternTime := ts.In(easternLoc)

		// Format labels based on data range
		var label string
		if len(portfolio.Timestamps) <= 10 {
			label = easternTime.Format("Jan 02")
		} else if len(portfolio.Timestamps) <= 60 {
			label = easternTime.Format("Jan 02")
		} else {
			label = easternTime.Format("Jan '06")
		}

		xLabels = append(xLabels, label)
		values = append(values, portfolio.Values[i])
	}

	// Calculate Y-axis range with padding
	minVal, maxVal := portfolio.Values[0], portfolio.Values[0]
	for _, val := range portfolio.Values {
		if val < minVal {
			minVal = val
		}
		if val > maxVal {
			maxVal = val
		}
	}

	padding := (maxVal - minVal) * 0.05
	if padding == 0 {
		padding = maxVal * 0.05
	}
	yMin := minVal - padding
	yMax := maxVal + padding

	// Create title with portfolio composition and statistics
	var composition []string
	for i, symbol := range symbols {
		weight := weights[i]
		if weight >= 0 {
			composition = append(composition, fmt.Sprintf("%s %.1f%%", symbol, weight*100))
		} else {
			composition = append(composition, fmt.Sprintf("%s %.1f%% SHORT", symbol, -weight*100))
		}
	}
	if config.CashWeight > 0 {
		composition = append(composition, fmt.Sprintf("Cash %.1f%%", config.CashWeight*100))
	} else if config.CashWeight < 0 {
		composition = append(composition, fmt.Sprintf("Margin %.1f%%", -config.CashWeight*100))
	}

	title := fmt.Sprintf("Weighted Portfolio (%s)", strings.Join(composition, ", "))
	subtitle := fmt.Sprintf("Return: %.2f%% | Sharpe: %.2f | Vol: %.2f%% | MaxDD: %.2f%%",
		stats.TotalReturn, stats.SharpeRatio, stats.Volatility, stats.MaxDrawdown)

	// Determine split number for x-axis based on data points
	splitNum := 6
	if len(xLabels) <= 30 {
		splitNum = len(xLabels) / 3
		if splitNum < 3 {
			splitNum = 3
		}
	}

	// Combine title and subtitle
	fullTitle := title + "\n" + subtitle

	p, err := charts.LineRender(
		[][]float64{values},
		charts.TitleTextOptionFunc(fullTitle),
		charts.XAxisOptionFunc(charts.XAxisOption{
			Data:        xLabels,
			SplitNumber: splitNum,
			BoundaryGap: charts.FalseFlag(),
		}),
		charts.YAxisOptionFunc(charts.YAxisOption{
			Min:         &yMin,
			Max:         &yMax,
			DivideCount: 5,
		}),
		charts.ThemeOptionFunc(charts.ThemeLight),
	)

	if err != nil {
		return nil, fmt.Errorf("failed to render chart: %w", err)
	}

	buf, err := p.Bytes()
	if err != nil {
		return nil, fmt.Errorf("failed to generate chart bytes: %w", err)
	}

	// Cache the result
	cacheSet(cacheKey, buf)

	return buf, nil
}
