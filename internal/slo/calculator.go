package slo

import "time"

func CalculateErrorBudget(target float64, current float64) float64 {
	return 1 - current
}

func CalculateBurnRate(current float64, previous float64, window time.Duration) float64 {
	if window <= 0 {
		return 0
	}
	delta := current - previous
	secs := window.Seconds()
	if secs <= 0 {
		return 0
	}
	return delta / secs
}

func TimeToBudgetExhaustion(errorBudget float64, burnRate float64) time.Duration {
	if burnRate <= 0 {
		return time.Duration(0)
	}
	secs := errorBudget / burnRate
	if secs < 0 {
		return time.Duration(0)
	}
	return time.Duration(secs * float64(time.Second))
}

func WithinSLO(current float64, target float64) bool {
	return current >= target
}
