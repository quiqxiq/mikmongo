"""Customers — list, get, gated CRUD + activate/deactivate."""
from __future__ import annotations

import time

import config
from client import ApiClient, Tally, check, info, print_summary, section, skip


def run() -> None:
    api = ApiClient()
    if not api.require_token("customers"):
        return

    section("Customers — read-only")
    check("GET /customers", api.get("/api/v1/customers", params={"limit": 10}))
    check(
        f"GET /customers/{config.CUSTOMER_ID}",
        api.get(f"/api/v1/customers/{config.CUSTOMER_ID}"),
    )

    if not config.DESTRUCTIVE:
        skip("POST /customers", "destructive disabled")
        skip("PATCH /customers/{id}", "destructive disabled")
        skip("DELETE /customers/{id}", "destructive disabled")
        skip("activate-account", "destructive disabled")
        skip("deactivate-account", "destructive disabled")
        return

    section("Customers — destructive")
    suffix = int(time.time())
    create = api.post(
        "/api/v1/customers",
        {
            "full_name": f"PyHttp Customer {suffix}",
            "phone": f"+628{suffix % 10**10:010d}",
            "email": f"py-cust-{suffix}@mikmongo.local",
            "address": "Jl. Test 123",
        },
    )
    if check("POST /customers", create):
        new_id = (create.data or {}).get("id")
        info(f"created customer id={new_id}")
        if new_id:
            check(
                f"PATCH /customers/{new_id}",
                api.patch(f"/api/v1/customers/{new_id}", {"notes": "updated by py-http test"}),
            )
            check(
                f"POST /customers/{new_id}/deactivate-account",
                api.post(f"/api/v1/customers/{new_id}/deactivate-account"),
            )
            check(
                f"POST /customers/{new_id}/activate-account",
                api.post(f"/api/v1/customers/{new_id}/activate-account"),
            )
            check(f"DELETE /customers/{new_id}", api.delete(f"/api/v1/customers/{new_id}"))


if __name__ == "__main__":
    Tally.reset()
    run()
    raise SystemExit(print_summary())
