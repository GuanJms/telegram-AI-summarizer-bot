package finance

import (
	"fmt"
	"strconv"
	"strings"
)

// ParseWeightedPortfolio parses a weighted portfolio command string
// Format: /port SPY 0.5 AAPL 0.25 1y
// Returns: symbols, weights, window, error
func ParseWeightedPortfolio(input string) ([]string, []float64, string, error) {
	// Remove command prefix and clean input
	input = strings.TrimSpace(input)
	if strings.HasPrefix(input, "/port") {
		input = strings.TrimSpace(input[5:])
	}

	parts := strings.Fields(input)
	if len(parts) < 3 {
		return nil, nil, "", fmt.Errorf("insufficient arguments: need at least symbol weight window")
	}

	// Last part should be the window
	window := parts[len(parts)-1]
	parts = parts[:len(parts)-1] // Remove window from parts

	// Remaining parts should be pairs of symbol weight
	if len(parts)%2 != 0 {
		return nil, nil, "", fmt.Errorf("invalid format: each symbol must have a weight")
	}

	var symbols []string
	var weights []float64
	totalWeight := 0.0

	for i := 0; i < len(parts); i += 2 {
		symbol := strings.ToUpper(strings.TrimSpace(parts[i]))
		weightStr := strings.TrimSpace(parts[i+1])

		if symbol == "" {
			return nil, nil, "", fmt.Errorf("empty symbol at position %d", i/2+1)
		}

		weight, err := strconv.ParseFloat(weightStr, 64)
		if err != nil {
			return nil, nil, "", fmt.Errorf("invalid weight '%s' for symbol %s: %w", weightStr, symbol, err)
		}

		// Allow negative weights for short positions
		if weight > 1 {
			return nil, nil, "", fmt.Errorf("long weight %f for symbol %s exceeds 1.0", weight, symbol)
		}

		if weight < -1 {
			return nil, nil, "", fmt.Errorf("short weight %f for symbol %s exceeds -1.0 (max 100%% short)", weight, symbol)
		}

		symbols = append(symbols, symbol)
		weights = append(weights, weight)
		totalWeight += weight
	}

	// For short selling portfolios, we need to validate differently
	// The total net weight (long - short) should not exceed available capital
	// But we'll allow flexibility as long as it's reasonable

	// Calculate total long and short exposure
	totalLong := 0.0
	totalShort := 0.0
	for _, w := range weights {
		if w > 0 {
			totalLong += w
		} else {
			totalShort += -w // Make positive for calculation
		}
	}

	// Total gross exposure should be reasonable (e.g., max 3x leverage)
	totalGrossExposure := totalLong + totalShort
	if totalGrossExposure > 3.0 {
		return nil, nil, "", fmt.Errorf("total gross exposure %.3f exceeds 3.0 (300%% leverage limit)", totalGrossExposure)
	}

	// Check for duplicate symbols
	seen := make(map[string]bool)
	for _, symbol := range symbols {
		if seen[symbol] {
			return nil, nil, "", fmt.Errorf("duplicate symbol: %s", symbol)
		}
		seen[symbol] = true
	}

	return symbols, weights, window, nil
}

// createPortfolioConfig creates a PortfolioConfig from symbols and weights
func createPortfolioConfig(symbols []string, weights []float64, initialValue float64) (*PortfolioConfig, error) {
	if len(symbols) != len(weights) {
		return nil, fmt.Errorf("symbols and weights length mismatch: %d vs %d", len(symbols), len(weights))
	}

	var assets []WeightedAsset
	totalWeight := 0.0

	for i, symbol := range symbols {
		weight := weights[i]
		assets = append(assets, WeightedAsset{
			Symbol: symbol,
			Weight: weight,
		})
		totalWeight += weight
	}

	// For portfolios with short positions, cash calculation is different
	// Net weight = total long positions - total short positions
	// Cash weight = 1.0 - net_weight
	// Note: Short positions generate cash proceeds, so cash can be > 1.0
	cashWeight := 1.0 - totalWeight

	return &PortfolioConfig{
		Assets:       assets,
		CashWeight:   cashWeight,
		InitialValue: initialValue,
	}, nil
}

// createEqualWeightConfig creates an equal weighted portfolio config
func createEqualWeightConfig(symbols []string, initialValue float64) *PortfolioConfig {
	if len(symbols) == 0 {
		return &PortfolioConfig{
			Assets:       []WeightedAsset{},
			CashWeight:   1.0,
			InitialValue: initialValue,
		}
	}

	weight := 1.0 / float64(len(symbols))
	var assets []WeightedAsset

	for _, symbol := range symbols {
		assets = append(assets, WeightedAsset{
			Symbol: symbol,
			Weight: weight,
		})
	}

	return &PortfolioConfig{
		Assets:       assets,
		CashWeight:   0.0, // No cash in equal weighted
		InitialValue: initialValue,
	}
}
