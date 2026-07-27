# Dxrk API Reference

> Auto-generated reference for all Dxrk HTTP, WebSocket, and MCP endpoints.

---

## Table of Contents

1. [Overview](#overview)
2. [WebUI API](#webui-api)
3. [Dxrk-Memory REST API](#dxrk-memory-rest-api)
4. [Dxrk-Memory Cloud API](#dxrk-memory-cloud-api)
5. [MCP Protocol](#mcp-protocol)
6. [WebSocket Protocol](#websocket-protocol)
7. [Error Handling](#error-handling)
8. [Data Types](#data-types)

---

## Overview

Dxrk exposes multiple servers for different purposes:

| Server | Default Address | Auth | Transport |
|--------|----------------|------|-----------|
| **WebUI** | `http://localhost:8080` | None (local) | HTTP + WebSocket |
| **Dxrk-Memory REST** | `http://127.0.0.1:{port}` | None (local) | HTTP |
| **Dxrk-Memory Cloud** | `{deployed URL}` | Bearer token | HTTPS |
| **MCP** | stdio or `127.0.0.1:{port}` | None (local) | stdio / TCP |

All WebUI responses include an `X-Dxrk-Version` header. CORS is fully permissive (`*`).

---

## WebUI API

> Source: `internal/webui/server.go`

### Base URL

```
http://localhost:8080
```

---

### GET /api/status

Returns full system status including version, uptime, providers, and subsystem health.

**Response** — `StatusResponse` (200)

```json
{
  "version": "0.1.0",
  "uptime": "2h34m12s",
  "status": "running",
  "providers": [
    {
      "name": "openai",
      "model": "gpt-4o",
      "healthy": true,
      "cost": 0.0
    }
  ],
  "autonomy": {
    "enabled": false,
    "iq": 0,
    "patterns": 0,
    "permissions": {},
    "updating": false,
    "verifying": false,
    "learning": false,
    "evolving": false
  },
  "rag": {
    "enabled": true,
    "vectors": 1024,
    "dimensions": 1536,
    "files_indexed": 42,
    "last_index": "2026-07-25T10:00:00Z"
  },
  "vault": {
    "enabled": true,
    "secrets": 3,
    "secret_names": ["API_KEY", "DB_PASS", "TOKEN"]
  },
  "cache": {
    "enabled": true,
    "size": 128,
    "max_size": 1024,
    "hits": 512,
    "ttl": "5m"
  },
  "pipeline": {
    "running": true,
    "last_run": "2026-07-25T12:00:00Z",
    "iterations": 10,
    "success": true
  },
  "requests": {
    "total": 1234,
    "errors": 5
  }
}
```

`server.go:196`

---

### GET /api/health

Simple health check.

**Response** (200)

```json
{ "status": "ok" }
```

`server.go:274`

---

### GET /api/config

Returns the current WebUI configuration.

**Response** — `WebUIConfig` JSON (200)

`server.go:278`

---

### GET /api/providers

Returns status of all configured LLM providers.

**Response** — `[]providerStatus` (200)

```json
[
  {
    "name": "openai",
    "model": "gpt-4o",
    "healthy": true,
    "cost": 0.0
  }
]
```

`server.go:282`

---

### GET /

Serves static WebUI files. Falls back to JSON response if no static files are embedded.

**Response** — static files or JSON (200)

`server.go:182`

---

## Dxrk-Memory REST API

> Source: `integrations/dxrk-memory/internal/server/server.go`

### Base URL

```
http://127.0.0.1:{port}
```

All routes use Go 1.22 method-based routing (`GET /path`, `POST /path`, etc.).

---

### Health

#### GET /health

**Response** (200)

```json
{
  "status": "ok",
  "service": "dxrk-memory",
  "version": "0.1.0"
}
```

`server.go:203`

---

### Sessions

#### POST /sessions

Create a new session.

**Request Body**

```json
{
  "project": "my-project",
  "directory": "/path/to/project"
}
```

**Response** (201)

```json
{
  "id": "sess_abc123",
  "status": "created"
}
```

`server.go:211`

#### POST /sessions/{id}/end

End a session with an optional summary.

**Request Body**

```json
{
  "summary": "Worked on auth module"
}
```

**Response** (200)

```json
{
  "id": "sess_abc123",
  "status": "completed"
}
```

`server.go:235`

#### GET /sessions/recent

List recent sessions.

**Query Parameters**

| Param | Type | Description |
|-------|------|-------------|
| `project` | string | Filter by project name |
| `limit` | int | Max results (default 20) |

**Response** — `[]Session` (200)

`server.go:252`

#### GET /sessions/{id}

Get a single session by ID.

**Response** — `Session` (200)

`server.go:265`

#### DELETE /sessions/{id}

Delete a session.

**Response** (200)

```json
{
  "id": "sess_abc123",
  "status": "deleted"
}
```

`server.go:529`

---

### Observations

#### POST /observations

Add a new observation.

**Request Body** — `AddObservationParams`

```json
{
  "content": "Fixed the memory leak in the connection pool",
  "type": "decision",
  "project": "dxrk",
  "scope": "backend",
  "session_id": "sess_abc123"
}
```

**Response** (201)

```json
{
  "id": "obs_xyz789",
  "status": "saved"
}
```

`server.go:279`

#### GET /observations

Alias for `/observations/recent`.

**Query Parameters**

| Param | Type | Description |
|-------|------|-------------|
| `sort` | string | Sort order |

`server.go:327`

#### POST /observations/passive

Passively capture an observation from ambient context.

**Request Body** — `PassiveCaptureParams`

**Response** — `PassiveCaptureResult`

`server.go:303`

#### GET /observations/recent

List recent observations.

**Query Parameters**

| Param | Type | Description |
|-------|------|-------------|
| `project` | string | Filter by project |
| `scope` | string | Filter by scope (e.g. `backend`, `frontend`) |
| `limit` | int | Max results (default 20) |

**Response** — `[]Observation` (200)

`server.go:337`

#### GET /observations/{id}

Get a single observation.

**Response** — `Observation` (200)

`server.go:164`

#### PATCH /observations/{id}

Update an observation.

**Request Body** — `UpdateObservationParams`

**Response** — `Observation` (200)

`server.go:389`

#### DELETE /observations/{id}

Delete an observation.

**Query Parameters**

| Param | Type | Description |
|-------|------|-------------|
| `hard` | bool | If `true`, permanently delete (default soft delete) |

**Response** (200)

```json
{
  "id": "obs_xyz789",
  "status": "deleted",
  "hard_delete": false
}
```

`server.go:418`

---

### Search

#### GET /search

Full-text search across observations.

**Query Parameters**

| Param | Type | Description |
|-------|------|-------------|
| `q` | string | Search query |
| `type` | string | Filter by observation type |
| `project` | string | Filter by project |
| `scope` | string | Filter by scope |
| `limit` | int | Max results |

**Response** — `[]Observation` (200)

`server.go:351`

---

### Timeline

#### GET /timeline

Get a chronological timeline of observations.

**Query Parameters**

| Param | Type | Description |
|-------|------|-------------|
| `observation_id` | string | Anchor observation ID |
| `before` | string | ISO timestamp — entries before this time |
| `after` | string | ISO timestamp — entries after this time |

**Response** — `Timeline` (200)

`server.go:445`

---

### Prompts

#### POST /prompts

Save a prompt template.

**Request Body** — `AddPromptParams`

```json
{
  "content": "Summarize the following code changes...",
  "project": "dxrk",
  "name": "code-summary"
}
```

**Response** (201)

```json
{
  "id": "prompt_abc",
  "status": "saved"
}
```

`server.go:472`

#### GET /prompts/recent

List recent prompts.

**Query Parameters**

| Param | Type | Description |
|-------|------|-------------|
| `project` | string | Filter by project |
| `limit` | int | Max results |

**Response** — `[]Prompt` (200)

`server.go:496`

#### GET /prompts/search

Search prompts.

**Query Parameters**

| Param | Type | Description |
|-------|------|-------------|
| `q` | string | Search query |
| `project` | string | Filter by project |
| `limit` | int | Max results |

**Response** — `[]Prompt` (200)

`server.go:509`

#### DELETE /prompts/{id}

Delete a prompt.

**Response** (200)

```json
{
  "id": "prompt_abc",
  "status": "deleted"
}
```

`server.go:554`

---

### Context

#### GET /context

Build a context string for the current project/scope.

**Query Parameters**

| Param | Type | Description |
|-------|------|-------------|
| `project` | string | Project name |
| `scope` | string | Scope filter |

**Response** (200)

```json
{
  "context": "Recent decisions: switched to ChromaDB for vector store..."
}
```

`server.go:632`

---

### Export / Import

#### GET /export

Export all data for a project as a JSON download.

**Query Parameters**

| Param | Type | Description |
|-------|------|-------------|
| `project` | string | Project to export |

**Response** — `ExportData` JSON (200, `Content-Disposition: attachment`)

`server.go:577`

#### POST /import

Import data from a JSON body. Max request size: **50 MB**.

**Request Body** — `ExportData` JSON

**Response** — `ImportResult` (200)

`server.go:605`

---

### Stats / Diagnostics

#### GET /stats

Returns store statistics.

**Response** — `store.Stats` (200)

`server.go:645`

#### GET /doctor

Runs diagnostics and returns a report.

**Response** — `diagnostic.Report` (200)

`server.go:655`

---

### Project Detection

#### GET /project/current

Detect the current project from a working directory.

**Query Parameters**

| Param | Type | Description |
|-------|------|-------------|
| `cwd` | string | Current working directory |

**Response** (200)

```json
{
  "project": "dxrk",
  "project_source": "directory",
  "project_path": "/home/user/Dxrk",
  "cwd": "/home/user/Dxrk/internal",
  "available_projects": ["dxrk", "dxrk-memory", "dotfiles"]
}
```

`server.go:722`

#### POST /projects/migrate

Migrate all data from one project name to another.

**Request Body**

```json
{
  "old_project": "old-name",
  "new_project": "new-name"
}
```

**Response** (200)

```json
{
  "status": "ok",
  "old_project": "old-name",
  "new_project": "new-name",
  "observations": 42,
  "sessions": 5,
  "prompts": 3
}
```

`server.go:784`

---

### Sync Status

#### GET /sync/status

**Response** (200)

```json
{
  "enabled": true,
  "phase": "idle",
  "last_error": "",
  "consecutive_failures": 0
}
```

`server.go:753`

---

### Conflicts

#### GET /conflicts

List conflict relations with optional filters.

**Query Parameters**

| Param | Type | Description |
|-------|------|-------------|
| `project` | string | Filter by project |
| `status` | string | Filter by status |
| `since` | string | ISO timestamp |
| `limit` | int | Page size |
| `offset` | int | Offset for pagination |

**Response** (200)

```json
{
  "total": 10,
  "limit": 20,
  "offset": 0,
  "relations": [...]
}
```

`server.go:850`

#### GET /conflicts/stats

Get conflict statistics for a project.

**Query Parameters**

| Param | Type | Description |
|-------|------|-------------|
| `project` | string | Project name |

**Response** (200)

```json
{
  "project": "dxrk",
  "by_relation": {},
  "by_judgment_status": {},
  "deferred": 0,
  "dead": 0
}
```

`server.go:901`

#### GET /conflicts/deferred

List deferred conflicts.

**Query Parameters**

| Param | Type | Description |
|-------|------|-------------|
| `status` | string | Filter by status |
| `limit` | int | Page size |
| `offset` | int | Offset |

**Response** (200)

```json
{
  "total": 5,
  "limit": 20,
  "rows": [...]
}
```

`server.go:921`

#### POST /conflicts/scan

Scan for new conflicts in a project.

**Request Body**

```json
{
  "project": "dxrk",
  "since": "2026-07-01T00:00:00Z",
  "apply": false,
  "max_insert": 10,
  "semantic": true,
  "concurrency": 4
}
```

**Response** — scan result (200)

`server.go:958`

#### POST /conflicts/judge

Submit a judgment on a conflict.

**Request Body**

```json
{
  "judgment_id": "jdg_abc",
  "relation": "contradicts",
  "reason": "The later observation supersedes the earlier one",
  "evidence": "obs_xyz supersedes obs_def",
  "confidence": 0.95,
  "session_id": "sess_123"
}
```

**Response** (200)

```json
{
  "relation": "contradicts"
}
```

`server.go:1084`

#### POST /conflicts/compare

Compare two memories for conflict detection.

**Request Body**

```json
{
  "memory_id_a": "obs_abc",
  "memory_id_b": "obs_def",
  "relation": "contradicts",
  "confidence": 0.8,
  "reasoning": "Both describe the same config value with different settings",
  "model": "gpt-4o"
}
```

**Response** (200)

```json
{
  "sync_id": "sync_xyz"
}
```

`server.go:1140`

#### POST /conflicts/deferred/replay

Replay deferred conflict judgments.

**Response** (200)

```json
{
  "retried": 3,
  "succeeded": 2,
  "failed": 1,
  "dead": 0
}
```

`server.go:1210`

#### GET /conflicts/{relation_id}

Get details for a specific conflict relation.

**Response** — `ConflictDetail` (200)

`server.go:1228`

---

## Dxrk-Memory Cloud API

> Source: `integrations/dxrk-memory/internal/cloud/cloudserver/`

### Authentication

All requests require a `Bearer` token in the `Authorization` header:

```
Authorization: Bearer <token>
```

### Dashboard Routes

Dashboard routes are served under `/dashboard/*`. Access is session-gated — a valid session cookie or token is required. Admin routes (marked with `admin` in the handler) require elevated privileges.

| Route | Method | Description |
|-------|--------|-------------|
| `/dashboard/` | GET | Dashboard index |
| `/dashboard/sessions` | GET | Session list |
| `/dashboard/observations` | GET | Observation list |
| `/dashboard/stats` | GET | Aggregate statistics |
| *(admin)* `/dashboard/admin/*` | GET | Admin-only views |

### Sync API

Sync endpoints enable bidirectional data synchronization between local Dxrk-Memory and cloud.

#### GET /sync/pull

Pull changes from the cloud.

**Query Parameters**

| Param | Type | Description |
|-------|------|-------------|
| `since` | string | ISO timestamp — pull changes after this time |

**Response** — chunked data

#### GET /sync/pull/{chunkID}

Pull a specific sync chunk by ID.

**Response** — chunk data

#### POST /sync/push

Push local changes to the cloud.

**Request Body** — local change set

**Response** — push result

#### POST /sync/mutations/push

Push fine-grained mutations.

**Request Body** — mutation array

**Response** — mutation push result

#### GET /sync/mutations/pull

Pull pending mutations from the cloud.

**Query Parameters**

| Param | Type | Description |
|-------|------|-------------|
| `since` | string | ISO timestamp |

**Response** — mutation list

---

## MCP Protocol

> Source: `internal/mcp/server.go`

Dxrk implements the Model Context Protocol (MCP) over **stdio** or **TCP** transport.

### Methods

| Method | Description |
|--------|-------------|
| `initialize` | Handshake — exchange capabilities and protocol version |
| `tools/list` | List available tools |
| `tools/call` | Invoke a tool by name with arguments |
| `resources/list` | List available resources |
| `health/ping` | Health check |

### JSON-RPC Request Format

```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "method": "tools/call",
  "params": {
    "name": "dxrk_status",
    "arguments": {}
  }
}
```

### JSON-RPC Response Format

```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "result": {
    "content": [
      {
        "type": "text",
        "text": "Dxrk v0.1.0 — status: running"
      }
    ]
  }
}
```

---

## WebSocket Protocol

> Source: `internal/webui/server.go`

Connect to `ws://localhost:8080/ws` for real-time communication.

### Message Format

All messages are JSON. Client sends requests, server sends responses or streamed data.

### Methods

#### ping

```json
// Client → Server
{"method": "ping"}

// Server → Client
{"type": "pong"}
```

#### get_status

```json
// Client → Server
{"method": "get_status"}

// Server → Client (full StatusResponse)
{
  "type": "status",
  "version": "0.1.0",
  "uptime": "2h34m12s",
  "status": "running",
  "providers": [...],
  "autonomy": {...},
  "rag": {...},
  "vault": {...},
  "cache": {...},
  "pipeline": {...},
  "requests": {...}
}
```

#### subscribe_logs

```json
// Client → Server
{"method": "subscribe_logs"}

// Server → Client (streaming)
{"type": "log", "level": "info", "time": "2026-07-25T12:00:00Z", "message": "Provider openai responding"}
{"type": "log", "level": "error", "time": "2026-07-25T12:00:01Z", "message": "Connection timeout"}
```

---

## Error Handling

### HTTP Errors

All servers return standard HTTP status codes:

| Code | Meaning |
|------|---------|
| `200` | Success |
| `201` | Created |
| `400` | Bad request — invalid body or parameters |
| `404` | Resource not found |
| `409` | Conflict — resource state conflict |
| `413` | Payload too large (Dxrk-Memory import: >50 MB) |
| `429` | Rate limited |
| `500` | Internal server error |

### Error Response Body

```json
{
  "error": "description of what went wrong"
}
```

### MCP Errors

MCP follows JSON-RPC error format:

```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "error": {
    "code": -32601,
    "message": "Method not found"
  }
}
```

---

## Data Types

### StatusResponse

| Field | Type | Description |
|-------|------|-------------|
| `version` | string | Dxrk version |
| `uptime` | string | Human-readable uptime |
| `status` | string | System status (`running`, `stopped`, etc.) |
| `providers` | `[]providerStatus` | LLM provider statuses |
| `autonomy` | `autonomyStatus` | Autonomy subsystem state |
| `rag` | `ragStatus` | RAG subsystem state |
| `vault` | `vaultStatus` | Secret vault state |
| `cache` | `cacheStatus` | Cache state |
| `pipeline` | `pipelineStatus` | Pipeline state |
| `requests` | object | Request counts (`total`, `errors`) |

### providerStatus

| Field | Type | Description |
|-------|------|-------------|
| `name` | string | Provider name (e.g. `openai`, `anthropic`) |
| `model` | string | Model identifier |
| `healthy` | bool | Whether the provider is reachable |
| `cost` | float | Current cost accumulation |

### autonomyStatus

| Field | Type | Description |
|-------|------|-------------|
| `enabled` | bool | Whether autonomy is active |
| `iq` | int | Autonomy intelligence quotient |
| `patterns` | int | Detected patterns count |
| `permissions` | object | Permission map |
| `updating` | bool | Currently updating |
| `verifying` | bool | Currently verifying |
| `learning` | bool | Currently learning |
| `evolving` | bool | Currently evolving |

### ragStatus

| Field | Type | Description |
|-------|------|-------------|
| `enabled` | bool | Whether RAG is active |
| `vectors` | int | Number of stored vectors |
| `dimensions` | int | Vector dimensions |
| `files_indexed` | int | Files in the index |
| `last_index` | string | ISO timestamp of last indexing |

### vaultStatus

| Field | Type | Description |
|-------|------|-------------|
| `enabled` | bool | Whether vault is active |
| `secrets` | int | Number of stored secrets |
| `secret_names` | `[]string` | Names of stored secrets |

### cacheStatus

| Field | Type | Description |
|-------|------|-------------|
| `enabled` | bool | Whether cache is active |
| `size` | int | Current entries |
| `max_size` | int | Max entries |
| `hits` | int | Total cache hits |
| `ttl` | string | Time-to-live (e.g. `5m`) |

### pipelineStatus

| Field | Type | Description |
|-------|------|-------------|
| `running` | bool | Whether pipeline is active |
| `last_run` | string | ISO timestamp of last run |
| `iterations` | int | Total iterations |
| `success` | bool | Last run success |

### Session

| Field | Type | Description |
|-------|------|-------------|
| `id` | string | Session ID |
| `project` | string | Project name |
| `directory` | string | Working directory |
| `status` | string | `created`, `completed`, `deleted` |
| `summary` | string | Session summary (optional) |

### Observation

| Field | Type | Description |
|-------|------|-------------|
| `id` | string | Observation ID |
| `content` | string | Observation text |
| `type` | string | Type (e.g. `decision`, `bug`, `insight`) |
| `project` | string | Project name |
| `scope` | string | Scope (e.g. `backend`, `frontend`) |
| `session_id` | string | Associated session (optional) |

### Prompt

| Field | Type | Description |
|-------|------|-------------|
| `id` | string | Prompt ID |
| `name` | string | Prompt name |
| `content` | string | Prompt template text |
| `project` | string | Project name |

### ConflictDetail

| Field | Type | Description |
|-------|------|-------------|
| `relation_id` | string | Relation ID |
| `memory_id_a` | string | First memory ID |
| `memory_id_b` | string | Second memory ID |
| `relation` | string | Relation type (e.g. `contradicts`) |
| `confidence` | float | Confidence score |
| `reasoning` | string | Reasoning text |
| `judgment_status` | string | Judgment status |
| `created_at` | string | ISO timestamp |

### ExportData

Full project data export including observations, sessions, prompts, and metadata. Structure mirrors the import format for round-trip compatibility.

---

*Generated from source references in `internal/webui/server.go`, `integrations/dxrk-memory/internal/server/server.go`, `integrations/dxrk-memory/internal/cloud/cloudserver/`, and `internal/mcp/server.go`.*
