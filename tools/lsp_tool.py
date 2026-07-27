# SPDX-License-Identifier: MIT
#!/usr/bin/env python3
"""
LSP Tool Module — Language Server Protocol integration.

Manages LSP server processes (start/stop), sends JSON-RPC requests
(stdin/stdout), and returns structured results for:
  - lsp_start_server
  - lsp_stop_server
  - lsp_diagnostics
  - lsp_references

Design:
  - LspServer wraps a single subprocess + read/write threads.
  - LspManager is a singleton that tracks servers by project root.
  - JSON-RPC 2.0 over stdio per the LSP spec.
"""

import json
import logging
import os
import queue
import subprocess
import threading
import uuid
from pathlib import Path

from tools.registry import registry, tool_error, tool_result

logger = logging.getLogger(__name__)

_MAX_FILE_SIZE = 10_000_000
_RESPONSE_TIMEOUT = 30.0
_STARTUP_TIMEOUT = 15.0


# ---------------------------------------------------------------------------
# LSP server definitions
# ---------------------------------------------------------------------------

LSP_SERVER_MAP: dict[str, tuple[str, list[str]]] = {
    # (command, args)
    "python": ("pyright-langserver", ["--stdio"]),
    "typescript": ("typescript-language-server", ["--stdio"]),
    "javascript": ("typescript-language-server", ["--stdio"]),
    "go": ("gopls", []),
    "rust": ("rust-analyzer", []),
    "ruby": ("solargraph", ["socket", "--port", "7658"]),
}


def _detect_language(file_path: str) -> str | None:
    """Guess programming language from file extension."""
    ext = Path(file_path).suffix.lower()
    mapping = {
        ".py": "python",
        ".js": "javascript",
        ".jsx": "javascript",
        ".ts": "typescript",
        ".tsx": "typescript",
        ".go": "go",
        ".rs": "rust",
        ".rb": "ruby",
        ".gd": "gdscript",
    }
    return mapping.get(ext)


def _find_project_root(file_path: str) -> str:
    """Walk up from *file_path* looking for sentinel files."""
    root = Path(file_path).resolve().parent
    sentinels = {".git", "pyproject.toml", "package.json", "go.mod",
                 "Cargo.toml", "Gemfile", "project.godot"}
    for parent in [root] + list(root.parents):
        if any((parent / s).exists() for s in sentinels):
            return str(parent)
    return str(root)


# ---------------------------------------------------------------------------
# JSON-RPC helpers
# ---------------------------------------------------------------------------

def _make_request(method: str, params: object = None) -> bytes:
    msg = json.dumps({
        "jsonrpc": "2.0",
        "id": str(uuid.uuid4()),
        "method": method,
        "params": params or {},
    })
    return f"Content-Length: {len(msg)}\r\n\r\n{msg}".encode("utf-8")


def _make_notification(method: str, params: object = None) -> bytes:
    msg = json.dumps({
        "jsonrpc": "2.0",
        "id": None,
        "method": method,
        "params": params or {},
    })
    return f"Content-Length: {len(msg)}\r\n\r\n{msg}".encode("utf-8")


def _parse_response(raw: str) -> dict | None:
    """Parse a single JSON-RPC response from raw LSP output."""
    try:
        return json.loads(raw)
    except json.JSONDecodeError:
        return None


class _ResponseReader(threading.Thread):
    """Background thread that reads LSP stdout and dispatches responses."""

    def __init__(self, stream, pending: dict[str, queue.Queue],
                 log_queue: queue.Queue):
        super().__init__(daemon=True)
        self._stream = stream
        self._pending = pending
        self._log_queue = log_queue
        self._buf = b""

    def run(self):
        while True:
            chunk = self._stream.readline()
            if not chunk:
                break
            self._buf += chunk
            while b"\r\n\r\n" in self._buf:
                header, _, rest = self._buf.partition(b"\r\n\r\n")
                header_str = header.decode("utf-8", errors="replace")
                content_length = 0
                for line in header_str.split("\r\n"):
                    if line.lower().startswith("content-length:"):
                        try:
                            content_length = int(line.split(":")[1].strip())
                        except ValueError:
                            pass
                if content_length and len(rest) >= content_length:
                    body = rest[:content_length].decode("utf-8")
                    self._buf = rest[content_length:]
                    self._dispatch(body)
                else:
                    break

    def _dispatch(self, body: str):
        msg = _parse_response(body)
        if msg is None:
            self._log_queue.put(body)
            return
        req_id = msg.get("id")
        if req_id is not None and str(req_id) in self._pending:
            self._pending[str(req_id)].put(msg)
        elif "method" in msg:
            pass
        else:
            self._log_queue.put(body)


