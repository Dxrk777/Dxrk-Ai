// Package rpc provides gRPC server and client utilities.
package rpc

import (
	"google.golang.org/grpc"
)

// Server creates a new gRPC server with default options.
func Server() *grpc.Server {
	return grpc.NewServer()
}
