# SPDX-License-Identifier: MIT
#!/usr/bin/env python3
"""
OAuth 2.0 Authorization Code Flow with PKCE

Port of Claude Code's OAuth service (src/services/oauth/) to Python.

Supports:
- PKCE code verifier / challenge (S256)
- Local HTTP server for redirect capture
- Browser-based OAuth login flow
- Token refresh
- User profile fetch
- Persistent token storage at ~/.dxrk/auth/{provider}.json

Usage:
    oauth = OAuthService()
    token = oauth.login("https://auth.example.com", "client123", ["openid", "profile"])
    new_token = oauth.refresh(token["refresh_token"])
    profile = oauth.get_profile(token["access_token"], "https://api.example.com/profile")
"""

import base64
import hashlib
import json
import logging
import os
import secrets
import socket
import sys
import threading
import time
import webbrowser
from http.server import BaseHTTPRequestHandler, HTTPServer
from pathlib import Path
from typing import Any, Optional
from urllib.parse import parse_qs, urlencode, urlparse

logger = logging.getLogger(__name__)


DXRK_HOME = Path(os.environ.get("DXRK_HOME", Path.home() / ".dxrk"))
AUTH_DIR = DXRK_HOME / "auth"


# ---------------------------------------------------------------------------
# PKCE Utilities
# ---------------------------------------------------------------------------

def _base64url_encode(data: bytes) -> str:
    """Base64URL encode without padding."""
    return base64.urlsafe_b64encode(data).rstrip(b"=").decode("ascii")


def generate_code_verifier() -> str:
    """Generate a PKCE code verifier (128 chars, 96 bytes of entropy)."""
    return _base64url_encode(secrets.token_bytes(96))


def generate_code_challenge(verifier: str) -> str:
    """Generate an S256 PKCE code challenge from a verifier."""
    return _base64url_encode(hashlib.sha256(verifier.encode("ascii")).digest())


def generate_state() -> str:
    """Generate a random state string for CSRF protection."""
    return _base64url_encode(secrets.token_bytes(32))


# ---------------------------------------------------------------------------
# Token Storage
# ---------------------------------------------------------------------------

def _token_path(provider: str) -> Path:
    return AUTH_DIR / f"{provider}.json"


def _read_tokens(provider: str) -> Optional[dict]:
    path = _token_path(provider)
    if not path.exists():
        return None
    try:
        return json.loads(path.read_text(encoding="utf-8"))
    except (json.JSONDecodeError, OSError) as exc:
        logger.warning("Failed to read tokens for '%s': %s", provider, exc)
        return None


def _write_tokens(provider: str, data: dict) -> None:
    path = _token_path(provider)
    path.parent.mkdir(parents=True, exist_ok=True)
    tmp = path.with_suffix(".tmp")
    try:
        tmp.write_text(json.dumps(data, indent=2, default=str), encoding="utf-8")
        os.chmod(tmp, 0o600)
        tmp.rename(path)
    except OSError:
        tmp.unlink(missing_ok=True)
        raise


def _clear_tokens(provider: str) -> None:
    path = _token_path(provider)
    path.unlink(missing_ok=True)


# ---------------------------------------------------------------------------
# HTTP helpers (httpx preferred, urllib fallback)
# ---------------------------------------------------------------------------

def _post_form(url: str, data: dict, timeout: int = 15) -> dict:
    """POST application/x-www-form-urlencoded, return parsed JSON."""
    try:
        import httpx
        resp = httpx.post(url, data=data, timeout=timeout)
        resp.raise_for_status()
        return resp.json()
    except ImportError:
        import urllib.request
        encoded = urlencode(data).encode("ascii")
        req = urllib.request.Request(url, data=encoded, method="POST")
        req.add_header("Content-Type", "application/x-www-form-urlencoded")
        with urllib.request.urlopen(req, timeout=timeout) as resp:
            return json.loads(resp.read().decode("utf-8"))


def _get_json(url: str, headers: dict, timeout: int = 10) -> dict:
    """GET and return parsed JSON."""
    try:
        import httpx
        resp = httpx.get(url, headers=headers, timeout=timeout)
        resp.raise_for_status()
        return resp.json()
    except ImportError:
        import urllib.request
        req = urllib.request.Request(url, headers=headers, method="GET")
        with urllib.request.urlopen(req, timeout=timeout) as resp:
            return json.loads(resp.read().decode("utf-8"))


