// Package ws provides WebSocket connectivity using coder/websocket.
package ws

import (
	"net/http"

	"github.com/coder/websocket"
)

// Accept upgrades an HTTP connection to WebSocket.
func Accept(w http.ResponseWriter, r *http.Request, opts *websocket.AcceptOptions) (*websocket.Conn, error) {
	return websocket.Accept(w, r, opts)
}
