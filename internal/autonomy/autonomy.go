// SPDX-License-Identifier: MIT
package autonomy

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"time"
)

type Autonomy struct {
	Config      *AutonomyConfig
	Permissions *PermissionStore
	Updater     *Updater
	Verifier    *Verifier
	Learner     *Learner
	Metrics     *IQMetrics
	Evolution   *EvolutionEngine

	ctx     context.Context
	cancel  context.CancelFunc
	running bool

	iqHistory []IQSnapshot
	startTime time.Time
}

type AutonomyConfig struct {
	Enabled        bool
	IntervalSec    int
	SelfUpdate     bool
	SelfVerify     bool
	SelfLearn      bool
	AutoFix        bool
	Evolution      bool
	LearnDir       string
	MemoriesFile   string
	MaxMemoryItems int
	IQMetricsFile  string
	IQReportEvery  int
	Capabilities   []string
	AskBefore      []string
}

func New(cfg *AutonomyConfig, projectRoot string, requestFn func(capability Capability, reason string) (bool, error)) *Autonomy {
	ctx, cancel := context.WithCancel(context.Background())

	perms := NewPermissionStore(cfg.Capabilities, cfg.AskBefore)
	if requestFn != nil {
		perms.SetRequestHandler(requestFn)
	}

	learnDir := filepath.Join(projectRoot, cfg.LearnDir)
	_ = os.MkdirAll(learnDir, 0o750)

	learner := NewLearner(filepath.Join(projectRoot, cfg.MemoriesFile), cfg.MaxMemoryItems)
	metrics := NewIQMetrics(filepath.Join(projectRoot, cfg.IQMetricsFile))
	updater := NewUpdater(projectRoot, cfg.IntervalSec, perms)
	verifier := NewVerifier(projectRoot, cfg.AutoFix, learner, metrics, perms)

	var evolution *EvolutionEngine
	if cfg.Evolution {
		evolutionPath := filepath.Join(learnDir, "evolution.json")
		evolution = NewEvolutionEngine(evolutionPath, learner, metrics)
	}

	return &Autonomy{
		Config:      cfg,
		Permissions: perms,
		Updater:     updater,
		Verifier:    verifier,
		Learner:     learner,
		Metrics:     metrics,
		Evolution:   evolution,
		ctx:         ctx,
		cancel:      cancel,
		startTime:   time.Now(),
	}
}

func (a *Autonomy) Start() {
	if !a.Config.Enabled || a.running {
		return
	}
	a.running = true

	a.log("autonomy starting (interval=%ds, update=%v, verify=%v, learn=%v, evolution=%v)",
		a.Config.IntervalSec, a.Config.SelfUpdate, a.Config.SelfVerify,
		a.Config.SelfLearn, a.Config.Evolution)

	if a.Config.SelfUpdate {
		result := a.Updater.Check(true)
		if result.Updated {
			a.log("self-update: %s -> %s (%d changes)", result.Before, result.After, result.Changes)
		} else if result.Error != "" {
			a.log("self-update check: %s", result.Error)
		}
	}

	if a.Config.SelfVerify {
		result := a.Verifier.Verify()
		a.log("verify: pass=%v failures=%d duration=%v", result.Pass, result.Failures, result.Duration)
		if !result.Pass {
			if err := a.saveResult("verify-fail", result); err != nil {
				a.log("save verify result: %v", err)
			}
		}
	}

	go a.loop()
}

func (a *Autonomy) loop() {
	ticker := time.NewTicker(time.Duration(a.Config.IntervalSec) * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-a.ctx.Done():
			a.running = false
			return
		case <-ticker.C:
			a.tick()
		}
	}
}

func (a *Autonomy) tick() {
	if a.Config.SelfUpdate {
		result := a.Updater.Check(false)
		if result.Updated {
			a.log("auto-updated: %s -> %s (%d changes)", result.Before, result.After, result.Changes)
		}
	}

	if a.Config.SelfVerify {
		result := a.Verifier.Verify()
		if !result.Pass {
			a.log("verify failed (%d failures)", result.Failures)
			if err := a.saveResult("verify-fail", result); err != nil {
				a.log("save verify result: %v", err)
			}
		}
	}

	if a.Config.Evolution && a.Evolution != nil {
		a.Evolution.Evolve()
		best := a.Evolution.BestGenome()
		a.log("evolution: gen=%d pop=%d best_score=%.1f",
			best.Generations, len(a.Evolution.Population()), best.Score)
	}

	if a.Config.SelfLearn {
		a.reportIQ()
	}
}

func (a *Autonomy) reportIQ() {
	snapshot := a.Metrics.Score()
	a.iqHistory = append(a.iqHistory, snapshot)

	if a.Config.IQReportEvery > 0 && len(a.iqHistory)%a.Config.IQReportEvery == 0 {
		a.log("=== IQ REPORT ===")
		a.log("  success_rate:     %.1f%%", snapshot.SuccessRate)
		a.log("  error_reduction:  %.1f%%", snapshot.ErrorReduction)
		a.log("  token_efficiency: %.1f", snapshot.TokenEfficiency)
		a.log("  latency_p50:      %.0fms", snapshot.LatencyP50)
		a.log("  test_pass_rate:   %.1f%%", snapshot.TestPassRate)
		a.log("  auto_fix_rate:    %.1f%%", snapshot.AutoFixRate)
		a.log("  evolution_score:  %.1f", snapshot.EvolutionScore)
		a.log("  OVERALL IQ:       %.1f", snapshot.OverallIQ)
		a.log("  turns_completed:  %d", snapshot.TurnsCompleted)
		a.log("  errors_fixed:     %d", snapshot.ErrorsFixed)
		a.log("=================")
	}
}

func (a *Autonomy) RecordTurn(success bool, tokens int, latencyMs float64) {
	if a.Metrics != nil {
		a.Metrics.RecordTurn(success, tokens, latencyMs)
	}

	if a.Learner != nil {
		a.Learner.Record(MemoryItem{
			Category:  "turn",
			Success:   success,
			Tokens:    tokens,
			LatencyMs: latencyMs,
		})
	}
}

func (a *Autonomy) Stop() {
	a.cancel()
	a.running = false
	a.log("autonomy stopped")
}

func (a *Autonomy) Running() bool {
	return a.running
}

func (a *Autonomy) CurrentIQ() IQSnapshot {
	return a.Metrics.Score()
}

func (a *Autonomy) saveResult(name string, data any) error {
	dir := filepath.Dir(a.Config.MemoriesFile)
	_ = os.MkdirAll(filepath.Join(dir, "results"), 0750)
	path := filepath.Join(dir, "results", fmt.Sprintf("%s-%d.json", name, time.Now().Unix()))
	f, err := os.Create(path) //nolint:gosec
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()
	enc := json.NewEncoder(f)
	enc.SetIndent("", "  ")
	return enc.Encode(data)
}

func (a *Autonomy) log(format string, args ...any) {
	slog.Info(fmt.Sprintf(format, args...), "component", "autonomy")
}
