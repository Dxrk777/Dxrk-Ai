// SPDX-License-Identifier: MIT
package dxrkmemory

import (
	"context"
	"fmt"
	"net/http"
	"os/exec"
	"strings"
	"time"

	"github.com/Dxrk777/Dxrk-Ai/internal/strconst"
)

var (
	lookPath    = exec.LookPath
	execCommand = exec.Command
)

func VerifyInstalled() error {
	if _, err := lookPath("dxrk-memory"); err != nil {
		return fmt.Errorf("dxrk-memory binary not found in PATH: %w", err)
	}

	return nil
}

// VerifyVersion runs "dxrk-memory version" and returns the trimmed output.
// Returns an error if the command fails or produces no output.
func VerifyVersion() (string, error) {
	cmd := execCommand("dxrk-memory", strconst.StrVersion)
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("dxrk-memory version command failed: %w", err)
	}

	version := strings.TrimSpace(string(out))
	if version == "" {
		return "", fmt.Errorf("dxrk-memory version returned empty output")
	}

	return version, nil
}

func VerifyHealth(ctx context.Context, baseURL string) error {
	if strings.TrimSpace(baseURL) == "" {
		baseURL = "http://127.0.0.1:7437"
	}

	client := &http.Client{Timeout: 2 * time.Second}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimRight(baseURL, "/")+"/health", nil)
	if err != nil {
		return fmt.Errorf("build dxrk-memory health request: %w", err)
	}

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("dxrk-memory health check failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("dxrk-memory health check returned status %d", resp.StatusCode)
	}

	return nil
}
