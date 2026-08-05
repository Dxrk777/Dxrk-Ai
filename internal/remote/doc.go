// SPDX-License-Identifier: MIT
// Package remote provides remote session management for Dxrk-Ai.
//
// The package enables connecting to and controlling remote Dxrk-Ai instances
// over a network. It supports:
//
//   - RemoteConfig: connection settings with TLS, auth, and timeout configuration
//   - RemoteSession: session lifecycle (connect, disconnect, reconnect, state tracking)
//   - RemoteControl: execute commands on remote instances (shell, file ops, clipboard)
//   - RemoteMessage: request/response protocol with streaming and event support
//   - RemoteManager: manage multiple concurrent remote connections
//
// All communication uses JSON-encoded messages over TCP connections.
// TLS is optional but recommended for production deployments.
package remote
