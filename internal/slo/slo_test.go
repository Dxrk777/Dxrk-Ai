package slo

import (
	"context"
	"math"
	"sync"
	"testing"
	"time"
)

func TestRegisterObjective(t *testing.T) {
	tracker := NewTracker()

	obj := Objective{
		Name:   "test-latency",
		Type:   ObjectiveLatency,
		Target: 0.99,
		Window: 5 * time.Minute,
	}

	err := tracker.RegisterObjective(obj)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	err = tracker.RegisterObjective(obj)
	if err == nil {
		t.Fatal("expected error for duplicate name")
	}
}

func TestUpdateObjective(t *testing.T) {
	tracker := NewTracker()

	obj := Objective{
		Name:    "test-avail",
		Type:    ObjectiveAvailability,
		Target:  0.999,
		Window:  time.Minute,
		Current: 0.99,
	}

	tracker.RegisterObjective(obj)

	err := tracker.UpdateObjective("test-avail", 0.995)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	updated, _ := tracker.GetObjective("test-avail")
	if updated.Current != 0.995 {
		t.Fatalf("expected 0.995, got %f", updated.Current)
	}
	const eps = 1e-9
	if math.Abs(updated.ErrorBudget-0.005) > eps {
		t.Fatalf("expected error budget 0.005, got %f", updated.ErrorBudget)
	}

	err = tracker.UpdateObjective("nonexistent", 0.99)
	if err == nil {
		t.Fatal("expected error for nonexistent objective")
	}
}

func TestGetObjective(t *testing.T) {
	tracker := NewTracker()

	obj := Objective{
		Name:   "test-acc",
		Type:   ObjectiveAccuracy,
		Target: 0.95,
		Window: time.Hour,
	}
	tracker.RegisterObjective(obj)

	got, err := tracker.GetObjective("test-acc")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Name != "test-acc" {
		t.Fatalf("expected name test-acc, got %s", got.Name)
	}

	_, err = tracker.GetObjective("nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent objective")
	}
}

func TestListObjectives(t *testing.T) {
	tracker := NewTracker()

	tracker.RegisterObjective(Objective{Name: "obj1", Type: ObjectiveLatency, Target: 0.99})
	tracker.RegisterObjective(Objective{Name: "obj2", Type: ObjectiveAvailability, Target: 0.999})
	tracker.RegisterObjective(Objective{Name: "obj3", Type: ObjectiveAccuracy, Target: 0.95})

	list := tracker.ListObjectives()
	if len(list) != 3 {
		t.Fatalf("expected 3 objectives, got %d", len(list))
	}
}

func TestDeleteObjective(t *testing.T) {
	tracker := NewTracker()

	tracker.RegisterObjective(Objective{Name: "delete-me", Target: 0.99})

	err := tracker.DeleteObjective("delete-me")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	_, err = tracker.GetObjective("delete-me")
	if err == nil {
		t.Fatal("expected error after deletion")
	}

	err = tracker.DeleteObjective("already-gone")
	if err == nil {
		t.Fatal("expected error deleting nonexistent")
	}
}

func TestSnapshot(t *testing.T) {
	tracker := NewTracker()

	tracker.RegisterObjective(Objective{
		Name:    "snap-test",
		Target:  0.99,
		Current: 0.985,
	})

	ctx := context.Background()
	snap, err := tracker.Snapshot(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if snap.ObjectiveName != "snap-test" {
		t.Fatalf("expected snap-test, got %s", snap.ObjectiveName)
	}
	if snap.Value != 0.985 {
		t.Fatalf("expected 0.985, got %f", snap.Value)
	}
	if snap.WithinSLO {
		t.Fatal("expected WithinSLO to be false")
	}
}

func TestSnapshotNoObjectives(t *testing.T) {
	tracker := NewTracker()
	_, err := tracker.Snapshot(context.Background())
	if err == nil {
		t.Fatal("expected error with no objectives")
	}
}

func TestHistory(t *testing.T) {
	tracker := NewTracker()

	tracker.RegisterObjective(Objective{Name: "hist-test", Target: 0.99, Current: 0.98})

	for i := 0; i < 5; i++ {
		tracker.UpdateObjective("hist-test", 0.98+float64(i)*0.005)
		tracker.Snapshot(context.Background())
	}

	history := tracker.History("hist-test", 3)
	if len(history) != 3 {
		t.Fatalf("expected 3 history entries, got %d", len(history))
	}

	for _, snap := range history {
		if snap.ObjectiveName != "hist-test" {
			t.Fatalf("expected hist-test, got %s", snap.ObjectiveName)
		}
	}

	history = tracker.History("hist-test", 100)
	if len(history) != 5 {
		t.Fatalf("expected 5 history entries with large limit, got %d", len(history))
	}

	history = tracker.History("nonexistent", 10)
	if len(history) != 0 {
		t.Fatalf("expected 0 history entries, got %d", len(history))
	}
}

func TestIsWithinSLO(t *testing.T) {
	tracker := NewTracker()

	tracker.RegisterObjective(Objective{
		Name:    "good",
		Target:  0.99,
		Current: 0.995,
	})
	tracker.RegisterObjective(Objective{
		Name:    "bad",
		Target:  0.99,
		Current: 0.98,
	})

	within, err := tracker.IsWithinSLO("good")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !within {
		t.Fatal("expected good to be within SLO")
	}

	within, err = tracker.IsWithinSLO("bad")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if within {
		t.Fatal("expected bad to be outside SLO")
	}

	_, err = tracker.IsWithinSLO("nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent")
	}
}

