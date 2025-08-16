package finance

import "time"

// PortfolioData represents the time series data for a portfolio
type PortfolioData struct {
	Timestamps []time.Time
	Values     []float64 // Portfolio values starting from 100
	Returns    []float64 // Daily returns
}

// PortfolioStats represents calculated portfolio statistics
type PortfolioStats struct {
	InitialValue float64
	FinalValue   float64
	TotalReturn  float64 // Total return as percentage
	AnnualReturn float64 // Annualized return
	Volatility   float64 // Annualized volatility
	SharpeRatio  float64 // Risk-free rate assumed to be 0
	MaxDrawdown  float64 // Maximum drawdown as percentage
	NumDays      int     // Number of trading days
}

// AssetData represents price data for a single asset
type AssetData struct {
	Symbol     string
	Timestamps []int64
	Prices     []float64
}

// WeightedAsset represents an asset with its target weight in the portfolio
type WeightedAsset struct {
	Symbol string
	Weight float64 // Target weight (0.0 to 1.0)
}

// PortfolioConfig represents the configuration for a portfolio
type PortfolioConfig struct {
	Assets       []WeightedAsset
	CashWeight   float64 // Remaining weight allocated to cash
	InitialValue float64 // Starting portfolio value (e.g., 100)
}
