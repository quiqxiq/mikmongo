"""Users — list, get, gated CRUD."""
from __future__ import annotations

import time

import config
from client import ApiClient, Tally, check, info, print_summary, section, skip


def run() -> None:
    api = ApiClient()
    if not api.require_token("users"):
        return

    section("Users — read-only")
    check("GET /users", api.get("/api/v1/users", params={"limit": 10}))
    check(f"GET /users/{config.USER_ID}", api.get(f"/api/v1/users/{config.USER_ID}"))

    if not config.DESTRUCTIVE:
        skip("POST /users", "destructive disabled")
        skip("PATCH /users/{id}", "destructive disabled")
        skip("DELETE /users/{id}", "destructive disabled")
        return

    section("Users — destructive")
    suffix = int(time.time())
    new_email = f"py-http-test-{suffix}@mikmongo.local"
    create = api.post(
        "/api/v1/users",
        {
            "full_name": f"PyHttp Test {suffix}",
            "email": new_email,
            "phone": "+62000000000",
            "role": "customer",
            "password": "ChangeMe123!",
        },
    )
    if check("POST /users", create):
        new_id = (create.data or {}).get("id")
        info(f"created user id={new_id}")
        if new_id:
            check(
                f"PATCH /users/{new_id}",
                api.patch(f"/api/v1/users/{new_id}", {"full_name": f"PyHttp Test {suffix} (renamed)"}),
            )
            check(f"DELETE /users/{new_id}", api.delete(f"/api/v1/users/{new_id}"))


if __name__ == "__main__":
    Tally.reset()
    run()
    raise SystemExit(print_summary())
