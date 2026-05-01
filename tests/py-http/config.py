"""Shared configuration for the MikMongo Python HTTP test suite.

Loads .env (if python-dotenv is installed) and exposes constants used by
client.py and every test_*.py file.
"""
from __future__ import annotations

import os
from pathlib import Path

try:
    from dotenv import load_dotenv
    load_dotenv(Path(__file__).parent / ".env")
except ImportError:
    pass


def _env(key: str, default: str = "") -> str:
    return os.environ.get(key, default).strip()


def _bool(key: str, default: bool = False) -> bool:
    raw = _env(key, "1" if default else "0").lower()
    return raw in ("1", "true", "yes", "on")


HERE = Path(__file__).parent
AUTH_CACHE_FILE = HERE / ".auth_cache.json"

BASE_URL = _env("MIKMONGO_BASE_URL", "http://localhost:8080").rstrip("/")
EMAIL = _env("MIKMONGO_EMAIL", "superadmin@mikmongo.local")
PASSWORD = _env("MIKMONGO_PASSWORD", "")
SEED_TOKEN = _env("MIKMONGO_TOKEN", "")

ROUTER_ID = _env("MIKMONGO_ROUTER_ID", "9eab7675-3abd-41df-97fc-1c8137367f5a")
CUSTOMER_ID = _env("MIKMONGO_CUSTOMER_ID", "cc7cead3-17e4-4bce-b635-a16ffb4e053e")
USER_ID = _env("MIKMONGO_USER_ID", "5763b510-6053-41e2-a953-17496c57b4f1")
SUBSCRIPTION_ID = _env("MIKMONGO_SUBSCRIPTION_ID", "65e6a6aa-c5b8-4eac-bc39-e5bae7ae2aeb")
INVOICE_ID = _env("MIKMONGO_INVOICE_ID", "e8f1a2b3-c4d5-4e6f-8a7b-12345678abcd")
PAYMENT_ID = _env("MIKMONGO_PAYMENT_ID", "a1f4b4b7-28ca-4bfe-a102-100076ba26df")
PROFILE_ID = _env("MIKMONGO_PROFILE_ID", "8f42e08a-98da-4518-902d-f68281853023")
SETTING_ID = _env("MIKMONGO_SETTING_ID", "")

DESTRUCTIVE = _bool("MIKMONGO_DESTRUCTIVE", False)
VERBOSE = _bool("MIKMONGO_VERBOSE", False)
