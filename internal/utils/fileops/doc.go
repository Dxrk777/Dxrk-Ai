// Package fileops provides safe, cached, atomic file operations with encoding detection.
//
// It is designed for AI agent tooling where file reads and writes must be
// deterministic, reversible, and resilient against partial failures.
//
// # Features
//
//   - Safe reading with automatic encoding detection (UTF-8, UTF-16, ASCII, Latin-1)
//   - Binary file detection to guard against accidental text processing of non-text content
//   - Atomic writes via temporary-file-then-rename to avoid corruption on crash
//   - LRU file-content cache with TTL and modification-time based auto-invalidation
//   - Search-and-replace editing with validation, indentation preservation, and regex support
//   - Path utilities that prevent directory traversal and resolve symlinks
//
// # Concurrency
//
// The [FileCache] is safe for concurrent use. All other functions are safe for
// concurrent use as long as the caller does not mutate the same file from
// multiple goroutines simultaneously.
//
// # Error handling
//
// Every exported function returns a non-nil error when the operation cannot be
// completed. Errors wrap underlying os errors where meaningful so callers can
// use errors.Is / errors.As.
package fileops