func TestCalculateErrorBudget(t *testing.T) {
	const eps = 1e-9

	budget := CalculateErrorBudget(0.99, 0.985)
	if math.Abs(budget-0.015) > eps {
		t.Fatalf("expected 0.015, got %f", budget)
	}

	budget = CalculateErrorBudget(0.99, 0.99)
	if math.Abs(budget-0.01) > eps {
		t.Fatalf("expected 0.01, got %f", budget)
	}

	budget = CalculateErrorBudget(0.99, 1.0)
	if math.Abs(budget-0.0) > eps {
		t.Fatalf("expected 0.0, got %f", budget)
	}
}

func TestCalculateBurnRate(t *testing.T) {
	const eps = 1e-9

	rate := CalculateBurnRate(0.99, 0.98, time.Minute)
	expected := 0.01 / 60.0
	if math.Abs(rate-expected) > eps {
		t.Fatalf("expected %g, got %g", expected, rate)
	}

	rate = CalculateBurnRate(0.99, 0.99, time.Minute)
	if math.Abs(rate-0) > eps {
		t.Fatalf("expected 0, got %g", rate)
	}

	rate = CalculateBurnRate(0.99, 0.98, 0)
	if math.Abs(rate-0) > eps {
		t.Fatalf("expected 0 for zero window, got %g", rate)
	}
}

func TestTimeToBudgetExhaustion(t *testing.T) {
	d := TimeToBudgetExhaustion(0.01, 0.001)
	expected := 10 * time.Second
	if d != expected {
		t.Fatalf("expected %v, got %v", expected, d)
	}

	d = TimeToBudgetExhaustion(0.01, 0)
	if d != 0 {
		t.Fatalf("expected 0 for zero burn rate, got %v", d)
	}

	d = TimeToBudgetExhaustion(-0.01, 0.001)
	if d != 0 {
		t.Fatalf("expected 0 for negative budget, got %v", d)
	}
}

func TestWithinSLO(t *testing.T) {
	if !WithinSLO(0.995, 0.99) {
		t.Fatal("expected true for current >= target")
	}
	if !WithinSLO(0.99, 0.99) {
		t.Fatal("expected true for current == target")
	}
	if WithinSLO(0.98, 0.99) {
		t.Fatal("expected false for current < target")
	}
}

func TestMultiWindowEvaluator(t *testing.T) {
	eval := MultiWindowEvaluator{}
	cfg := DefaultWindowConfig()

	passed, msg := eval.Evaluate([]float64{0.995, 0.993, 0.991}, cfg)
	if !passed {
		t.Fatalf("expected pass, got: %s", msg)
	}

	passed, msg = eval.Evaluate([]float64{0.85, 0.80, 0.90}, cfg)
	if passed {
		t.Fatalf("expected fail for low values, got: %s", msg)
	}

	cfg.ShortTarget = 0.80
	cfg.LongTarget = 0.80
	passed, msg = eval.Evaluate([]float64{0.85, 0.83, 0.82}, cfg)
	if !passed {
		t.Fatalf("expected pass with relaxed targets, got: %s", msg)
	}
}

func TestMultiWindowEvaluatorEmpty(t *testing.T) {
	eval := MultiWindowEvaluator{}
	passed, msg := eval.Evaluate(nil, DefaultWindowConfig())
	if passed {
		t.Fatal("expected fail for empty values")
	}
	if msg != "no values provided" {
		t.Fatalf("expected 'no values provided', got %s", msg)
	}
}

func TestTrackerConcurrentAccess(t *testing.T) {
	tracker := NewTracker()

	tracker.RegisterObjective(Objective{
		Name:    "concurrent",
		Target:  0.99,
		Current: 0.95,
		Window:  time.Minute,
	})

	var wg sync.WaitGroup

	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			tracker.UpdateObjective("concurrent", 0.95)
		}()
	}

	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			tracker.GetObjective("concurrent")
		}()
	}

	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			tracker.Snapshot(context.Background())
		}()
	}

	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			tracker.IsWithinSLO("concurrent")
		}()
	}

	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			tracker.ListObjectives()
		}()
	}

	wg.Wait()

	if _, err := tracker.GetObjective("concurrent"); err != nil {
		t.Fatalf("objective should still exist: %v", err)
	}
}

func TestZeroBurnRate(t *testing.T) {
	tracker := NewTracker()

	tracker.RegisterObjective(Objective{
		Name:    "no-change",
		Target:  0.99,
		Current: 0.99,
		Window:  time.Minute,
	})

	tracker.UpdateObjective("no-change", 0.99)

	obj, _ := tracker.GetObjective("no-change")
	if obj.BurnRate != 0 {
		t.Fatalf("expected 0 burn rate, got %f", obj.BurnRate)
	}

	d := TimeToBudgetExhaustion(0.01, 0)
	if d != 0 {
		t.Fatalf("expected 0 duration, got %v", d)
	}
}

func TestMissingObjective(t *testing.T) {
	tracker := NewTracker()

	_, err := tracker.GetObjective("nonexistent")
	if err == nil {
		t.Fatal("expected error")
	}

	_, err = tracker.IsWithinSLO("nonexistent")
	if err == nil {
		t.Fatal("expected error")
	}

	err = tracker.UpdateObjective("nonexistent", 0.99)
	if err == nil {
		t.Fatal("expected error")
	}

	err = tracker.DeleteObjective("nonexistent")
	if err == nil {
		t.Fatal("expected error")
	}
}