# ---------------------------------------------------------------------------
# Callback HTTP Server
# ---------------------------------------------------------------------------

class _CallbackHandler(BaseHTTPRequestHandler):
    """Ephemeral HTTP server that captures the OAuth redirect."""

    result: dict = {}
    expected_state: str = ""

    def do_GET(self) -> None:
        params = parse_qs(urlparse(self.path).query)
        code = params.get("code", [None])[0]
        state = params.get("state", [None])[0]
        error = params.get("error", [None])[0]
        self.result["code"] = code
        self.result["state"] = state
        self.result["error"] = error

        if error:
            body = (
                f"<html><body><h2>Authorization Failed</h2>"
                f"<p>Error: {error}</p></body></html>"
            )
            self._respond(400, body)
        elif code and state == self.expected_state:
            body = (
                "<html><body><h2>Authorization Successful</h2>"
                "<p>You can close this tab and return to Dxrk.</p></body></html>"
            )
            self._respond(200, body)
        elif code and state != self.expected_state:
            body = "<html><body><h2>State Mismatch</h2><p>CSRF validation failed.</p></body></html>"
            self._respond(400, body)
        else:
            body = "<html><body><h2>No Authorization Code</h2></body></html>"
            self._respond(400, body)

    def _respond(self, status: int, body: str) -> None:
        self.send_response(status)
        self.send_header("Content-Type", "text/html; charset=utf-8")
        self.send_header("Content-Length", str(len(body.encode("utf-8"))))
        self.end_headers()
        self.wfile.write(body.encode("utf-8"))

    def log_message(self, fmt: str, *args: Any) -> None:
        logger.debug("OAuth callback: %s", fmt % args)


def _find_free_port() -> int:
    with socket.socket(socket.AF_INET, socket.SOCK_STREAM) as s:
        s.bind(("127.0.0.1", 0))
        return s.getsockname()[1]


def _wait_for_callback(port: int, expected_state: str, timeout: float = 300.0) -> str:
    """Start a temporary HTTP server on *port*, wait for the OAuth redirect,
    validate state, and return the authorization code.

    Raises RuntimeError on failure or timeout.
    """
    _CallbackHandler.result = {}
    _CallbackHandler.expected_state = expected_state

    server = HTTPServer(("127.0.0.1", port), _CallbackHandler)
    server.timeout = 0.5

    deadline = time.time() + timeout
    try:
        while time.time() < deadline:
            server.handle_request()
            result = _CallbackHandler.result
            if result.get("error"):
                raise RuntimeError(f"OAuth authorization error: {result['error']}")
            if result.get("code"):
                if result.get("state") != expected_state:
                    raise RuntimeError("Invalid state parameter — CSRF attack?")
                return result["code"]
    finally:
        server.server_close()
        _CallbackHandler.result = {}
        _CallbackHandler.expected_state = ""

    raise RuntimeError(
        "OAuth callback timed out. "
        "Complete the authorization in the browser and try again."
    )


# ---------------------------------------------------------------------------
# OAuthService
# ---------------------------------------------------------------------------

