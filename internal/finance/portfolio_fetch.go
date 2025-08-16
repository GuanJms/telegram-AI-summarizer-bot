package finance

import (
	"fmt"
	"strings"
	"time"
)

// parsePortfolioWindow parses window string and returns Yahoo range parameter and target days
func parsePortfolioWindow(window string) (string, int, error) {
	if window == "" {
		return "1y", 365, nil // Default to 1 year
	}

	window = strings.ToLower(window)

	// Map user input to Yahoo Finance range parameters and target days for filtering
	switch {
	case strings.HasSuffix(window, "d"):
		days := strings.TrimSuffix(window, "d")
		var dayNum int
		if _, err := fmt.Sscanf(days, "%d", &dayNum); err != nil {
			return "1mo", 30, nil // Default if parsing fails
		}

		// For specific day requests, determine appropriate Yahoo range
		if dayNum <= 5 {
			return "5d", dayNum, nil
		} else if dayNum <= 30 {
			return "1mo", dayNum, nil
		} else if dayNum <= 90 {
			return "3mo", dayNum, nil
		} else {
			return "1y", dayNum, nil
		}

	case strings.HasSuffix(window, "w"):
		weeks := strings.TrimSuffix(window, "w")
		var weekNum int
		if _, err := fmt.Sscanf(weeks, "%d", &weekNum); err != nil {
			return "1mo", 21, nil // Default 3 weeks if parsing fails
		}

		targetDays := weekNum * 7 // Convert weeks to days

		// For week requests, determine appropriate Yahoo range
		if weekNum <= 1 {
			return "5d", targetDays, nil
		} else if weekNum <= 4 {
			return "1mo", targetDays, nil // Use 1mo but filter to requested weeks
		} else if weekNum <= 12 {
			return "3mo", targetDays, nil
		} else if weekNum <= 26 {
			return "6mo", targetDays, nil
		} else {
			return "1y", targetDays, nil
		}

	case strings.HasSuffix(window, "m"):
		months := strings.TrimSuffix(window, "m")
		var monthNum int
		if _, err := fmt.Sscanf(months, "%d", &monthNum); err != nil {
			return "1y", 365, nil // Default if parsing fails
		}

		targetDays := monthNum * 30 // Approximate days

		if monthNum <= 1 {
			return "1mo", targetDays, nil
		} else if monthNum <= 3 {
			return "3mo", targetDays, nil
		} else if monthNum <= 6 {
			return "6mo", targetDays, nil
		} else if monthNum <= 12 {
			return "1y", targetDays, nil
		} else if monthNum <= 24 {
			return "2y", targetDays, nil
		} else {
			return "5y", targetDays, nil
		}

	case strings.HasSuffix(window, "y"):
		years := strings.TrimSuffix(window, "y")
		var yearNum int
		if _, err := fmt.Sscanf(years, "%d", &yearNum); err != nil {
			return "1y", 365, nil // Default if parsing fails
		}

		targetDays := yearNum * 365 // Approximate days

		if yearNum <= 1 {
			return "1y", targetDays, nil
		} else if yearNum <= 2 {
			return "2y", targetDays, nil
		} else if yearNum <= 5 {
			return "5y", targetDays, nil
		} else if yearNum <= 10 {
			return "10y", targetDays, nil
		} else {
			return "max", targetDays, nil
		}

	default:
		return "", 0, fmt.Errorf("invalid window format: %s (use format like 1d, 1w, 1m, 1y)", window)
	}
}

// fetchPortfolioAssets fetches daily price data for multiple assets and filters to target timeframe
func fetchPortfolioAssets(symbols []string, window string) ([]AssetData, error) {
	rangeParam, targetDays, err := parsePortfolioWindow(window)
	if err != nil {
		return nil, err
	}

	var assets []AssetData

	for _, symbol := range symbols {
		// Use daily interval for portfolio analysis
		ts, prices, err := fetchSeries(symbol, "1d", rangeParam)
		if err != nil {
			return nil, fmt.Errorf("failed to fetch %s: %w", symbol, err)
		}

		if len(ts) == 0 || len(prices) == 0 {
			return nil, fmt.Errorf("no data available for %s", symbol)
		}

		// Filter to target timeframe if needed
		filteredTs, filteredPrices := filterToTargetDays(ts, prices, targetDays)

		assets = append(assets, AssetData{
			Symbol:     symbol,
			Timestamps: filteredTs,
			Prices:     filteredPrices,
		})
	}

	return assets, nil
}

// filterToTargetDays filters timestamps and prices to the most recent N days
func filterToTargetDays(timestamps []int64, prices []float64, targetDays int) ([]int64, []float64) {
	if len(timestamps) == 0 || targetDays <= 0 {
		return timestamps, prices
	}

	// If we have fewer data points than target days, return all
	if len(timestamps) <= targetDays {
		return timestamps, prices
	}

	// Calculate the cutoff timestamp (targetDays ago from the most recent timestamp)
	if len(timestamps) == 0 {
		return timestamps, prices
	}

	latestTimestamp := timestamps[len(timestamps)-1]
	cutoffTimestamp := latestTimestamp - int64(targetDays*24*3600) // targetDays ago

	// Find the first timestamp >= cutoff
	startIdx := 0
	for i, ts := range timestamps {
		if ts >= cutoffTimestamp {
			startIdx = i
			break
		}
	}

	// Return the filtered data
	filteredTs := make([]int64, len(timestamps)-startIdx)
	filteredPrices := make([]float64, len(prices)-startIdx)

	copy(filteredTs, timestamps[startIdx:])
	copy(filteredPrices, prices[startIdx:])

	return filteredTs, filteredPrices
}

