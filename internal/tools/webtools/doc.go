// SPDX-License-Identifier: MIT
// Package webtools provides web fetching, searching, and content extraction tools
// for the Dxrk tool registry. All tools use only stdlib and golang.org/x/net/html.
package webtools

import "github.com/Dxrk777/Dxrk/internal/tools"

// RegisterAll registers all web tools into the given registry.
func RegisterAll(reg *tools.Registry) error {
	for _, fn := range []func(*tools.Registry) error{
		registerWebFetch,
		registerWebSearch,
		registerWebExtract,
	} {
		if err := fn(reg); err != nil {
			return err
		}
	}
	return nil
}

func boolPtr(b bool) *bool { return &b }
