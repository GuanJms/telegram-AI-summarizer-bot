package finance

import (
	"fmt"
	"math"
	"time"
)

// calculateWeightedPortfolio creates a weighted portfolio with optional cash and calculates PnL
func calculateWeightedPortfolio(timestamps []time.Time, assetPrices [][]float64, config *PortfolioConfig) (*PortfolioData, error) {
	if config == nil {
		return nil, fmt.Errorf("portfolio config is nil")
	}

	if len(timestamps) == 0 {
		return nil, fmt.Errorf("no timestamps provided")
	}

	numAssets := len(assetPrices)
	numDays := len(timestamps)

	// Validate configuration
	if len(config.Assets) != numAssets {
		return nil, fmt.Errorf("config assets (%d) don't match price data (%d)", len(config.Assets), numAssets)
	}

	// Validate all assets have same number of data points
	for i, prices := range assetPrices {
		if len(prices) != numDays {
			return nil, fmt.Errorf("asset %d has %d data points, expected %d", i, len(prices), numDays)
		}
	}

	if numDays < 2 {
		return nil, fmt.Errorf("need at least 2 data points for portfolio calculation")
	}

	// Calculate portfolio values and returns
	portfolioValues := make([]float64, numDays)
	portfolioReturns := make([]float64, numDays-1) // Returns start from day 1

	// Initial portfolio value
	initialValue := config.InitialValue
	portfolioValues[0] = initialValue

	// Calculate shares for each asset based on target weights and initial prices
	// Note: Negative weights represent short positions (negative shares)
	shares := make([]float64, numAssets)

	// Calculate cash position accounting for short selling proceeds
	// Short selling generates cash proceeds, while long positions consume cash
	netWeight := 0.0
	for _, asset := range config.Assets {
		netWeight += asset.Weight
	}
	// Cash weight is calculated as: 1.0 - net_long_exposure + short_proceeds
	// This accounts for the fact that shorts generate cash proceeds
	remainingCash := 1.0 - netWeight
	cashValue := initialValue * remainingCash

	for i := 0; i < numAssets; i++ {
		weight := config.Assets[i].Weight
		if assetPrices[i][0] <= 0 {
			return nil, fmt.Errorf("invalid initial price for asset %d (%s): %f", i, config.Assets[i].Symbol, assetPrices[i][0])
		}
		if math.IsNaN(assetPrices[i][0]) || math.IsInf(assetPrices[i][0], 0) {
			return nil, fmt.Errorf("invalid initial price for asset %d (%s): %f (NaN or Inf)", i, config.Assets[i].Symbol, assetPrices[i][0])
		}

		// shares[i] = (initialValue * weight) / initialPrice[i]
		// For short positions (negative weight), this gives negative shares
		// Negative shares * positive price = negative position value (debt)
		shares[i] = (initialValue * weight) / assetPrices[i][0]
		if math.IsNaN(shares[i]) || math.IsInf(shares[i], 0) {
			return nil, fmt.Errorf("invalid share calculation for asset %d (%s): %f", i, config.Assets[i].Symbol, shares[i])
		}
	}

	// Calculate portfolio value for each day
	for day := 1; day < numDays; day++ {
		portfolioValue := cashValue // Start with cash position

		// Sum up value of all asset positions
		for assetIdx := 0; assetIdx < numAssets; assetIdx++ {
			price := assetPrices[assetIdx][day]
			if price < 0 {
				return nil, fmt.Errorf("invalid price for asset %d (%s) on day %d: %f", assetIdx, config.Assets[assetIdx].Symbol, day, price)
			}
			if math.IsNaN(price) || math.IsInf(price, 0) {
				return nil, fmt.Errorf("invalid price for asset %d (%s) on day %d: %f (NaN or Inf)", assetIdx, config.Assets[assetIdx].Symbol, day, price)
			}
			portfolioValue += shares[assetIdx] * price
		}

		if math.IsNaN(portfolioValue) || math.IsInf(portfolioValue, 0) {
			return nil, fmt.Errorf("invalid portfolio value on day %d: %f", day, portfolioValue)
		}

		portfolioValues[day] = portfolioValue

		// Calculate daily return
		if portfolioValues[day-1] > 0 {
			dailyReturn := (portfolioValues[day] - portfolioValues[day-1]) / portfolioValues[day-1]
			if math.IsNaN(dailyReturn) || math.IsInf(dailyReturn, 0) {
				return nil, fmt.Errorf("invalid daily return on day %d: %f", day, dailyReturn)
			}
			portfolioReturns[day-1] = dailyReturn
		} else {
			portfolioReturns[day-1] = 0.0
		}
	}

	return &PortfolioData{
		Timestamps: timestamps,
		Values:     portfolioValues,
		Returns:    portfolioReturns,
	}, nil
}

// calculateEqualWeightedPortfolio creates an equal weighted portfolio and calculates PnL
// This is now a wrapper around calculateWeightedPortfolio for backward compatibility
func calculateEqualWeightedPortfolio(timestamps []time.Time, assetPrices [][]float64, initialValue float64) (*PortfolioData, error) {
	if len(assetPrices) == 0 {
		return nil, fmt.Errorf("no asset data provided")
	}

	// Create equal weight configuration
	var symbols []string
	for i := range assetPrices {
		symbols = append(symbols, fmt.Sprintf("Asset%d", i+1)) // Generic symbols for backward compatibility
	}

	config := createEqualWeightConfig(symbols, initialValue)
	return calculateWeightedPortfolio(timestamps, assetPrices, config)
}

