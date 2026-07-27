// SPDX-License-Identifier: MIT
// Package mcp implements a lightweight Model Context Protocol client.
// Supports JSON-RPC 2.0 over stdio and HTTP/SSE transports.
package mcp

import "encoding/json"

// --- JSON-RPC 2.0 types ---

type jsonRPCRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type jsonRPCResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *jsonRPCError   `json:"error,omitempty"`
}

type jsonRPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// --- MCP protocol types ---

// Implementation describes the MCP server implementation.
type Implementation struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

// InitializeResult is the response to an initialize request.
type InitializeResult struct {
	ProtocolVersion string         `json:"protocolVersion"`
	Capabilities    *Capabilities  `json:"capabilities"`
	ServerInfo      Implementation `json:"serverInfo"`
}

// Capabilities describes what the MCP server supports.
type Capabilities struct {
	Tools     *ToolsCapabilities     `json:"tools,omitempty"`
	Resources *ResourcesCapabilities `json:"resources,omitempty"`
}

// ToolsCapabilities describes tool-related capabilities.
type ToolsCapabilities struct {
	ListChanged bool `json:"listChanged"`
}

// ResourcesCapabilities describes resource-related capabilities.
type ResourcesCapabilities struct {
	ListChanged bool `json:"listChanged"`
}

// ToolSchema defines the input schema for a tool.
type ToolSchema struct {
	Type       string                  `json:"type"`
	Properties map[string]ToolProperty `json:"properties,omitempty"`
}

// ToolProperty defines a single property in a tool's input schema.
type ToolProperty struct {
	Type        string `json:"type"`
	Description string `json:"description,omitempty"`
	Required    bool   `json:"-"`
}

// ToolDefinition describes a tool exposed by the MCP server.
type ToolDefinition struct {
	Name        string     `json:"name"`
	Description string     `json:"description"`
	InputSchema ToolSchema `json:"inputSchema"`
}

// CallToolParams are the parameters for a tools/call request.
type CallToolParams struct {
	Name      string         `json:"name"`
	Arguments map[string]any `json:"arguments"`
}

// ToolResultContent is a single content item in a tool result.
type ToolResultContent struct {
	Type string `json:"type"` // "text" or "resource"
	Text string `json:"text,omitempty"`
}

// CallToolResult is the response from a tools/call request.
type CallToolResult struct {
	Content []ToolResultContent `json:"content"`
	IsError bool                `json:"isError,omitempty"`
}

// ListToolsResult is the response from a tools/list request.
type ListToolsResult struct {
	Tools      []ToolDefinition `json:"tools"`
	NextCursor string           `json:"nextCursor,omitempty"`
}

// ResourceDefinition describes a resource exposed by the MCP server.
type ResourceDefinition struct {
	URI         string `json:"uri"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	MimeType    string `json:"mimeType,omitempty"`
}

// ListResourcesResult is the response from a resources/list request.
type ListResourcesResult struct {
	Resources  []ResourceDefinition `json:"resources"`
	NextCursor string               `json:"nextCursor,omitempty"`
}

// ReadResourceResult is the response from a resources/read request.
type ReadResourceResult struct {
	Contents []ResourceContent `json:"contents"`
}

// ResourceContent is a single content item from reading a resource.
type ResourceContent struct {
	URI      string `json:"uri"`
	MimeType string `json:"mimeType,omitempty"`
	Text     string `json:"text,omitempty"`
	Blob     string `json:"blob,omitempty"`
}
