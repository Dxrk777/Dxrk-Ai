package slo

import (
	"fmt"
	"time"
)

type WindowConfig struct {
	ShortWindow time.Duration
	LongWindow  time.Duration
	ShortTarget float64
	LongTarget  float64
}

func DefaultWindowConfig() WindowConfig {
	return WindowConfig{
		ShortWindow: 5 * time.Minute,
		LongWindow:  30 * time.Minute,
		ShortTarget: 0.99,
		LongTarget:  0.99,
	}
}

type MultiWindowEvaluator struct{}

func (m MultiWindowEvaluator) Evaluate(values []float64, config WindowConfig) (bool, string) {
	if len(values) == 0 {
		return false, "no values provided"
	}

	var sum float64
	for _, v := range values {
		sum += v
	}
	mean := sum / float64(len(values))

	passShort := mean >= config.ShortTarget
	passLong := mean >= config.LongTarget

	if !passShort && !passLong {
		return false, fmt.Sprintf(
			"both windows failed: short=%.4f (target %.4f), long=%.4f (target %.4f)",
			mean, config.ShortTarget, mean, config.LongTarget,
		)
	}
	if !passShort {
		return false, fmt.Sprintf(
			"short window failed: %.4f (target %.4f)",
			mean, config.ShortTarget,
		)
	}
	if !passLong {
		return false, fmt.Sprintf(
			"long window failed: %.4f (target %.4f)",
			mean, config.LongTarget,
		)
	}

	return true, fmt.Sprintf("all windows passed: mean=%.4f", mean)
}