# ---------------------------------------------------------------------------
# LspServer
# ---------------------------------------------------------------------------

class LspServer:
    """Manages a single LSP server subprocess over stdio."""

    def __init__(self, project_root: str, command: str, args: list[str]):
        self.project_root = project_root
        self._command = command
        self._args = args
        self._proc: subprocess.Popen | None = None
        self._pending: dict[str, queue.Queue] = {}
        self._log_queue: queue.Queue = queue.Queue()
        self._reader: _ResponseReader | None = None
        self._lock = threading.Lock()
        self._open_files: set[str] = set()
        self._capabilities: dict | None = None

    def start(self) -> str | None:
        """Start the LSP server. Returns error message or None on success."""
        if self._proc:
            return None
        try:
            self._proc = subprocess.Popen(
                [self._command, *self._args],
                stdin=subprocess.PIPE,
                stdout=subprocess.PIPE,
                stderr=subprocess.DEVNULL,
                cwd=self.project_root,
            )
        except FileNotFoundError:
            return f"LSP server '{self._command}' not found on PATH"
        except Exception as e:
            return f"Failed to start LSP server: {e}"

        self._reader = _ResponseReader(
            self._proc.stdout, self._pending, self._log_queue,
        )
        self._reader.start()

        err = self._initialize()
        if err:
            self.stop()
            return err
        return None

    def stop(self):
        with self._lock:
            if not self._proc:
                return
            try:
                shutdown = _make_request("shutdown")
                self._proc.stdin.write(shutdown)
                self._proc.stdin.flush()
                exit_notif = _make_notification("exit")
                self._proc.stdin.write(exit_notif)
                self._proc.stdin.flush()
            except Exception:
                pass
            self._proc.terminate()
            try:
                self._proc.wait(timeout=5)
            except subprocess.TimeoutExpired:
                self._proc.kill()
            self._proc = None
            self._reader = None
            self._open_files.clear()
            self._capabilities = None
            self._pending.clear()

    @property
    def running(self) -> bool:
        return self._proc is not None

    def is_file_open(self, file_path: str) -> bool:
        return file_path in self._open_files

    def open_file(self, file_path: str, content: str):
        uri = Path(file_path).as_uri()
        notif = _make_notification("textDocument/didOpen", {
            "textDocument": {
                "uri": uri,
                "languageId": _detect_language(file_path) or "plaintext",
                "version": 1,
                "text": content,
            },
        })
        self._send_notification(notif)
        self._open_files.add(file_path)

    def close_file(self, file_path: str):
        uri = Path(file_path).as_uri()
        notif = _make_notification("textDocument/didClose", {
            "textDocument": {"uri": uri},
        })
        self._send_notification(notif)
        self._open_files.discard(file_path)

    def send_request(self, method: str, params: object) -> dict | None:
        req_id = str(uuid.uuid4())
        q: queue.Queue = queue.Queue()
        self._pending[req_id] = q
        raw = _make_request(method, params)
        self._send(raw)
        try:
            resp = q.get(timeout=_RESPONSE_TIMEOUT)
        except queue.Empty:
            self._pending.pop(req_id, None)
            return None
        finally:
            self._pending.pop(req_id, None)

        if "error" in resp:
            logger.warning("LSP error for %s: %s", method, resp["error"])
            return None
        return resp.get("result")

    # -- Internal helpers --

    def _send(self, data: bytes):
        with self._lock:
            if self._proc and self._proc.stdin:
                try:
                    self._proc.stdin.write(data)
                    self._proc.stdin.flush()
                except BrokenPipeError:
                    self.stop()

    def _send_notification(self, data: bytes):
        self._send(data)

    def _initialize(self) -> str | None:
        init_params = {
            "processId": os.getpid(),
            "rootUri": Path(self.project_root).as_uri(),
            "capabilities": {
                "textDocument": {
                    "synchronization": {
                        "didOpen": True,
                        "didClose": True,
                    },
                    "references": {},
                    "diagnostics": {},
                },
                "workspace": {
                    "diagnostics": {},
                },
            },
        }
        result = self.send_request("initialize", init_params)
        if result is None:
            return "LSP server did not respond to initialize"
        if isinstance(result, dict):
            self._capabilities = result.get("capabilities", {})
        notif = _make_notification("initialized", {})
        self._send_notification(notif)
        return None

    def get_diagnostics(self, file_path: str, content: str) -> list[dict]:
        if file_path not in self._open_files:
            self.open_file(file_path, content)

        uri = Path(file_path).as_uri()
        notif = _make_notification("textDocument/didChange", {
            "textDocument": {
                "uri": uri,
                "version": 2,
            },
            "contentChanges": [{"text": content}],
        })
        self._send_notification(notif)

        result = self.send_request("textDocument/diagnostic", {
            "textDocument": {"uri": uri},
        })
        if result is None:
            return []

        kind = result.get("kind", "full")
        if kind == "full":
            return result.get("items", [])
        return result.get("items", [])


