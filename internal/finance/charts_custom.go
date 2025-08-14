package finance

import (
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/vicanso/go-charts/v2"
)

// normalizeIntervalWindow clamps and maps to Yahoo-supported ranges given interval constraints.
func normalizeIntervalWindow(intervalIn, windowIn string) (interval string, rangeParam string) {
	allowed := map[string]string{"1m": "1m", "5m": "5m", "15m": "15m", "1h": "1h", "1d": "1d"}
	interval = strings.ToLower(strings.TrimSpace(intervalIn))
	if _, ok := allowed[interval]; !ok {
		interval = "5m"
	}
	w := strings.ToLower(strings.TrimSpace(windowIn))
	if w == "" {
		switch interval {
		case "1m":
			w = "30d"
		case "5m":
			w = "1m"
		case "15m":
			w = "3m"
		case "1h":
			w = "1y"
		default:
			w = "1y"
		}
	}
	rank := func(win string) int {
		switch win {
		case "1d":
			return 1
		case "5d":
			return 2
		case "30d", "1m":
			return 3
		case "90d", "3m":
			return 4
		case "180d", "6m":
			return 5
		case "1y":
			return 6
		case "2y":
			return 7
		case "5y":
			return 8
		case "10y":
			return 9
		case "30y":
			return 10
		default:
			return 3
		}
	}
	maxRank := map[string]int{"1m": 3, "5m": 4, "15m": 5, "1h": 7, "1d": 10}[interval]
	r := rank(w)
	if r > maxRank {
		r = maxRank
	}
	switch r {
	case 1:
		rangeParam = "1d"
	case 2:
		rangeParam = "5d"
	case 3:
		rangeParam = "1mo"
	case 4:
		rangeParam = "3mo"
	case 5:
		rangeParam = "6mo"
	case 6:
		rangeParam = "1y"
	case 7:
		rangeParam = "2y"
	case 8:
		rangeParam = "5y"
	case 9:
		rangeParam = "10y"
	default:
		rangeParam = "30y"
	}
	return interval, rangeParam
}

// MakeChart builds a single-symbol chart with custom interval and window.
func MakeChart(symbol string, interval string, window string) ([]byte, error) {
	itv, rng := normalizeIntervalWindow(interval, window)
	ts, cl, err := fetchSeries(symbol, itv, rng)
	if err != nil {
		return nil, err
	}
	if len(ts) == 0 || len(cl) == 0 {
		return nil, errors.New("no data")
	}
	et := getEasternTime()
	x := make([]string, len(ts))
	var yMin, yMax float64
	for i := range ts {
		tt := time.Unix(ts[i], 0).UTC().In(et)
		switch itv {
		case "1d":
			x[i] = tt.Format("2006-01-02")
		case "1h":
			x[i] = tt.Format("Jan 02 15:00")
		default:
			x[i] = tt.Format("Jan 02 15:04")
		}
		v := cl[i]
		if i == 0 {
			yMin, yMax = v, v
		} else {
			if v < yMin {
				yMin = v
			}
			if v > yMax {
				yMax = v
			}
		}
	}
	if len(cl) < 2 {
		return nil, errors.New("not enough data points")
	}
	pad := (yMax - yMin) * 0.05
	if pad < yMax*0.002 {
		pad = yMax * 0.002
	}
	yMin -= pad
	if yMin < 0 {
		yMin = 0
	}
	yMax += pad
	split := 12
	switch rng {
	case "5d":
		split = 8
	case "1mo", "3mo", "6mo":
		split = 10
	}
	painter, err := charts.LineRender([][]float64{cl},
		charts.TitleTextOptionFunc(strings.ToUpper(symbol)+" • "+strings.ToUpper(itv)+" • "+strings.ToUpper(rng)),
		charts.XAxisOptionFunc(charts.XAxisOption{Data: x, BoundaryGap: charts.FalseFlag(), SplitNumber: split}),
		charts.YAxisOptionFunc(charts.YAxisOption{Min: &yMin, Max: &yMax, DivideCount: 5}),
		charts.ThemeOptionFunc(charts.ThemeLight),
	)
	if err != nil {
		return nil, err
	}
	return painter.Bytes()
}

