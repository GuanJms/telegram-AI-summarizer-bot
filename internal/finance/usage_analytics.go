package finance

import (
	"fmt"
	"sort"
	"time"

	"telegramBotTrade/internal/storage"

	"github.com/vicanso/go-charts/v2"
)

// UsageAnalytics handles usage metrics visualization
type UsageAnalytics struct{}

func NewUsageAnalytics() *UsageAnalytics {
	return &UsageAnalytics{}
}

// MakeUsageChart creates a usage statistics chart
func (ua *UsageAnalytics) MakeUsageChart(stats map[string]*storage.UsageStats, days int) ([]byte, error) {
	if len(stats) == 0 {
		return nil, fmt.Errorf("no usage data available")
	}

	// Prepare data for pie chart showing category distribution
	var categories []string
	var values []float64

	// Sort categories for consistent ordering
	var sortedCategories []string
	for category := range stats {
		sortedCategories = append(sortedCategories, category)
	}
	sort.Strings(sortedCategories)

	totalUsage := 0
	for _, category := range sortedCategories {
		stat := stats[category]
		categories = append(categories, category)
		values = append(values, float64(stat.Count))
		totalUsage += stat.Count
	}

	// Create pie chart with proper labels
	var pieLabels []string
	for i, category := range categories {
		percentage := (values[i] / float64(totalUsage)) * 100
		pieLabels = append(pieLabels, fmt.Sprintf("%s (%.1f%%)", category, percentage))
	}

	p, err := charts.PieRender(
		values,
		charts.TitleTextOptionFunc(fmt.Sprintf("Command Usage Distribution (%d days)", days)),
		charts.LegendOptionFunc(charts.LegendOption{
			Data: pieLabels,
			Top:  charts.PositionTop,
		}),

		charts.ThemeOptionFunc(charts.ThemeLight),
		charts.WidthOptionFunc(800),
		charts.HeightOptionFunc(600),
	)
	if err != nil {
		return nil, err
	}

	buf, err := p.Bytes()
	if err != nil {
		return nil, err
	}
	return buf, nil
}

// MakeUsageTimeSeriesChart creates a time series chart for usage analytics
func (ua *UsageAnalytics) MakeUsageTimeSeriesChart(series map[string][]storage.TimeSeriesPoint, days int) ([]byte, error) {
	if len(series) == 0 {
		return nil, fmt.Errorf("no time series data available")
	}

	// Prepare data for line chart
	var allTimestamps []int64
	timestampSet := make(map[int64]bool)

	// Collect all unique timestamps
	for _, points := range series {
		for _, point := range points {
			if !timestampSet[point.Timestamp] {
				allTimestamps = append(allTimestamps, point.Timestamp)
				timestampSet[point.Timestamp] = true
			}
		}
	}
	sort.Slice(allTimestamps, func(i, j int) bool {
		return allTimestamps[i] < allTimestamps[j]
	})

	// Convert timestamps to readable format
	var xAxisData []string
	for _, ts := range allTimestamps {
		t := time.Unix(ts, 0)
		if days <= 1 {
			xAxisData = append(xAxisData, t.Format("15:04")) // Hour:minute for single day
		} else if days <= 7 {
			xAxisData = append(xAxisData, t.Format("Mon 15:04")) // Day + time for week
		} else {
			xAxisData = append(xAxisData, t.Format("01/02")) // Month/day for longer periods
		}
	}

	// Prepare series data
	var allSeries [][]float64
	var seriesNames []string

	// Sort categories for consistent ordering
	var sortedCategories []string
	for category := range series {
		sortedCategories = append(sortedCategories, category)
	}
	sort.Strings(sortedCategories)

	for _, category := range sortedCategories {
		points := series[category]

		// Create a map for quick lookup
		pointMap := make(map[int64]int)
		for _, point := range points {
			pointMap[point.Timestamp] = point.Count
		}

		// Build data series with all timestamps
		var data []float64
		for _, ts := range allTimestamps {
			if count, exists := pointMap[ts]; exists {
				data = append(data, float64(count))
			} else {
				data = append(data, 0)
			}
		}

		allSeries = append(allSeries, data)
		seriesNames = append(seriesNames, category)
	}

	// Create line chart
	p, err := charts.LineRender(
		allSeries,
		charts.XAxisOptionFunc(charts.XAxisOption{
			Data: xAxisData,
		}),
		charts.TitleTextOptionFunc(fmt.Sprintf("Command Usage Over Time (%d days)", days)),
		charts.LegendOptionFunc(charts.LegendOption{
			Data: seriesNames,
			Top:  charts.PositionTop,
		}),
		charts.YAxisOptionFunc(charts.YAxisOption{}),
		charts.ThemeOptionFunc(charts.ThemeLight),
		charts.WidthOptionFunc(1000),
		charts.HeightOptionFunc(600),
	)
	if err != nil {
		return nil, err
	}

	buf, err := p.Bytes()
	if err != nil {
		return nil, err
	}
	return buf, nil
}

// FormatUsageStatsText creates a formatted text summary of usage statistics
func (ua *UsageAnalytics) FormatUsageStatsText(stats map[string]*storage.UsageStats, days int) string {
	if len(stats) == 0 {
		return "No usage data available for the specified period."
	}

	// Sort categories for consistent ordering
	var sortedCategories []string
	totalCommands := 0
	for category := range stats {
		sortedCategories = append(sortedCategories, category)
		totalCommands += stats[category].Count
	}
	sort.Strings(sortedCategories)

	text := fmt.Sprintf("📊 **Usage Analytics** (%d days)\n\n", days)
	text += fmt.Sprintf("**Total Commands**: %d\n\n", totalCommands)

	for _, category := range sortedCategories {
		stat := stats[category]
		percentage := float64(stat.Count) / float64(totalCommands) * 100

		text += fmt.Sprintf("**%s** (%d commands, %.1f%%)\n",
			formatCategoryName(category), stat.Count, percentage)

		// Sort commands within category
		type cmdCount struct {
			cmd   string
			count int
		}
		var commands []cmdCount
		for cmd, count := range stat.Commands {
			commands = append(commands, cmdCount{cmd, count})
		}
		sort.Slice(commands, func(i, j int) bool {
			return commands[i].count > commands[j].count
		})

		// Show top commands
		for i, cmd := range commands {
			if i >= 5 { // Limit to top 5 commands per category
				break
			}
			text += fmt.Sprintf("  • %s: %d\n", cmd.cmd, cmd.count)
		}
		text += "\n"
	}

	return text
}

// formatCategoryName converts category names to user-friendly format
func formatCategoryName(category string) string {
	switch category {
	case "recommender":
		return "🤖 AI Recommendations"
	case "summarizer":
		return "📝 Chat Summaries"
	case "portfolio":
		return "💼 Portfolio Analysis"
	case "charts":
		return "📈 Stock Charts"
	default:
		return category
	}
}
