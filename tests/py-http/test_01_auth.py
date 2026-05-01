"""Auth endpoints — login, refresh, me, change-password*, logout*.

Asterisked tests are gated behind MIKMONGO_DESTRUCTIVE=1.
This module also writes the JWT to .auth_cache.json so subsequent test files
can pick it up via client.get_token().
"""
from __future__ import annotations

import config
from client import (
    ApiClient,
    Tally,
    check,
    info,
    print_summary,
    save_cached_token,
    section,
    skip,
    warn,
)


def run() -> None:
    section("Auth — login + token cache")
    api = ApiClient(token="")  # start without auth so /auth/login is unauthenticated

    if not config.PASSWORD:
        warn("MIKMONGO_PASSWORD not set in .env — login will be skipped")
        skip("POST /auth/login", "no password configured")
    else:
        login_res = api.post(
            "/api/v1/auth/login",
            {"email": config.EMAIL, "password": config.PASSWORD},
        )
        if check("POST /auth/login", login_res):
            data = login_res.data or {}
            access = data.get("access_token", "")
            refresh = data.get("refresh_token", "")
            if access:
                api.set_token(access)
                save_cached_token(access, refresh)
                info(f"token cached → {config.AUTH_CACHE_FILE.name}")
            else:
                warn("login succeeded but access_token missing in response")

    section("Auth — authenticated calls")
    if not api.require_token("auth checks"):
        return

    check("GET /auth/me", api.get("/api/v1/auth/me"))

    # refresh requires the refresh_token from the cache
    cached_refresh = ""
    if config.AUTH_CACHE_FILE.exists():
        import json as _json
        try:
            cached_refresh = _json.loads(config.AUTH_CACHE_FILE.read_text()).get("refresh_token", "")
        except Exception:
            cached_refresh = ""
    if cached_refresh:
        check(
            "POST /auth/refresh",
            api.post("/api/v1/auth/refresh", {"refresh_token": cached_refresh}),
        )
    else:
        skip("POST /auth/refresh", "no refresh token cached")

    if config.DESTRUCTIVE:
        section("Auth — destructive")
        # change-password is destructive: revert by setting same password
        check(
            "POST /auth/change-password",
            api.post(
                "/api/v1/auth/change-password",
                {"old_password": config.PASSWORD, "new_password": config.PASSWORD},
            ),
        )
        check("POST /auth/logout", api.post("/api/v1/auth/logout"))
    else:
        skip("POST /auth/change-password", "destructive disabled")
        skip("POST /auth/logout", "destructive disabled")


if __name__ == "__main__":
    Tally.reset()
    run()
    raise SystemExit(print_summary())
