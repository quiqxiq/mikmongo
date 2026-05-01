"""Subscriptions — router-scoped list, get, gated CRUD + lifecycle.

Lifecycle endpoints: activate, isolate, restore, suspend, terminate.
"""
from __future__ import annotations

import time

import config
from client import ApiClient, Tally, check, info, print_summary, section, skip


def run() -> None:
    api = ApiClient()
    if not api.require_token("subscriptions"):
        return

    base = f"/api/v1/routers/{config.ROUTER_ID}/subscriptions"

    section("Subscriptions — read-only")
    check(f"GET {base}", api.get(base, params={"limit": 10}))
    check(
        f"GET {base}/{config.SUBSCRIPTION_ID}",
        api.get(f"{base}/{config.SUBSCRIPTION_ID}"),
    )

    if not config.DESTRUCTIVE:
        for action in ("POST create", "PATCH update", "DELETE", "activate", "isolate", "restore", "suspend", "terminate"):
            skip(f"{action}", "destructive disabled")
        return

    section("Subscriptions — destructive")
    suffix = int(time.time())
    create = api.post(
        base,
        {
            "customer_id": config.CUSTOMER_ID,
            "plan_id": config.PROFILE_ID,
            "username": f"pyhttp{suffix}",
            "password": "ChangeMe123!",
            "auto_isolate": True,
        },
    )
    if check(f"POST {base}", create):
        new_id = (create.data or {}).get("id")
        info(f"created subscription id={new_id}")
        if new_id:
            check(
                f"PATCH {base}/{new_id}",
                api.patch(f"{base}/{new_id}", {"notes": "updated"}),
            )
            for action in ("activate", "isolate", "restore", "suspend"):
                check(
                    f"POST {base}/{new_id}/{action}",
                    api.post(f"{base}/{new_id}/{action}"),
                )
            check(
                f"POST {base}/{new_id}/terminate",
                api.post(f"{base}/{new_id}/terminate"),
            )
            check(f"DELETE {base}/{new_id}", api.delete(f"{base}/{new_id}"))


if __name__ == "__main__":
    Tally.reset()
    run()
    raise SystemExit(print_summary())
