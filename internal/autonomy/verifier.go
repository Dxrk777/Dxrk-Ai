// SPDX-License-Identifier: MIT
package autonomy

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

type VerifyResult struct {
	Pass       bool
	VetOutput  string
	TestOutput string
	Duration   time.Duration
	Failures   int
}

type Verifier struct {
	projectRoot string
	autoFix     bool
	learner     *Learner
	metrics     *IQMetrics
	perms       *PermissionStore
}

func NewVerifier(root string, autoFix bool, learner *Learner, metrics *IQMetrics, perms *PermissionStore) *Verifier {
	return &Verifier{
		projectRoot: root,
		autoFix:     autoFix,
		learner:     learner,
		metrics:     metrics,
		perms:       perms,
	}
}

func (v *Verifier) Verify() *VerifyResult {
	start := time.Now()
	result := &VerifyResult{}

	vetOut, vetErr := v.runCmd("go", "vet", "./...")
	result.VetOutput = strings.TrimSpace(string(vetOut))
	if vetErr != nil {
		result.Failures++
		result.VetOutput += "\n" + vetErr.Error()
	}

	testOut, testErr := v.runCmd("go", "test", "./...")
	result.TestOutput = strings.TrimSpace(string(testOut))
	if testErr != nil {
		result.Failures++
		result.TestOutput += "\n" + testErr.Error()
	}

	result.Duration = time.Since(start)
	result.Pass = result.Failures == 0

	if v.metrics != nil {
		v.metrics.RecordTestResult(result.Pass)
	}

	if !result.Pass && v.autoFix {
		v.autoFixFailures(result)
	}

	return result
}

func (v *Verifier) autoFixFailures(result *VerifyResult) {
	input := fmt.Sprintf("vet: %s\ntest: %s", result.VetOutput, result.TestOutput)

	suggestions := v.learner.Suggest(input)
	for _, s := range suggestions {
		if s.SuccessRate > 0.7 {
			lines := strings.Split(s.Action, "\n")
			for _, line := range lines {
				line = strings.TrimSpace(line)
				if line == "" || strings.HasPrefix(line, "#") {
					continue
				}
				parts := strings.Fields(line)
				if len(parts) >= 2 {
					_, _ = v.runCmd(parts[0], parts[1:]...)
				}
			}
		}
	}

	wait := 2 * time.Second
	time.Sleep(wait)

	retry := v.runCmdRaw("go", "vet", "./...")
	testRetry := v.runCmdRaw("go", "test", "./...")

	fixed := retry.err == nil && testRetry.err == nil
	if v.metrics != nil {
		v.metrics.RecordAutoFix(fixed)
	}

	v.learner.Record(MemoryItem{
		Category: "auto_fix",
		Input:    input,
		Output:   fmt.Sprintf("fixed=%v\nvet:%s\ntest:%s", fixed, retry.out, testRetry.out),
		Success:  fixed,
	})
}

type cmdResult struct {
	out []byte
	err error
}

func (v *Verifier) runCmd(name string, args ...string) ([]byte, error) {
	r := v.runCmdRaw(name, args...)
	return r.out, r.err
}

func (v *Verifier) runCmdRaw(name string, args ...string) cmdResult {
	var stdout, stderr bytes.Buffer
	cmd := exec.Command(name, args...) //nolint:gosec
	cmd.Dir = v.projectRoot
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	out := stdout.Bytes()
	if stderr.Len() > 0 {
		out = append(out, '\n')
		out = append(out, stderr.Bytes()...)
	}
	return cmdResult{out: out, err: err}
}
