// SPDX-License-Identifier: MIT

// Package synthtools provides synthetic output generation, execution
// pausing, sandboxed code evaluation, and data transformation utilities.
//
// It exposes four LLM-invokable tools:
//
//   - synthetic_output — generate structured output for testing and validation
//   - sleep            — pause execution with interruptible sleep
//   - repl             — evaluate code in a sandboxed environment
//   - data_transform   — apply common data transformations
//
// All tools are registered via RegisterAll and follow the standard
// tools.Build(tools.ToolDef{...}) pattern.
package synthtools