// alignTimestamps aligns assets using forward-fill for mixed 24/7 and market-hours assets
func alignTimestamps(assets []AssetData) ([]time.Time, [][]float64, error) {
	if len(assets) == 0 {
		return nil, nil, fmt.Errorf("no assets provided")
	}

	// Find the asset with the most conservative (least frequent) data points
	// This will be our base timeline to avoid excessive data points
	var baseAsset AssetData
	minDataPoints := int(^uint(0) >> 1) // Max int

	for _, asset := range assets {
		if len(asset.Timestamps) < minDataPoints {
			minDataPoints = len(asset.Timestamps)
			baseAsset = asset
		}
	}

	if len(baseAsset.Timestamps) == 0 {
		return nil, nil, fmt.Errorf("no timestamps found in base asset")
	}

	// Use base asset's timestamps as our unified timeline
	// This prevents excessive data points when mixing daily stocks with minute-level crypto
	unifiedTimestamps := make([]int64, len(baseAsset.Timestamps))
	copy(unifiedTimestamps, baseAsset.Timestamps)

	// Sort timestamps
	for i := 0; i < len(unifiedTimestamps)-1; i++ {
		for j := i + 1; j < len(unifiedTimestamps); j++ {
			if unifiedTimestamps[i] > unifiedTimestamps[j] {
				unifiedTimestamps[i], unifiedTimestamps[j] = unifiedTimestamps[j], unifiedTimestamps[i]
			}
		}
	}

	// Convert to time.Time slice
	var alignedTimes []time.Time
	for _, ts := range unifiedTimestamps {
		alignedTimes = append(alignedTimes, time.Unix(ts, 0))
	}

	// Forward-fill each asset to match the unified timeline
	var alignedPrices [][]float64

	for assetIdx, asset := range assets {
		// Create timestamp to price map for this asset
		priceMap := make(map[int64]float64)
		for i, ts := range asset.Timestamps {
			if i < len(asset.Prices) && asset.Prices[i] > 0 {
				priceMap[ts] = asset.Prices[i]
			}
		}

		// Forward-fill prices for the unified timestamp series
		var assetPrices []float64
		var lastKnownPrice float64
		hasFirstPrice := false

		for _, ts := range unifiedTimestamps {
			if price, exists := priceMap[ts]; exists {
				// Exact timestamp match - use actual price
				lastKnownPrice = price
				hasFirstPrice = true
				assetPrices = append(assetPrices, price)
			} else if hasFirstPrice {
				// No exact match - forward fill with last known price
				assetPrices = append(assetPrices, lastKnownPrice)
			} else {
				// No price data available yet - find the closest price before or at this timestamp
				closestPrice := findClosestPrice(asset, ts)
				if closestPrice > 0 {
					lastKnownPrice = closestPrice
					hasFirstPrice = true
					assetPrices = append(assetPrices, closestPrice)
				} else {
					return nil, nil, fmt.Errorf("no valid price data found for asset %s (%s) at or before timestamp %d", asset.Symbol, assets[assetIdx].Symbol, ts)
				}
			}
		}

		if len(assetPrices) != len(unifiedTimestamps) {
			return nil, nil, fmt.Errorf("price alignment failed for asset %s: got %d prices, expected %d", asset.Symbol, len(assetPrices), len(unifiedTimestamps))
		}

		alignedPrices = append(alignedPrices, assetPrices)
	}

	return alignedTimes, alignedPrices, nil
}

// findClosestPrice finds the closest price for an asset at or before the given timestamp
// This is used for forward-filling when an exact timestamp match is not found
func findClosestPrice(asset AssetData, targetTimestamp int64) float64 {
	var bestPrice float64
	var bestTimestamp int64 = -1

	// Find the most recent price at or before the target timestamp
	for i, ts := range asset.Timestamps {
		if ts <= targetTimestamp && i < len(asset.Prices) && asset.Prices[i] > 0 {
			if ts > bestTimestamp {
				bestTimestamp = ts
				bestPrice = asset.Prices[i]
			}
		}
	}

	// If no price found before target, look for the first price after target
	if bestTimestamp == -1 {
		for i, ts := range asset.Timestamps {
			if ts > targetTimestamp && i < len(asset.Prices) && asset.Prices[i] > 0 {
				return asset.Prices[i]
			}
		}
	}

	return bestPrice
}

// findNextPrice finds the next available price for an asset at or after the given timestamp
func findNextPrice(asset AssetData, fromTimestamp int64) float64 {
	for i, ts := range asset.Timestamps {
		if ts >= fromTimestamp && i < len(asset.Prices) && asset.Prices[i] > 0 {
			return asset.Prices[i]
		}
	}
	return 0 // No valid price found
}
