package finance

import (
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/vicanso/go-charts/v2"
)

// Make5mChart generates a 5-minute chart for the given symbol and time window (1d,1w,1m)
func Make5mChart(symbol string, window ...string) ([]byte, error) {
	w := "1d"
	if len(window) > 0 && window[0] != "" {
		switch strings.ToLower(strings.TrimSpace(window[0])) {
		case "1d", "day", "1day":
			w = "1d"
		case "1w", "1wk", "week", "1week":
			w = "1w"
		case "1m", "1mo", "month", "1month":
			w = "1m"
		}
	}
	rangeParam := map[string]string{"1d": "1d", "1w": "5d", "1m": "1mo"}[w]

	// cache
	cacheKey := strings.ToUpper(symbol) + "|" + w
	if img, ok := cacheGet(cacheKey); ok {
		return img, nil
	}

	ts, cl, err := fetch5mSeries(symbol, rangeParam)
	if err != nil {
		return nil, err
	}
	if len(ts) == 0 || len(cl) == 0 {
		return nil, errors.New("no data")
	}

	// build labels and y-range
	et := getEasternTime()
	xAll := make([]string, len(ts))
	var yMin, yMax float64
	for i, t := range ts {
		tt := time.Unix(t, 0).UTC().In(et)
		if w == "1d" {
			xAll[i] = tt.Format("15:04")
		} else {
			xAll[i] = tt.Format("Jan 02 15:04")
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
	split := map[string]int{"1d": 8, "1w": 7, "1m": 10}[w]

	painter, err := charts.LineRender([][]float64{cl},
		charts.TitleTextOptionFunc(strings.ToUpper(symbol)+" • 5m • "+strings.ToUpper(w)),
		charts.XAxisOptionFunc(charts.XAxisOption{Data: xAll, BoundaryGap: charts.FalseFlag(), SplitNumber: split}),
		charts.YAxisOptionFunc(charts.YAxisOption{Min: &yMin, Max: &yMax, DivideCount: 5}),
		charts.ThemeOptionFunc(charts.ThemeLight),
	)
	if err != nil {
		return nil, err
	}
	img, err := painter.Bytes()
	if err != nil {
		return nil, err
	}
	cacheSet(cacheKey, img)
	return img, nil
}

// MakeMulti5mChart renders multiple symbols in one chart with legends and two y-axes if needed.
func MakeMulti5mChart(symbols []string, window ...string) ([]byte, error) {
	if len(symbols) == 0 {
		return nil, errors.New("no symbols provided")
	}
	w := "1d"
	if len(window) > 0 && window[0] != "" {
		switch strings.ToLower(strings.TrimSpace(window[0])) {
		case "1d", "day", "1day":
			w = "1d"
		case "1w", "1wk", "week", "1week":
			w = "1w"
		case "1m", "1mo", "month", "1month":
			w = "1m"
		}
	}
	rangeParam := map[string]string{"1d": "1d", "1w": "5d", "1m": "1mo"}[w]

	type sd struct {
		sym string
		ts  []int64
		cl  []float64
	}
	arr := make([]sd, 0, len(symbols))
	for _, s := range symbols {
		s = strings.TrimSpace(s)
		if s == "" {
			continue
		}
		ts, cl, err := fetch5mSeries(s, rangeParam)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", s, err)
		}
		arr = append(arr, sd{sym: strings.ToUpper(s), ts: ts, cl: cl})
		time.Sleep(120 * time.Millisecond)
	}
	if len(arr) == 0 {
		return nil, errors.New("no series fetched")
	}

	// intersect timestamps across all series
	count := map[int64]int{}
	for _, x := range arr {
		for _, t := range x.ts {
			count[t]++
		}
	}
	common := make([]int64, 0, len(count))
	for t, c := range count {
		if c == len(arr) {
			common = append(common, t)
		}
	}
	if len(common) < 2 {
		return nil, errors.New("not enough overlapping time points")
	}
	sort.Slice(common, func(i, j int) bool { return common[i] < common[j] })

	// labels
	et := getEasternTime()
	xLabels := make([]string, len(common))
	for i, t := range common {
		tt := time.Unix(t, 0).UTC().In(et)
		if w == "1d" {
			xLabels[i] = tt.Format("15:04")
		} else {
			xLabels[i] = tt.Format("Jan 02 15:04")
		}
	}

	// build aligned values
	values := make([][]float64, 0, len(arr))
	names := make([]string, 0, len(arr))
	normalized := len(arr) > 2
	var leftMin, leftMax, rightMin, rightMax *float64
	var commonMin, commonMax *float64
	for i, x := range arr {
		mp := make(map[int64]float64, len(x.ts))
		for j, t := range x.ts {
			if j < len(x.cl) {
				mp[t] = x.cl[j]
			}
		}
		aligned := make([]float64, 0, len(common))
		for _, t := range common {
			if v, ok := mp[t]; ok {
				aligned = append(aligned, v)
			}
		}
		cl := aligned
		if normalized {
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
			for j, v := range cl {
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

	split := map[string]int{"1d": 8, "1w": 7, "1m": 10}[w]
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
		painter, err = charts.Render(charts.ChartOption{SeriesList: seriesList},
			charts.TitleTextOptionFunc("Multi • 5m • "+strings.ToUpper(w), strings.Join(names, ", ")+" • normalized %"),
			charts.XAxisOptionFunc(charts.XAxisOption{Data: xLabels, BoundaryGap: charts.FalseFlag(), SplitNumber: split}),
			charts.YAxisOptionFunc(charts.YAxisOption{Min: yMin, Max: yMax, DivideCount: 5}),
			charts.LegendOptionFunc(charts.LegendOption{Data: names}),
			charts.ThemeOptionFunc(charts.ThemeLight),
		)
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
	return painter.Bytes()
}
