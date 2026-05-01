"""Routers — list, selected, get, test-connection, gated CRUD + sync + select."""
from __future__ import annotations

import time

import config
from client import ApiClient, Tally, check, info, print_summary, section, skip


def run() -> None:
    api = ApiClient()
    if not api.require_token("routers"):
        return

    section("Routers — read-only")
    check("GET /routers", api.get("/api/v1/routers", params={"limit": 10}))
    check("GET /routers/selected", api.get("/api/v1/routers/selected"))
    check(
        f"GET /routers/{config.ROUTER_ID}",
        api.get(f"/api/v1/routers/{config.ROUTER_ID}"),
    )
    check(
        f"POST /routers/{config.ROUTER_ID}/test-connection",
        api.post(f"/api/v1/routers/{config.ROUTER_ID}/test-connection"),
    )

    if not config.DESTRUCTIVE:
        skip("POST /routers", "destructive disabled")
        skip("PATCH /routers/{id}", "destructive disabled")
        skip("DELETE /routers/{id}", "destructive disabled")
        skip("POST /routers/{id}/sync", "destructive disabled")
        skip("POST /routers/sync-all", "destructive disabled")
        skip(f"POST /routers/select/{config.ROUTER_ID}", "destructive disabled")
        return

    section("Routers — destructive")
    suffix = int(time.time())
    create = api.post(
        "/api/v1/routers",
        {
            "name": f"py-http-router-{suffix}",
            "address": "192.0.2.1",
            "username": "admin",
            "password": "test",
            "api_port": 8728,
            "rest_port": 80,
            "use_ssl": False,
            "is_master": False,
            "notes": "created by py-http test",
        },
    )
    if check("POST /routers", create):
        new_id = (create.data or {}).get("id")
        info(f"created router id={new_id}")
        if new_id:
            check(
                f"PATCH /routers/{new_id}",
                api.patch(f"/api/v1/routers/{new_id}", {"notes": "updated"}),
            )
            check(f"DELETE /routers/{new_id}", api.delete(f"/api/v1/routers/{new_id}"))

    check(
        f"POST /routers/{config.ROUTER_ID}/sync",
        api.post(f"/api/v1/routers/{config.ROUTER_ID}/sync"),
    )
    check("POST /routers/sync-all", api.post("/api/v1/routers/sync-all"))
    check(
        f"POST /routers/select/{config.ROUTER_ID}",
        api.post(f"/api/v1/routers/select/{config.ROUTER_ID}"),
    )


if __name__ == "__main__":
    Tally.reset()
    run()
    raise SystemExit(print_summary())