class OAuthService:
    """OAuth 2.0 Authorization Code flow with PKCE.

    Port of Claude Code's OAuthService (src/services/oauth/index.ts).
    """

    def login(
        self,
        provider_url: str,
        client_id: str,
        scopes: Optional[list[str]] = None,
        port: int = 0,
        token_url: Optional[str] = None,
        authorize_url: Optional[str] = None,
    ) -> str:
        """Run the full OAuth authorization code flow with PKCE.

        1. Generate PKCE code verifier + challenge
        2. Start local callback server
        3. Open browser to authorization URL
        4. Exchange the returned code for tokens
        5. Store tokens to ~/.dxrk/auth/{provider}.json
        6. Return the access token

        Args:
            provider_url: Base URL of the OAuth provider (used to derive
                endpoints unless *token_url* / *authorize_url* are given).
            client_id: OAuth client ID.
            scopes: Space-separated scope string. Default ``["openid", "profile", "email"]``.
            port: Localhost port for the redirect listener. 0 = auto-pick.
            token_url: Explicit token endpoint. Default ``{provider_url}/oauth/token``.
            authorize_url: Explicit authorize endpoint. Default ``{provider_url}/oauth/authorize``.

        Returns:
            The access token string.

        Raises:
            RuntimeError: If the flow fails at any step.
        """
        provider = urlparse(provider_url).netloc or provider_url.replace("://", "_")
        scopes = scopes or ["openid", "profile", "email"]
        resolved_port = port if port else _find_free_port()
        redirect_uri = f"http://127.0.0.1:{resolved_port}/callback"

        # PKCE
        code_verifier = generate_code_verifier()
        code_challenge = generate_code_challenge(code_verifier)
        state = generate_state()

        # Endpoints
        auth_url_base = authorize_url or f"{provider_url.rstrip('/')}/oauth/authorize"
        tok_url = token_url or f"{provider_url.rstrip('/')}/oauth/token"

        # Build authorization URL
        params = {
            "response_type": "code",
            "client_id": client_id,
            "redirect_uri": redirect_uri,
            "scope": " ".join(scopes),
            "code_challenge": code_challenge,
            "code_challenge_method": "S256",
            "state": state,
        }
        auth_url = f"{auth_url_base}?{urlencode(params)}"

        # Start callback listener in a background thread
        result_holder: dict = {}

        def _listen() -> None:
            try:
                code = _wait_for_callback(resolved_port, state)
                result_holder["code"] = code
            except Exception as exc:
                result_holder["error"] = exc

        listener = threading.Thread(target=_listen, daemon=True)
        listener.start()

        # Small delay so the server is listening before we open the browser
        time.sleep(0.2)

        # Open browser
        self._open_browser(auth_url)

        # Wait for the listener to finish
        listener.join(timeout=310)

        if "error" in result_holder:
            raise result_holder["error"]
        if "code" not in result_holder:
            raise RuntimeError("No authorization code received — timed out.")

        auth_code = result_holder["code"]

        # Exchange code for tokens
        token_data = _post_form(tok_url, {
            "grant_type": "authorization_code",
            "code": auth_code,
            "redirect_uri": redirect_uri,
            "client_id": client_id,
            "code_verifier": code_verifier,
        })

        tokens = {
            "access_token": token_data["access_token"],
            "refresh_token": token_data.get("refresh_token"),
            "expires_in": token_data.get("expires_in"),
            "scope": token_data.get("scope"),
            "token_type": token_data.get("token_type"),
            "expires_at": (
                time.time() + int(token_data["expires_in"])
                if token_data.get("expires_in")
                else None
            ),
            "provider": provider,
            "client_id": client_id,
        }

        _write_tokens(provider, tokens)

        return token_data["access_token"]

    def refresh(self, refresh_token: str, client_id: str, token_url: str) -> str:
        """Refresh an access token.

        Returns:
            The new access token.

        Raises:
            RuntimeError: If the refresh fails.
        """
        token_data = _post_form(token_url, {
            "grant_type": "refresh_token",
            "refresh_token": refresh_token,
            "client_id": client_id,
        })

        new_access = token_data["access_token"]
        new_refresh = token_data.get("refresh_token", refresh_token)

        # Update stored tokens
        provider = urlparse(token_url).netloc or token_url.replace("://", "_")
        stored = _read_tokens(provider) or {}
        stored["access_token"] = new_access
        stored["refresh_token"] = new_refresh
        if "expires_in" in token_data:
            stored["expires_in"] = token_data["expires_in"]
            stored["expires_at"] = time.time() + int(token_data["expires_in"])
        _write_tokens(provider, stored)

        return new_access

    def get_profile(
        self,
        access_token: str,
        profile_url: str,
    ) -> dict:
        """Fetch the user profile using an OAuth access token.

        Args:
            access_token: A valid OAuth access token.
            profile_url: The profile endpoint URL (e.g.
                ``https://api.example.com/oauth/profile``).

        Returns:
            The parsed JSON profile response.
        """
        headers = {
            "Authorization": f"Bearer {access_token}",
            "Content-Type": "application/json",
        }
        return _get_json(profile_url, headers=headers)

    @staticmethod
    def _open_browser(url: str) -> None:
        """Open *url* in the system browser, or print it as a fallback."""
        can_open = False
        if not (os.environ.get("SSH_CLIENT") or os.environ.get("SSH_TTY")):
            if os.name == "nt":
                can_open = True
            else:
                try:
                    can_open = bool(
                        os.uname().sysname == "Darwin"
                        or os.environ.get("DISPLAY")
                        or os.environ.get("WAYLAND_DISPLAY")
                    )
                except AttributeError:
                    can_open = False

        print(f"\n  Open this URL in your browser to authorize:\n\n    {url}\n", file=sys.stderr)

        if can_open:
            try:
                if webbrowser.open(url):
                    print("  (Browser opened automatically.)\n", file=sys.stderr)
                else:
                    print("  (Could not open browser — use the URL above.)\n", file=sys.stderr)
            except Exception:
                print("  (Could not open browser — use the URL above.)\n", file=sys.stderr)


