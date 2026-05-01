"""Payments — list, get, gated create + confirm/reject/refund/initiate-gateway."""
from __future__ import annotations

from datetime import date

import config
from client import ApiClient, Tally, check, info, print_summary, section, skip


def run() -> None:
    api = ApiClient()
    if not api.require_token("payments"):
        return

    section("Payments — read-only")
    check("GET /payments", api.get("/api/v1/payments", params={"limit": 10}))
    check(
        f"GET /payments/{config.PAYMENT_ID}",
        api.get(f"/api/v1/payments/{config.PAYMENT_ID}"),
    )

    if not config.DESTRUCTIVE:
        for label in ("POST /payments", "confirm", "reject", "refund", "initiate-gateway"):
            skip(label, "destructive disabled")
        return

    section("Payments — destructive")
    create = api.post(
        "/api/v1/payments",
        {
            "customer_id": config.CUSTOMER_ID,
            "amount": 100000.0,
            "payment_method": "cash",
            "payment_date": date.today().isoformat(),
            "notes": "created by py-http test",
        },
    )
    if check("POST /payments", create):
        new_id = (create.data or {}).get("id")
        info(f"created payment id={new_id}")
        if new_id:
            check(
                f"POST /payments/{new_id}/confirm",
                api.post(f"/api/v1/payments/{new_id}/confirm"),
            )
            check(
                f"POST /payments/{new_id}/refund",
                api.post(
                    f"/api/v1/payments/{new_id}/refund",
                    {"amount": 100000.0, "reason": "py-http test rollback"},
                ),
            )

    # initiate-gateway uses the seeded payment id (read-only-ish, just initiates a session)
    check(
        f"POST /payments/{config.PAYMENT_ID}/initiate-gateway",
        api.post(f"/api/v1/payments/{config.PAYMENT_ID}/initiate-gateway?gateway=xendit"),
    )


if __name__ == "__main__":
    Tally.reset()
    run()
    raise SystemExit(print_summary())
