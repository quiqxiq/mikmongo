"""Reports — summary, subscriptions, cash-flow, cash-balance, reconciliation.

All endpoints are read-only; nothing is gated.
"""
from __future__ import annotations

from datetime import date, timedelta

from client import ApiClient, Tally, check, print_summary, section


def run() -> None:
    api = ApiClient()
    if not api.require_token("reports"):
        return

    today = date.today()
    start = (today - timedelta(days=30)).isoformat()
    end = today.isoformat()
    range_params = {"start_date": start, "end_date": end}

    section("Reports")
    check("GET /reports/summary", api.get("/api/v1/reports/summary"))
    check(
        "GET /reports/subscriptions",
        api.get("/api/v1/reports/subscriptions", params=range_params),
    )
    check(
        "GET /reports/cash-flow",
        api.get("/api/v1/reports/cash-flow", params=range_params),
    )
    check("GET /reports/cash-balance", api.get("/api/v1/reports/cash-balance"))
    check(
        "GET /reports/reconciliation",
        api.get("/api/v1/reports/reconciliation", params=range_params),
    )


if __name__ == "__main__":
    Tally.reset()
    run()
    raise SystemExit(print_summary())