# ---------------------------------------------------------------------------
# Module-level helpers for the tool handler
# ---------------------------------------------------------------------------

_OAUTH_SERVICE = OAuthService()


def _resolve_provider_url(provider: str) -> str:
    """Map a short provider name to a base URL."""
    known = {
        "anthropic": "https://auth.anthropic.com",
        "claude": "https://auth.anthropic.com",
        "github": "https://github.com",
        "google": "https://accounts.google.com",
    }
    if provider in known:
        return known[provider]
    if provider.startswith("http://") or provider.startswith("https://"):
        return provider
    return f"https://{provider}"


# ---------------------------------------------------------------------------
# Tool handler
# ---------------------------------------------------------------------------

def _handler(args: dict, **kw) -> str:
    from tools.registry import tool_error, tool_result

    provider = args.get("provider", "")
    client_id = args.get("client_id", "")
    scopes = args.get("scopes")
    port = args.get("port", 0)
    token_url = args.get("token_url")
    authorize_url = args.get("authorize_url")
    profile_url = args.get("profile_url")

    if not provider:
        return tool_error("'provider' is required")
    if not client_id:
        return tool_error("'client_id' is required")

    try:
        provider_url = _resolve_provider_url(provider)
        svc = OAuthService()
        access_token = svc.login(
            provider_url=provider_url,
            client_id=client_id,
            scopes=scopes,
            port=port,
            token_url=token_url,
            authorize_url=authorize_url,
        )

        result = {
            "access_token": access_token,
            "provider": provider,
        }

        if profile_url:
            try:
                profile = svc.get_profile(access_token, profile_url)
                result["profile"] = profile
            except Exception as exc:
                logger.warning("Profile fetch failed (non-fatal): %s", exc)

        return tool_result(result)
    except Exception as exc:
        return tool_error(f"OAuth login failed: {exc}")


# ---------------------------------------------------------------------------
# Schema
# ---------------------------------------------------------------------------

SCHEMA = {
    "name": "oauth_login",
    "description": (
        "Run the OAuth 2.0 Authorization Code flow with PKCE for a given "
        "provider. Opens a browser for the user to authorize, exchanges the "
        "returned code for tokens, stores them in ~/.dxrk/auth/{provider}.json, "
        "and returns the access token. Optionally fetches the user profile."
    ),
    "parameters": {
        "type": "object",
        "properties": {
            "provider": {
                "type": "string",
                "description": (
                    "OAuth provider name or base URL. Known shortcuts: "
                    "'anthropic'/'claude' (https://auth.anthropic.com), "
                    "'github' (https://github.com), 'google' (https://accounts.google.com). "
                    "Any https:// URL also works."
                ),
            },
            "client_id": {
                "type": "string",
                "description": "OAuth client ID registered with the provider.",
            },
            "scopes": {
                "type": "array",
                "items": {"type": "string"},
                "description": (
                    "OAuth scopes to request. Defaults to "
                    '["openid", "profile", "email"].'
                ),
            },
            "port": {
                "type": "integer",
                "description": (
                    "Localhost port for the OAuth redirect listener. "
                    "Defaults to 0 (auto-pick a free port)."
                ),
            },
            "token_url": {
                "type": "string",
                "description": (
                    "Explicit token endpoint URL. Defaults to "
                    "{provider_url}/oauth/token."
                ),
            },
            "authorize_url": {
                "type": "string",
                "description": (
                    "Explicit authorization endpoint URL. Defaults to "
                    "{provider_url}/oauth/authorize."
                ),
            },
            "profile_url": {
                "type": "string",
                "description": (
                    "Optional profile endpoint URL. If provided, fetches the "
                    "user profile after token exchange and includes it in the result."
                ),
            },
        },
        "required": ["provider", "client_id"],
    },
}


from tools.registry import registry

registry.register(
    name="oauth_login",
    toolset="dev",
    schema=SCHEMA,
    handler=_handler,
    emoji="🔐",
)
