package finance

import "sort"

// filterNonNegative removes points where close < 0, keeping timestamp and value arrays aligned.
func filterNonNegative(ts []int64, cl []float64) ([]int64, []float64) {
	if len(ts) != len(cl) {
		n := len(ts)
		if len(cl) < n {
			n = len(cl)
		}
		ts = ts[:n]
		cl = cl[:n]
	}
	outTs := make([]int64, 0, len(ts))
	outCl := make([]float64, 0, len(cl))
	for i := 0; i < len(ts); i++ {
		if cl[i] < 0 {
			continue
		}
		outTs = append(outTs, ts[i])
		outCl = append(outCl, cl[i])
	}
	return outTs, outCl
}

// filterIQR removes outliers using the Interquartile Range (IQR) rule.
// Any point with value outside [Q1 - k*IQR, Q3 + k*IQR] is dropped.
// For short series (< minPoints), it returns original data.
func filterIQR(ts []int64, cl []float64, k float64, minPoints int) ([]int64, []float64) {
	if len(ts) != len(cl) {
		n := len(ts)
		if len(cl) < n {
			n = len(cl)
		}
		ts = ts[:n]
		cl = cl[:n]
	}
	if len(cl) < minPoints {
		return ts, cl
	}
	vals := make([]float64, len(cl))
	copy(vals, cl)
	sort.Float64s(vals)
	percentile := func(p float64) float64 {
		if len(vals) == 0 {
			return 0
		}
		if p <= 0 {
			return vals[0]
		}
		if p >= 1 {
			return vals[len(vals)-1]
		}
		pos := p * float64(len(vals)-1)
		lo := int(pos)
		hi := lo + 1
		if hi >= len(vals) {
			return vals[lo]
		}
		frac := pos - float64(lo)
		return vals[lo]*(1-frac) + vals[hi]*frac
	}
	q1 := percentile(0.25)
	q3 := percentile(0.75)
	iqr := q3 - q1
	if iqr <= 0 {
		return ts, cl
	}
	lower := q1 - k*iqr
	upper := q3 + k*iqr
	outTs := make([]int64, 0, len(ts))
	outCl := make([]float64, 0, len(cl))
	for i := 0; i < len(ts); i++ {
		v := cl[i]
		if v < lower || v > upper {
			continue
		}
		outTs = append(outTs, ts[i])
		outCl = append(outCl, v)
	}
	if len(outCl) < minPoints/2 {
		return ts, cl
	}
	return outTs, outCl
}
