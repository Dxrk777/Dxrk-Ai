// Package session provides session persistence, serialization, restore, and
// management utilities for Dxrk-Ai.
//
// It defines a canonical session model and a pluggable storage backend for
// saving, loading, and querying sessions. Sessions can be serialized to JSON,
// Markdown, or HTML and imported back. A migration framework tracks format
// evolution and allows upgrading older session files to the current schema.
//
// Typical usage:
//
//	storage := session.NewFileStorage("")
//	sess := session.NewSession(session.SessionOpts{Title: "My Task", Model: "claude-sonnet-4-20250514"})
//	sess.AddMessage(session.Message{Role: "user", Content: "Hello"})
//	_ = storage.Save(sess)
//
//	loaded, _ := storage.Load(sess.ID)
//	md, _ := session.Serialize(loaded, session.Markdown)
package session
