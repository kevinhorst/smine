// Package stats provides the mean, variance and Welch's t-test the skill A/B evals report.
package stats

import "math"

// tCrit95's df 200 entry stands in for the asymptotic 1.96 beyond the table.
var tCrit95 = []struct {
	df   float64
	crit float64
}{
	{1, 12.706}, {2, 4.303}, {3, 3.182}, {4, 2.776}, {5, 2.571},
	{6, 2.447}, {7, 2.365}, {8, 2.306}, {9, 2.262}, {10, 2.228},
	{12, 2.179}, {15, 2.131}, {20, 2.086}, {30, 2.042}, {60, 2.000},
	{120, 1.980}, {200, 1.960},
}

// CriticalT95 interpolates linearly between table rows; df below 1 uses the df=1 value.
func CriticalT95(df float64) float64 {
	if df <= tCrit95[0].df {
		return tCrit95[0].crit
	}
	for i := 1; i < len(tCrit95); i++ {
		if df <= tCrit95[i].df {
			lower := tCrit95[i-1]
			upper := tCrit95[i]
			fraction := (df - lower.df) / (upper.df - lower.df)
			return lower.crit + fraction*(upper.crit-lower.crit)
		}
	}
	return tCrit95[len(tCrit95)-1].crit
}

// Mean returns the arithmetic mean; 0 for an empty slice.
func Mean(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	sum := 0.0
	for _, value := range values {
		sum += value
	}
	return sum / float64(len(values))
}

// SignificantAt05 never reports df < 1 (fewer than 2 samples per group) significant.
func SignificantAt05(t, df float64) bool {
	if df < 1 {
		return false
	}
	abs := t
	if abs < 0 {
		abs = -abs
	}
	return abs > CriticalT95(df)
}

// StdDev returns the sample standard deviation; 0 for fewer than 2 values.
func StdDev(values []float64) float64 {
	return math.Sqrt(Variance(values))
}

// Variance uses the n-1 denominator.
func Variance(values []float64) float64 {
	if len(values) < 2 {
		return 0
	}
	mean := Mean(values)
	sum := 0.0
	for _, value := range values {
		delta := value - mean
		sum += delta * delta
	}
	return sum / float64(len(values)-1)
}

// WelchT yields (0, 0), not comparable, for groups under 2 values or two zero-variance groups.
func WelchT(a, b []float64) (float64, float64) {
	if len(a) < 2 || len(b) < 2 {
		return 0, 0
	}
	varA := Variance(a) / float64(len(a))
	varB := Variance(b) / float64(len(b))
	if varA+varB == 0 {
		return 0, 0
	}
	t := (Mean(a) - Mean(b)) / math.Sqrt(varA+varB)
	df := (varA + varB) * (varA + varB) /
		(varA*varA/float64(len(a)-1) + varB*varB/float64(len(b)-1))
	return t, df
}
