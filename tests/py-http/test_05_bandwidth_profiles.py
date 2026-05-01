"""Bandwidth Profiles — router-scoped list, get, gated CRUD."""
from __future__ import annotations

import time

import config
from client import ApiClient, Tally, check, info, print_summary, section, skip


def run() -> None:
    api = ApiClient()
    if not api.require_token("bandwidth profiles"):
        return

    base = f"/api/v1/routers/{config.ROUTER_ID}/bandwidth-profiles"

    section("Bandwidth Profiles — read-only")
    check(f"GET {base}", api.get(base, params={"limit": 10}))
    check(f"GET {base}/{config.PROFILE_ID}", api.get(f"{base}/{config.PROFILE_ID}"))

    if not config.DESTRUCTIVE:
        skip(f"POST {base}", "destructive disabled")
        skip(f"PATCH {base}/{{id}}", "destructive disabled")
        skip(f"DELETE {base}/{{id}}", "destructive disabled")
        return

    section("Bandwidth Profiles — destructive")
    suffix = int(time.time())
    create = api.post(
        base,
        {
            "profile_code": f"PY{suffix}",
            "name": f"PyHttp Profile {suffix}",
            "description": "created by py-http test",
            "download_speed": 10_000_000,
            "upload_speed": 5_000_000,
            "price_monthly": 100000.0,
            "billing_cycle": "monthly",
            "billing_day": 1,
            "grace_period_days": 3,
            "is_visible": True,
        },
    )
    if check(f"POST {base}", create):
        new_id = (create.data or {}).get("id")
        info(f"created bandwidth profile id={new_id}")
        if new_id:
            check(
                f"PATCH {base}/{new_id}",
                api.patch(f"{base}/{new_id}", {"description": "updated"}),
            )
            check(f"DELETE {base}/{new_id}", api.delete(f"{base}/{new_id}"))


if __name__ == "__main__":
    Tally.reset()
    run()
    raise SystemExit(print_summary())