# ---------------------------------------------------------------------------
# LspManager (singleton)
# ---------------------------------------------------------------------------

class LspManager:
    """Tracks active LSP servers by project root."""

    def __init__(self):
        self._servers: dict[str, LspServer] = {}

    def get_or_start(self, project_root: str, file_path: str) -> LspServer | str:
        """Return server for *project_root*, starting it if missing.
        Returns LspServer on success, or error string on failure.
        """
        existing = self._servers.get(project_root)
        if existing and existing.running:
            return existing

        lang = _detect_language(file_path)
        if not lang:
            return f"Unsupported language for file: {file_path}"
        entry = LSP_SERVER_MAP.get(lang)
        if not entry:
            return f"No LSP server configured for language: {lang}"

        command, args = entry
        server = LspServer(project_root, command, args)
        err = server.start()
        if err:
            return err
        self._servers[project_root] = server
        return server

    def stop_server(self, project_root: str) -> bool:
        server = self._servers.pop(project_root, None)
        if server:
            server.stop()
            return True
        return False

    def stop_all(self):
        for server in self._servers.values():
            server.stop()
        self._servers.clear()


_manager = LspManager()


# ---------------------------------------------------------------------------
# Schema: lsp_start_server
# ---------------------------------------------------------------------------

SCHEMA_START = {
    "name": "lsp_start_server",
    "description": "Start an LSP server for a given file path. Detects project root and language automatically.",
    "parameters": {
        "type": "object",
        "properties": {
            "file_path": {
                "type": "string",
                "description": "Path to a file in the project (used to determine project root and language)",
            },
        },
        "required": ["file_path"],
    },
}


def _handle_start(args: dict, **kw) -> str:
    file_path = args["file_path"]
    abs_path = str(Path(file_path).resolve())
    project_root = _find_project_root(abs_path)
    server = _manager.get_or_start(project_root, abs_path)

    if isinstance(server, str):
        return tool_error(server)

    return tool_result(
        status="started",
        project_root=project_root,
        language=_detect_language(abs_path),
    )


# ---------------------------------------------------------------------------
# Schema: lsp_stop_server
# ---------------------------------------------------------------------------

SCHEMA_STOP = {
    "name": "lsp_stop_server",
    "description": "Stop the LSP server for a given project root.",
    "parameters": {
        "type": "object",
        "properties": {
            "project_root": {
                "type": "string",
                "description": "Project root path for the server to stop",
            },
        },
        "required": ["project_root"],
    },
}


def _handle_stop(args: dict, **kw) -> str:
    root = args["project_root"]
    if _manager.stop_server(root):
        return tool_result(status="stopped", project_root=root)
    return tool_error(f"No active LSP server for project root: {root}")


# ---------------------------------------------------------------------------
# Schema: lsp_diagnostics
# ---------------------------------------------------------------------------

