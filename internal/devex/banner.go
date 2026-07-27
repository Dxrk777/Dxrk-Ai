// SPDX-License-Identifier: MIT
package devex

import (
	"context"
	"fmt"
	"runtime"
	"time"
)

const (
	colorReset  = "\033[0m"
	colorBlue   = "\033[34m"
	colorCyan   = "\033[36m"
	colorGreen  = "\033[32m"
	colorYellow = "\033[33m"
	colorBold   = "\033[1m"
)

type Banner struct {
	Version   string
	Project   string
	StartTime time.Time
}

func NewBanner(version string) *Banner {
	return &Banner{
		Version:   version,
		Project:   "Dxrk",
		StartTime: time.Now(),
	}
}

func (b *Banner) Render(_ context.Context) string {
	top := fmt.Sprintf("%s┌─────────────────────────────────────────────┐%s", colorCyan, colorReset)
	title := fmt.Sprintf("%s│%s %s%s%s  %s%s%-39s%s│%s",
		colorCyan, colorReset,
		colorBold, colorYellow, "⚡", colorReset, colorBlue, "Dxrk CLI", colorReset, colorCyan)
	sub := fmt.Sprintf("%s│%s %s%s%s  %s%s%-40s%s│%s",
		colorCyan, colorReset,
		colorBlue, colorBold, "Dxrk", colorReset, colorGreen, "Ecosystem Configurator", colorReset, colorCyan)
	sep := fmt.Sprintf("%s│%s                                             %s│%s", colorCyan, colorReset, colorReset, colorCyan)
	ver := fmt.Sprintf("%s│%s %sVersion%s  %s%-43s%s│%s",
		colorCyan, colorReset,
		colorGreen, colorReset, colorYellow, b.Version, colorReset, colorCyan)
	gov := fmt.Sprintf("%s│%s %sGo%s      %s%-42s%s│%s",
		colorCyan, colorReset,
		colorGreen, colorReset, colorYellow, runtime.Version(), colorReset, colorCyan)
	agt := fmt.Sprintf("%s│%s %sAgents%s  %s%-42s%s│%s",
		colorCyan, colorReset,
		colorGreen, colorReset, colorYellow, "8 installed", colorReset, colorCyan)
	bot := fmt.Sprintf("%s└─────────────────────────────────────────────┘%s", colorCyan, colorReset)

	return fmt.Sprintf("%s%s\n%s\n%s\n%s\n%s\n%s\n%s\n%s\n%s",
		colorBold,
		top, title, sub, sep, ver, gov, agt, bot,
		colorReset)
}

func (b *Banner) ShortBanner() string {
	return fmt.Sprintf("%s⚡%s %sDxrk%s %sDxrk%s %s%s%s",
		colorYellow, colorReset,
		colorBold, colorReset,
		colorBlue, colorReset,
		colorGreen, b.Version, colorReset)
}
