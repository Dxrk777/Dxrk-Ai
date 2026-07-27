// SPDX-License-Identifier: MIT
package autonomy

import (
	"testing"
	"time"
)

func TestPermissionStore(t *testing.T) {
	ps := NewPermissionStore([]string{"fs.read", "git"}, []string{"fs.write", "sudo"})

	if err := ps.Check(CapFSRead, "read config"); err != nil {
		t.Fatalf("expected fs.read allowed: %v", err)
	}

	if err := ps.Check(CapGit, "commit"); err != nil {
		t.Fatalf("expected git allowed: %v", err)
	}

	requested := false
	ps.SetRequestHandler(func(capability Capability, reason string) (bool, error) {
		requested = true
		return true, nil
	})

	if err := ps.Check(CapFSWrite, "write file"); err != nil {
		t.Fatalf("expected fs.write to be approved: %v", err)
	}
	if !requested {
		t.Fatal("expected permission request to be made")
	}
}

func TestPermissionStore_Deny(t *testing.T) {
	ps := NewPermissionStore([]string{"fs.read"}, nil)
	ps.Deny(CapFSRead, true)

	if err := ps.Check(CapFSRead, "read"); err == nil {
		t.Fatal("expected error for denied capability")
	}
}

func TestIQMetrics(t *testing.T) {
	metrics := NewIQMetrics(t.TempDir() + "/iq.json")

	for i := 0; i < 15; i++ {
		metrics.RecordTurn(true, 100+i*10, float64(50+i*5))
	}
	metrics.RecordTurn(false, 200, 100)
	metrics.RecordTestResult(true)
	metrics.RecordTestResult(true)
	metrics.RecordTestResult(false)
	metrics.RecordAutoFix(true)

	score := metrics.Score()
	if score.SuccessRate <= 0 {
		t.Fatalf("expected positive success rate, got %.1f", score.SuccessRate)
	}
	if score.TestPassRate <= 0 {
		t.Fatalf("expected positive test pass rate, got %.1f", score.TestPassRate)
	}
	if score.OverallIQ <= 0 {
		t.Fatalf("expected positive IQ, got %.1f", score.OverallIQ)
	}
}

func TestLearner(t *testing.T) {
	learner := NewLearner(t.TempDir()+"/learn.json", 100)

	learner.Record(MemoryItem{
		Category: "test",
		Input:    "create a function that adds two numbers",
		Output:   "func add(a, b int) int { return a + b }",
		Success:  true,
		Tags:     []string{"math", "function"},
	})

	suggestions := learner.Suggest("add two numbers")
	if len(suggestions) == 0 {
		suggestions = learner.Suggest("create a function")
	}
	if len(suggestions) == 0 {
		t.Fatal("expected at least one suggestion from pattern match")
	}

	mems := learner.RecentMemories(1)
	if len(mems) != 1 {
		t.Fatalf("expected 1 memory, got %d", len(mems))
	}
	if !mems[0].Success {
		t.Fatal("expected successful memory")
	}
}

func TestEvolution(t *testing.T) {
	learner := NewLearner(t.TempDir()+"/learn.json", 100)
	metrics := NewIQMetrics(t.TempDir() + "/iq.json")
	evo := NewEvolutionEngine(t.TempDir()+"/evo.json", learner, metrics)

	if len(evo.Population()) == 0 {
		t.Fatal("expected seeded population")
	}

	genome := evo.Evolve()
	if genome == nil {
		t.Fatal("expected evolved genome")
	}

	best := evo.BestGenome()
	if best.Score <= 0 {
		t.Fatalf("expected positive best score, got %.1f", best.Score)
	}
}

func TestAutonomyConfig(t *testing.T) {
	cfg := &AutonomyConfig{
		Enabled:        true,
		IntervalSec:    30,
		SelfUpdate:     false,
		SelfVerify:     false,
		SelfLearn:      true,
		AutoFix:        true,
		Capabilities:   []string{"fs.read"},
		AskBefore:      []string{"fs.write"},
		LearnDir:       t.TempDir(),
		MemoriesFile:   t.TempDir() + "/mem.json",
		IQMetricsFile:  t.TempDir() + "/iq.json",
		MaxMemoryItems: 100,
	}

	a := New(cfg, t.TempDir(), nil)
	if a == nil {
		t.Fatal("expected non-nil autonomy")
	}
	if a.Config.Enabled != true {
		t.Fatal("expected enabled")
	}
	a.Stop()
}

func TestNewLearner_SaveLoad(t *testing.T) {
	path := t.TempDir() + "/learn.json"

	l1 := NewLearner(path, 100)
	l1.Record(MemoryItem{
		Category: "persist",
		Input:    "how to sort a slice in Go",
		Output:   "sort.Slice(s, func(i, j int) bool { return s[i] < s[j] })",
		Success:  true,
	})

	time.Sleep(100 * time.Millisecond)

	l2 := NewLearner(path, 100)
	suggestions := l2.Suggest("sort a slice")
	if len(suggestions) == 0 {
		suggestions = l2.Suggest("how to sort")
	}
	if len(suggestions) == 0 {
		t.Fatal("expected persisted pattern to load")
	}
	if suggestions[0].Trigger == "" {
		t.Fatal("expected non-empty trigger in loaded pattern")
	}
}

func TestEvolution_Score(t *testing.T) {
	learner := NewLearner(t.TempDir()+"/learn.json", 100)
	metrics := NewIQMetrics(t.TempDir() + "/iq.json")
	evo := NewEvolutionEngine(t.TempDir()+"/evo.json", learner, metrics)

	first := evo.BestGenome()
	evo.UpdateScore(first.ID, 90.0)

	genome := evo.Evolve()
	if genome.Score <= first.Score {
		t.Logf("evolution: %.1f -> %.1f (expected improvement or diversity)", first.Score, genome.Score)
	}
}

func TestVerifier_AutoFix(t *testing.T) {
	learner := NewLearner(t.TempDir()+"/learn.json", 100)
	metrics := NewIQMetrics(t.TempDir() + "/iq.json")
	perms := NewPermissionStore([]string{"fs.read", "fs.write", "exec"}, nil)
	verifier := NewVerifier(t.TempDir(), false, learner, metrics, perms)

	result := verifier.Verify()
	if result == nil {
		t.Fatal("expected verify result")
	}
}

func TestPermission_RequestHandler(t *testing.T) {
	ps := NewPermissionStore(nil, []string{"docker"})

	denied := false
	ps.SetRequestHandler(func(capability Capability, reason string) (bool, error) {
		denied = true
		return false, nil
	})

	err := ps.Check(CapDocker, "run container")
	if err == nil {
		t.Fatal("expected error for denied permission")
	}
	if !denied {
		t.Fatal("expected request handler to be called")
	}
}

func TestIQMetrics_History(t *testing.T) {
	metrics := NewIQMetrics(t.TempDir() + "/iq.json")
	for i := 0; i < 25; i++ {
		metrics.RecordTurn(i%5 != 0, 100, float64(50+i%10*5))
	}

	score := metrics.Score()
	if score.OverallIQ < 0 || score.OverallIQ > 100 {
		t.Fatalf("IQ out of range [0,100]: %.1f", score.OverallIQ)
	}
}
