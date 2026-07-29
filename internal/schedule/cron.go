// Package schedule provides cron-based job scheduling.
package schedule

import (
	"github.com/go-co-op/gocron"
)

// NewScheduler creates a new gocron scheduler.
func NewScheduler() *gocron.Scheduler {
	return gocron.NewScheduler(nil)
}
