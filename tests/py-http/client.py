"""Lightweight HTTP client + assertion helpers for the MikMongo API.

Each test_*.py file imports `ApiClient`, `section`, and `check` from this
module. The client wraps requests.Session with bearer auth, stores results in
a per-process tally, and provides a colored pass/fail console output.
"""
from __future__ import annotations

import json
import sys
import time
from typing import Any, Optional

import requests

import config


# ---------- ANSI colors ----------
_USE_COLOR = sys.stdout.isatty() and sys.platform != "win32" or "ANSICON" in __import__("os").environ
GREEN = "\033[32m" if _USE_COLOR else ""
RED = "\033[31m" if _USE_COLOR else ""
YELLOW = "\033[33m" if _USE_COLOR else ""
BLUE = "\033[34m" if _USE_COLOR else ""
DIM = "\033[2m" if _USE_COLOR else ""
RESET = "\033[0m" if _USE_COLOR else ""


# ---------- Global tally (used by run_all.py) ----------
class Tally:
    passed = 0
    failed = 0
    skipped = 0
    failures: list[str] = []

    @classmethod
    def reset(cls) -> None:
        cls.passed = 0
        cls.failed = 0
        cls.skipped = 0
        cls.failures = []


# ---------- Auth cache ----------
def load_cached_token() -> Optional[str]:
    if not config.AUTH_CACHE_FILE.exists():
        return None
    try:
        data = json.loads(config.AUTH_CACHE_FILE.read_text())
        return data.get("token")
    except (OSError, json.JSONDecodeError):
        return None


def save_cached_token(token: str, refresh_token: str = "") -> None:
    config.AUTH_CACHE_FILE.write_text(
        json.dumps({"token": token, "refresh_token": refresh_token, "saved_at": int(time.time())}, indent=2)
    )


def get_token() -> Optional[str]:
    return config.SEED_TOKEN or load_cached_token() or None


# ---------- Output helpers ----------
def section(title: str) -> None:
    print(f"\n{BLUE}━━ {title} ━━{RESET}")


def info(msg: str) -> None:
    print(f"{DIM}  · {msg}{RESET}")


def warn(msg: str) -> None:
    print(f"{YELLOW}  ! {msg}{RESET}")


def check(label: str, response: "Result", expect: tuple[int, ...] = (200, 201, 202, 204)) -> bool:
    """Validate a Result, print colored line, update tally. Returns True on pass."""
    if response.status in expect and response.envelope_ok():
        Tally.passed += 1
        print(f"{GREEN}  ✓ {label}{RESET} {DIM}{response.method} {response.path} → {response.status}{RESET}")
        return True
    Tally.failed += 1
    detail = response.error_summary()
    Tally.failures.append(f"{label}: {detail}")
    print(f"{RED}  ✗ {label}{RESET} {DIM}{response.method} {response.path} → {response.status}{RESET}")
    print(f"{RED}     {detail}{RESET}")
    return False


def skip(label: str, reason: str) -> None:
    Tally.skipped += 1
    print(f"{YELLOW}  ⊘ {label}{RESET} {DIM}({reason}){RESET}")


# ---------- Result wrapper ----------
class Result:
    def __init__(self, method: str, path: str, response: Optional[requests.Response], err: Optional[str] = None):
        self.method = method
        self.path = path
        self.response = response
        self.err = err
        self.status = response.status_code if response is not None else 0

    @property
    def json(self) -> Any:
        if self.response is None:
            return None
        try:
            return self.response.json()
        except ValueError:
            return None

    @property
    def data(self) -> Any:
        body = self.json
        if isinstance(body, dict):
            return body.get("data")
        return None

    def envelope_ok(self) -> bool:
        body = self.json
        if not isinstance(body, dict):
            return False
        return bool(body.get("success", False))

    def error_summary(self) -> str:
        if self.err:
            return self.err
        body = self.json
        if isinstance(body, dict):
            err = body.get("error") or body.get("message")
            if err:
                return str(err)
        if self.response is not None:
            text = self.response.text or ""
            return text[:300] if text else f"HTTP {self.status}"
        return "no response"


# ---------- HTTP client ----------
class ApiClient:
    def __init__(self, token: Optional[str] = None, base_url: Optional[str] = None):
        self.base_url = (base_url or config.BASE_URL).rstrip("/")
        self.session = requests.Session()
        self.token = token if token is not None else get_token()
        if self.token:
            self.session.headers["Authorization"] = f"Bearer {self.token}"
        self.session.headers["Accept"] = "application/json"
        self.session.headers["Content-Type"] = "application/json"

    def set_token(self, token: str) -> None:
        self.token = token
        self.session.headers["Authorization"] = f"Bearer {token}"

    def require_token(self, label: str = "request") -> bool:
        if not self.token:
            warn(f"no auth token — run test_01_auth.py first (skipping {label})")
            return False
        return True

    def _do(self, method: str, path: str, **kwargs) -> Result:
        url = f"{self.base_url}{path}"
        if config.VERBOSE:
            body = kwargs.get("json")
            print(f"{DIM}    → {method} {url} {json.dumps(body) if body else ''}{RESET}")
        try:
            response = self.session.request(method, url, timeout=15, **kwargs)
        except requests.RequestException as exc:
            return Result(method, path, None, err=str(exc))
        if config.VERBOSE:
            print(f"{DIM}    ← {response.status_code} {response.text[:200]}{RESET}")
        return Result(method, path, response)

    def get(self, path: str, params: Optional[dict] = None) -> Result:
        return self._do("GET", path, params=params)

    def post(self, path: str, json_body: Optional[dict] = None) -> Result:
        return self._do("POST", path, json=json_body)

    def patch(self, path: str, json_body: Optional[dict] = None) -> Result:
        return self._do("PATCH", path, json=json_body)

    def put(self, path: str, json_body: Optional[dict] = None) -> Result:
        return self._do("PUT", path, json=json_body)

    def delete(self, path: str) -> Result:
        return self._do("DELETE", path)


def print_summary() -> int:
    """Print final tally. Returns suggested process exit code."""
    total = Tally.passed + Tally.failed
    color = GREEN if Tally.failed == 0 else RED
    print(f"\n{color}━━━ {Tally.passed}/{total} passed, {Tally.skipped} skipped ━━━{RESET}")
    if Tally.failures:
        print(f"{RED}Failures:{RESET}")
        for f in Tally.failures:
            print(f"  {RED}- {f}{RESET}")
    return 1 if Tally.failed else 0