// calculatePortfolioStats computes portfolio statistics including Sharpe ratio
func calculatePortfolioStats(portfolio *PortfolioData) (*PortfolioStats, error) {
	if portfolio == nil || len(portfolio.Values) < 2 {
		return nil, fmt.Errorf("insufficient portfolio data")
	}

	numDays := len(portfolio.Values)
	initialValue := portfolio.Values[0]
	finalValue := portfolio.Values[numDays-1]

	// Validate we have sufficient return data
	if len(portfolio.Returns) == 0 {
		return nil, fmt.Errorf("no return data available")
	}
	if len(portfolio.Returns) < 2 {
		return nil, fmt.Errorf("need at least 2 return observations for statistics")
	}

	// Total return
	totalReturn := (finalValue - initialValue) / initialValue

	// Calculate mean daily return
	meanDailyReturn := 0.0
	for _, ret := range portfolio.Returns {
		meanDailyReturn += ret
	}
	meanDailyReturn /= float64(len(portfolio.Returns))

	// Calculate sample standard deviation of daily returns (using N-1 degrees of freedom)
	variance := 0.0
	n := float64(len(portfolio.Returns))
	for _, ret := range portfolio.Returns {
		diff := ret - meanDailyReturn
		variance += diff * diff
	}
	// Use sample variance (N-1) for unbiased estimator
	variance /= (n - 1)
	dailyVolatility := math.Sqrt(variance)

	// Annualization constants
	tradingDaysPerYear := 252.0

	// Annualized return: Use geometric mean for compounding
	daysInPeriod := float64(len(portfolio.Returns))
	yearsInPeriod := daysInPeriod / tradingDaysPerYear

	var annualReturn float64
	if yearsInPeriod > 0 && finalValue > 0 && initialValue > 0 {
		// Geometric annualization: (1 + total_return)^(1/years) - 1
		annualReturn = math.Pow(finalValue/initialValue, 1.0/yearsInPeriod) - 1.0
	}

	// Alternative: Arithmetic annualization of daily returns
	// annualReturn = meanDailyReturn * tradingDaysPerYear

	// Annualized volatility (standard scaling)
	annualVolatility := dailyVolatility * math.Sqrt(tradingDaysPerYear)

	// Sharpe ratio calculation
	// Standard formula: (Annualized Return - Risk Free Rate) / Annualized Volatility
	// We assume risk-free rate = 0 for simplicity
	//
	// Note: This implementation uses:
	// 1. Sample standard deviation (N-1 degrees of freedom) for unbiased estimation
	// 2. Geometric annualization for returns to account for compounding
	// 3. Standard square-root-of-time scaling for volatility annualization
	// 4. 252 trading days per year convention
	var sharpeRatio float64
	if annualVolatility > 0 {
		// Using annualized figures for consistency
		sharpeRatio = annualReturn / annualVolatility

		// Alternative method using daily figures (should yield same result when annualized):
		// if dailyVolatility > 0 {
		//     sharpeRatio = (meanDailyReturn / dailyVolatility) * math.Sqrt(tradingDaysPerYear)
		// }
	}

	// Maximum drawdown
	maxDrawdown := calculateMaxDrawdown(portfolio.Values)

	// Final validation of calculated statistics
	stats := &PortfolioStats{
		InitialValue: initialValue,
		FinalValue:   finalValue,
		TotalReturn:  totalReturn * 100,      // Convert to percentage
		AnnualReturn: annualReturn * 100,     // Convert to percentage
		Volatility:   annualVolatility * 100, // Convert to percentage
		SharpeRatio:  sharpeRatio,
		MaxDrawdown:  maxDrawdown * 100, // Convert to percentage
		NumDays:      numDays,
	}

	// Validate final statistics for any anomalies
	if math.IsNaN(stats.TotalReturn) || math.IsInf(stats.TotalReturn, 0) {
		return nil, fmt.Errorf("invalid total return: %f", stats.TotalReturn)
	}
	if math.IsNaN(stats.AnnualReturn) || math.IsInf(stats.AnnualReturn, 0) {
		return nil, fmt.Errorf("invalid annual return: %f", stats.AnnualReturn)
	}
	if math.IsNaN(stats.Volatility) || math.IsInf(stats.Volatility, 0) {
		return nil, fmt.Errorf("invalid volatility: %f", stats.Volatility)
	}
	if math.IsNaN(stats.SharpeRatio) || math.IsInf(stats.SharpeRatio, 0) {
		return nil, fmt.Errorf("invalid Sharpe ratio: %f", stats.SharpeRatio)
	}
	if math.IsNaN(stats.MaxDrawdown) || math.IsInf(stats.MaxDrawdown, 0) {
		return nil, fmt.Errorf("invalid max drawdown: %f", stats.MaxDrawdown)
	}

	return stats, nil
}

// calculateMaxDrawdown calculates the maximum drawdown as a percentage
// Maximum drawdown is the largest peak-to-trough decline in portfolio value
func calculateMaxDrawdown(values []float64) float64 {
	if len(values) < 2 {
		return 0.0
	}

	maxDrawdown := 0.0
	peak := values[0]

	// Handle edge case where first value is 0 or negative
	if peak <= 0 {
		// Find first positive value as starting peak
		for i := 1; i < len(values); i++ {
			if values[i] > 0 {
				peak = values[i]
				break
			}
		}
		if peak <= 0 {
			return 0.0 // No positive values found
		}
	}

	for _, value := range values {
		// Update peak if current value is higher
		if value > peak {
			peak = value
		}

		// Calculate drawdown from peak to current value
		if peak > 0 && value >= 0 {
			drawdown := (peak - value) / peak
			if drawdown > maxDrawdown {
				maxDrawdown = drawdown
			}
		}
	}

	return maxDrawdown
}