// MakeMultiChart builds a multi-symbol chart that normalizes when >2 symbols.
func MakeMultiChart(symbols []string, interval string, window string) ([]byte, error) {
	if len(symbols) == 0 {
		return nil, errors.New("no symbols provided")
	}
	itv, rng := normalizeIntervalWindow(interval, window)
	type sd struct {
		sym string
		ts  []int64
		cl  []float64
	}
	arr := make([]sd, 0, len(symbols))
	for _, s := range symbols {
		su := strings.TrimSpace(s)
		if su == "" {
			continue
		}
		ts, cl, err := fetchSeries(su, itv, rng)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", su, err)
		}
		arr = append(arr, sd{sym: strings.ToUpper(su), ts: ts, cl: cl})
		time.Sleep(120 * time.Millisecond)
	}
	if len(arr) == 0 {
		return nil, errors.New("no series fetched")
	}
	ref := arr[0]
	for _, x := range arr[1:] {
		if len(x.ts) > len(ref.ts) {
			ref = x
		}
	}
	minLen := len(ref.ts)
	for _, x := range arr {
		if len(x.cl) < minLen {
			minLen = len(x.cl)
		}
	}
	if minLen < 2 {
		return nil, errors.New("not enough data points")
	}
	sort.Slice(ref.ts, func(i, j int) bool { return ref.ts[i] < ref.ts[j] })
	xLabels := make([]string, minLen)
	et := getEasternTime()
	for i, ts := range ref.ts[len(ref.ts)-minLen:] {
		tt := time.Unix(ts, 0).UTC().In(et)
		switch itv {
		case "1d":
			xLabels[i] = tt.Format("2006-01-02")
		case "1h":
			xLabels[i] = tt.Format("Jan 02 15:00")
		default:
			xLabels[i] = tt.Format("Jan 02 15:04")
		}
	}
	normalized := len(arr) > 2
	values := make([][]float64, 0, len(arr))
	names := make([]string, 0, len(arr))
	var leftMin, leftMax, rightMin, rightMax *float64
	var commonMin, commonMax *float64
	for i, x := range arr {
		clOrig := x.cl[len(x.cl)-minLen:]
		cl := clOrig
		if normalized {
			base := 0.0
			for _, v := range clOrig {
				if v != 0 {
					base = v
					break
				}
			}
			if base == 0 {
				base = clOrig[0]
				if base == 0 {
					base = 1
				}
			}
			cl = make([]float64, len(clOrig))
			for j, v := range clOrig {
				cl[j] = (v/base - 1.0) * 100.0
			}
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
			vmin, vmax := mn-pad, mx+pad
			if i%2 == 0 {
				if leftMin == nil || vmin < *leftMin {
					vv := vmin
					leftMin = &vv
				}
				if leftMax == nil || vmax > *leftMax {
					vv := vmax
					leftMax = &vv
				}
			} else {
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
		names = append(names, x.sym)
	}
	split := 12
	switch rng {
	case "5d":
		split = 8
	case "1mo", "3mo", "6mo":
		split = 10
	}
	seriesList := charts.NewSeriesListDataFromValues(values, charts.ChartTypeLine)
	for i := range seriesList {
		seriesList[i].Name = names[i]
		if normalized {
			seriesList[i].AxisIndex = 0
		} else {
			seriesList[i].AxisIndex = i % 2
		}
	}
	var painter *charts.Painter
	var err error
	if normalized {
		var yMin, yMax *float64
		if commonMin != nil && commonMax != nil {
			pad := (*commonMax - *commonMin) * 0.05
			vmin := *commonMin - pad
			vmax := *commonMax + pad
			yMin = &vmin
			yMax = &vmax
		}
		painter, err = charts.Render(charts.ChartOption{SeriesList: seriesList}, charts.TitleTextOptionFunc("Multi • "+strings.ToUpper(itv)+" • "+strings.ToUpper(rng), strings.Join(names, ", ")+" • normalized %"), charts.XAxisOptionFunc(charts.XAxisOption{Data: xLabels, BoundaryGap: charts.FalseFlag(), SplitNumber: split}), charts.YAxisOptionFunc(charts.YAxisOption{Min: yMin, Max: yMax, DivideCount: 5}), charts.LegendOptionFunc(charts.LegendOption{Data: names}), charts.ThemeOptionFunc(charts.ThemeLight))
	} else {
		painter, err = charts.Render(charts.ChartOption{SeriesList: seriesList}, charts.TitleTextOptionFunc("Multi • "+strings.ToUpper(itv)+" • "+strings.ToUpper(rng), strings.Join(names, ", ")), charts.XAxisOptionFunc(charts.XAxisOption{Data: xLabels, BoundaryGap: charts.FalseFlag(), SplitNumber: split}), charts.YAxisOptionFunc(charts.YAxisOption{Min: leftMin, Max: leftMax, DivideCount: 5}, charts.YAxisOption{Min: rightMin, Max: rightMax, DivideCount: 5, Position: charts.PositionRight}), charts.LegendOptionFunc(charts.LegendOption{Data: names}), charts.ThemeOptionFunc(charts.ThemeLight))
	}
	if err != nil {
		return nil, err
	}
	return painter.Bytes()
}

// MakeIndexedChart renders multiple symbols indexed to base 100 at the first point.
func MakeIndexedChart(symbols []string, interval string, window string, base100 bool) ([]byte, error) {
	if len(symbols) == 0 {
		return nil, errors.New("no symbols provided")
	}
	itv, rng := normalizeIntervalWindow(interval, window)
	type sd struct {
		sym string
		ts  []int64
		cl  []float64
	}
	arr := make([]sd, 0, len(symbols))
	for _, s := range symbols {
		su := strings.TrimSpace(s)
		if su == "" {
			continue
		}
		ts, cl, err := fetchSeries(su, itv, rng)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", su, err)
		}
		arr = append(arr, sd{sym: strings.ToUpper(su), ts: ts, cl: cl})
		time.Sleep(120 * time.Millisecond)
	}
	if len(arr) == 0 {
		return nil, errors.New("no series fetched")
	}
	// choose reference timeline longest ts
	ref := arr[0]
	for _, x := range arr[1:] {
		if len(x.ts) > len(ref.ts) {
			ref = x
		}
	}
	minLen := len(ref.ts)
	for _, x := range arr {
		if len(x.cl) < minLen {
			minLen = len(x.cl)
		}
	}
	if minLen < 2 {
		return nil, errors.New("not enough data points")
	}
	// labels
	et := getEasternTime()
	xLabels := make([]string, minLen)
	for i, ts := range ref.ts[len(ref.ts)-minLen:] {
		tt := time.Unix(ts, 0).UTC().In(et)
		switch itv {
		case "1d":
			xLabels[i] = tt.Format("2006-01-02")
		case "1h":
			xLabels[i] = tt.Format("Jan 02 15:00")
		default:
			xLabels[i] = tt.Format("Jan 02 15:04")
		}
	}
	// index values
	values := make([][]float64, 0, len(arr))
	names := make([]string, 0, len(arr))
	var gmin, gmax *float64
	for _, x := range arr {
		cl := x.cl[len(x.cl)-minLen:]
		base := 0.0
		for _, v := range cl {
			if v != 0 {
				base = v
				break
			}
		}
		if base == 0 {
			base = cl[0]
			if base == 0 {
				base = 1
			}
		}
		out := make([]float64, len(cl))
		mul := 1.0
		if base100 {
			mul = 100.0
		}
		for i, v := range cl {
			out[i] = (v / base) * mul
		}
		for _, v := range out {
			if gmin == nil || v < *gmin {
				vv := v
				gmin = &vv
			}
			if gmax == nil || v > *gmax {
				vv := v
				gmax = &vv
			}
		}
		values = append(values, out)
		names = append(names, x.sym)
	}
	var yMin, yMax *float64
	if gmin != nil && gmax != nil {
		pad := (*gmax - *gmin) * 0.05
		vmin := *gmin - pad
		vmax := *gmax + pad
		yMin = &vmin
		yMax = &vmax
	}
	split := 12
	switch rng {
	case "5d":
		split = 8
	case "1mo", "3mo", "6mo":
		split = 10
	}
	seriesList := charts.NewSeriesListDataFromValues(values, charts.ChartTypeLine)
	for i := range seriesList {
		seriesList[i].Name = names[i]
		seriesList[i].AxisIndex = 0
	}
	title := "Indexed • " + strings.ToUpper(itv) + " • " + strings.ToUpper(rng)
	subtitle := strings.Join(names, ", ") + " • base "
	if base100 {
		subtitle += "100"
	} else {
		subtitle += "1.0"
	}
	painter, err := charts.Render(charts.ChartOption{SeriesList: seriesList}, charts.TitleTextOptionFunc(title, subtitle), charts.XAxisOptionFunc(charts.XAxisOption{Data: xLabels, BoundaryGap: charts.FalseFlag(), SplitNumber: split}), charts.YAxisOptionFunc(charts.YAxisOption{Min: yMin, Max: yMax, DivideCount: 5}), charts.LegendOptionFunc(charts.LegendOption{Data: names}), charts.ThemeOptionFunc(charts.ThemeLight))
	if err != nil {
		return nil, err
	}
	return painter.Bytes()
}
