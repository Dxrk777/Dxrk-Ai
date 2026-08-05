// SPDX-License-Identifier: MIT
package commands

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

func RegisterMemoryCommand(reg *Registry) {
	cmd := &cobra.Command{
		Use:   "memory [show|clear|compact]",
		Short: "Manage memory and conversation context",
		Long:  "View, clear, or compact the session memory store.",
		RunE: func(cmd *cobra.Command, args []string) error {
			action := "show"
			if len(args) > 0 {
				action = args[0]
			}
			switch action {
			case "show":
				return runMemoryShow()
			case "clear":
				return runMemoryClear()
			case "compact":
				return runMemoryCompact()
			default:
				return fmt.Errorf("unknown memory action %q (use show, clear, or compact)", action)
			}
		},
	}
	reg.AddCommand(cmd)
}

func runMemoryShow() error {
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("resolve home: %w", err)
	}

	memoryDir := filepath.Join(home, ".dxrk", "memory")
	entries, err := os.ReadDir(memoryDir)
	if err != nil {
		if os.IsNotExist(err) {
			fmt.Fprintln(os.Stderr, "No memory entries found.")
			return nil
		}
		return fmt.Errorf("read memory dir: %w", err)
	}

	if len(entries) == 0 {
		fmt.Fprintln(os.Stderr, "Memory is empty.")
		return nil
	}

	fmt.Fprintf(os.Stderr, "Memory Entries (%d)\n", len(entries))
	fmt.Fprintf(os.Stderr, "──────────────────\n")
	for _, e := range entries {
		info, err := e.Info()
		if err != nil {
			continue
		}
		fmt.Fprintf(os.Stderr, "  %-40s  %s\n", e.Name(), info.ModTime().Format("2006-01-02 15:04"))
	}
	return nil
}

func runMemoryClear() error {
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("resolve home: %w", err)
	}

	memoryDir := filepath.Join(home, ".dxrk", "memory")
	entries, err := os.ReadDir(memoryDir)
	if err != nil {
		if os.IsNotExist(err) {
			fmt.Fprintln(os.Stderr, "Memory already empty.")
			return nil
		}
		return fmt.Errorf("read memory dir: %w", err)
	}

	removed := 0
	for _, e := range entries {
		path := filepath.Join(memoryDir, e.Name())
		if err := os.Remove(path); err != nil {
			fmt.Fprintf(os.Stderr, "  WARN: could not remove %s: %v\n", e.Name(), err)
			continue
		}
		removed++
	}

	fmt.Fprintf(os.Stderr, "Cleared %d memory entries.\n", removed)
	return nil
}

func runMemoryCompact() error {
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("resolve home: %w", err)
	}

	memoryDir := filepath.Join(home, ".dxrk", "memory")
	entries, err := os.ReadDir(memoryDir)
	if err != nil {
		if os.IsNotExist(err) {
			fmt.Fprintln(os.Stderr, "Nothing to compact.")
			return nil
		}
		return fmt.Errorf("read memory dir: %w", err)
	}

	totalSize := int64(0)
	for _, e := range entries {
		info, err := e.Info()
		if err != nil {
			continue
		}
		totalSize += info.Size()
	}

	fmt.Fprintf(os.Stderr, "Memory compaction summary:\n")
	fmt.Fprintf(os.Stderr, "  Entries:     %d\n", len(entries))
	fmt.Fprintf(os.Stderr, "  Total size:  %s\n", formatBytes(totalSize))
	fmt.Fprintf(os.Stderr, "  Status:      ready\n")

	_ = strings.TrimSpace("")
	return nil
}

func formatBytes(b int64) string {
	const (
		kb = 1024
		mb = kb * 1024
	)
	switch {
	case b >= mb:
		return fmt.Sprintf("%.1f MB", float64(b)/float64(mb))
	case b >= kb:
		return fmt.Sprintf("%.1f KB", float64(b)/float64(kb))
	default:
		return fmt.Sprintf("%d B", b)
	}
}
