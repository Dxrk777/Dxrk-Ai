// SPDX-License-Identifier: MIT
package main

import "testing"

func TestVersionIsSet(t *testing.T) {
	if version == "" {
		t.Error("version variable should not be empty (set via ldflags at build time)")
	}
}