SCHEMA_DIAGNOSTICS = {
    "name": "lsp_diagnostics",
    "description": "Get LSP diagnostics for a file (syntax errors, warnings, etc.). Opens the file on the server if not already open.",
    "parameters": {
        "type": "object",
        "properties": {
            "file_path": {
                "type": "string",
                "description": "Path to the file to diagnose",
            },
        },
        "required": ["file_path"],
    },
}


def _handle_diagnostics(args: dict, **kw) -> str:
    file_path = args["file_path"]
    abs_path = str(Path(file_path).resolve())
    project_root = _find_project_root(abs_path)

    server = _manager.get_or_start(project_root, abs_path)
    if isinstance(server, str):
        return tool_error(server)

    try:
        content = Path(abs_path).read_text(encoding="utf-8")
    except FileNotFoundError:
        return tool_error(f"File not found: {abs_path}")
    except Exception as e:
        return tool_error(f"Cannot read file: {e}")

    if len(content) > _MAX_FILE_SIZE:
        return tool_error(f"File too large ({len(content)} bytes)")

    items = server.get_diagnostics(abs_path, content)
    return tool_result(
        file_path=abs_path,
        diagnostics=items,
        count=len(items),
    )


# ---------------------------------------------------------------------------
# Schema: lsp_references
# ---------------------------------------------------------------------------

SCHEMA_REFERENCES = {
    "name": "lsp_references",
    "description": "Find all references to a symbol at a given position in a file.",
    "parameters": {
        "type": "object",
        "properties": {
            "file_path": {
                "type": "string",
                "description": "Path to the file",
            },
            "line": {
                "type": "integer",
                "description": "Line number (1-based)",
            },
            "character": {
                "type": "integer",
                "description": "Character offset (1-based)",
            },
        },
        "required": ["file_path", "line", "character"],
    },
}


def _handle_references(args: dict, **kw) -> str:
    file_path = args["file_path"]
    line = args["line"]
    character = args["character"]
    abs_path = str(Path(file_path).resolve())
    project_root = _find_project_root(abs_path)

    server = _manager.get_or_start(project_root, abs_path)
    if isinstance(server, str):
        return tool_error(server)

    try:
        content = Path(abs_path).read_text(encoding="utf-8")
    except FileNotFoundError:
        return tool_error(f"File not found: {abs_path}")
    except Exception as e:
        return tool_error(f"Cannot read file: {e}")

    if server.is_file_open(abs_path):
        pass
    else:
        server.open_file(abs_path, content)

    uri = Path(abs_path).as_uri()
    result = server.send_request("textDocument/references", {
        "textDocument": {"uri": uri},
        "position": {"line": line - 1, "character": character - 1},
        "context": {"includeDeclaration": True},
    })

    if result is None:
        return tool_result(references=[], count=0)

    refs = []
    for loc in (result if isinstance(result, list) else [result]):
        uri = loc.get("uri", "")
        rng = loc.get("range", {})
        refs.append({
            "uri": uri,
            "range": rng,
            "file_path": uri.replace("file://", ""),
        })

    return tool_result(
        file_path=abs_path,
        references=refs,
        count=len(refs),
    )


# ---------------------------------------------------------------------------
# Registration
# ---------------------------------------------------------------------------

registry.register(
    name="lsp_start_server",
    toolset="dev",
    schema=SCHEMA_START,
    handler=_handle_start,
    emoji="🚀",
    max_result_size_chars=5000,
)

registry.register(
    name="lsp_stop_server",
    toolset="dev",
    schema=SCHEMA_STOP,
    handler=_handle_stop,
    emoji="⏹️",
    max_result_size_chars=2000,
)

registry.register(
    name="lsp_diagnostics",
    toolset="dev",
    schema=SCHEMA_DIAGNOSTICS,
    handler=_handle_diagnostics,
    emoji="🔍",
    max_result_size_chars=50_000,
)

registry.register(
    name="lsp_references",
    toolset="dev",
    schema=SCHEMA_REFERENCES,
    handler=_handle_references,
    emoji="🔗",
    max_result_size_chars=50_000,
)
