"""Invoices — list, overdue, get, gated trigger-monthly."""
from __future__ import annotations

import config
from client import ApiClient, Tally, check, print_summary, section, skip


def run() -> None:
    api = ApiClient()
    if not api.require_token("invoices"):
        return

    section("Invoices — read-only")
    check("GET /invoices", api.get("/api/v1/invoices", params={"limit": 10}))
    check("GET /invoices/overdue", api.get("/api/v1/invoices/overdue"))
    check(
        f"GET /invoices/{config.INVOICE_ID}",
        api.get(f"/api/v1/invoices/{config.INVOICE_ID}"),
    )

    if not config.DESTRUCTIVE:
        skip("POST /invoices/trigger-monthly", "destructive disabled")
        return

    section("Invoices — destructive")
    check(
        "POST /invoices/trigger-monthly",
        api.post("/api/v1/invoices/trigger-monthly"),
    )


if __name__ == "__main__":
    Tally.reset()
    run()
    raise SystemExit(print_summary())
