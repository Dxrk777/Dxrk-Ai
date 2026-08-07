// SPDX-License-Identifier: MIT
package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"sync/atomic"

	"github.com/Dxrk777/Dxrk/internal/strconst"
	"github.com/Dxrk777/Dxrk/internal/version"
)

// Client is an MCP protocol client.
type Client struct {
	transport Transport
	seq       atomic.Int64
	info      *InitializeResult
}

// NewClient creates an MCP client over the given transport.
func NewClient(transport Transport) *Client {
	return &Client{transport: transport}
}

// Initialize sends the initialize handshake and returns server capabilities.
func (c *Client) Initialize(ctx context.Context) (*InitializeResult, error) {
	params := map[string]any{
		"protocolVersion": "2024-11-05",
		"clientInfo": map[string]string{
			"name":              "dxrk",
			strconst.StrVersion: version.Version,
		},
	}

	var result InitializeResult
	if err := c.call(ctx, "initialize", params, &result); err != nil {
		return nil, err
	}

	c.info = &result
	return &result, nil
}

// ListTools returns all tools exposed by the server.
func (c *Client) ListTools(ctx context.Context) ([]ToolDefinition, error) {
	var result ListToolsResult
	if err := c.call(ctx, "tools/list", nil, &result); err != nil {
		return nil, err
	}
	return result.Tools, nil
}

// CallTool invokes a tool and returns the result.
func (c *Client) CallTool(ctx context.Context, name string, args map[string]any) (*CallToolResult, error) {
	params := CallToolParams{Name: name, Arguments: args}
	var result CallToolResult
	if err := c.call(ctx, "tools/call", params, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// ListResources returns all resources exposed by the server.
func (c *Client) ListResources(ctx context.Context) ([]ResourceDefinition, error) {
	var result ListResourcesResult
	if err := c.call(ctx, "resources/list", nil, &result); err != nil {
		return nil, err
	}
	return result.Resources, nil
}

// ReadResource reads a resource by URI.
func (c *Client) ReadResource(ctx context.Context, uri string) (*ReadResourceResult, error) {
	params := map[string]string{"uri": uri}
	var result ReadResourceResult
	if err := c.call(ctx, "resources/read", params, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// Close closes the underlying transport.
func (c *Client) Close() error {
	return c.transport.Close()
}

// ServerInfo returns the cached server info from initialization.
func (c *Client) ServerInfo() *InitializeResult {
	return c.info
}

// call sends a JSON-RPC request and unmarshals the result.
func (c *Client) call(ctx context.Context, method string, params any, result any) error {
	id := c.seq.Add(1)
	idBytes, _ := json.Marshal(id)

	var paramsBytes json.RawMessage
	if params != nil {
		p, err := json.Marshal(params)
		if err != nil {
			return fmt.Errorf("mcp marshal params: %w", err)
		}
		paramsBytes = p
	}

	req := jsonRPCRequest{
		JSONRPC: "2.0",
		ID:      idBytes,
		Method:  method,
		Params:  paramsBytes,
	}

	reqBytes, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("mcp marshal request: %w", err)
	}

	respBytes, err := c.transport.Send(ctx, reqBytes)
	if err != nil {
		return fmt.Errorf("mcp send: %w", err)
	}

	var resp jsonRPCResponse
	if err := json.Unmarshal(respBytes, &resp); err != nil {
		return fmt.Errorf("mcp unmarshal response: %w", err)
	}

	if resp.Error != nil {
		return &RPCError{
			Code:    resp.Error.Code,
			Message: resp.Error.Message,
		}
	}

	if err := json.Unmarshal(resp.Result, result); err != nil {
		return fmt.Errorf("mcp unmarshal result: %w", err)
	}

	return nil
}

// RPCError represents a JSON-RPC error response.
type RPCError struct {
	Code    int
	Message string
}

func (e *RPCError) Error() string {
	return fmt.Sprintf("MCP error %d: %s", e.Code, e.Message)
}
