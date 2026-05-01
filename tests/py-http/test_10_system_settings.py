"""System Settings — list, get, gated CRUD."""
from __future__ import annotations

import time

import config
from client import ApiClient, Tally, check, info, print_summary, section, skip


def run() -> None:
    api = ApiClient()
    if not api.require_token("settings"):
        return

    section("System Settings — read-only")
    list_res = api.get("/api/v1/settings", params={"limit": 10})
    check("GET /settings", list_res)

    setting_id = config.SETTING_ID
    if not setting_id:
        # try to pick one from the list
        body = list_res.json or {}
        items = body.get("data") if isinstance(body, dict) else None
        if isinstance(items, list) and items:
            setting_id = items[0].get("id", "")
        elif isinstance(items, dict):
            for v in items.values():
                if isinstance(v, list) and v:
                    setting_id = v[0].get("id", "")
                    break
    if setting_id:
        check(f"GET /settings/{setting_id}", api.get(f"/api/v1/settings/{setting_id}"))
    else:
        skip("GET /settings/{id}", "no setting id available")

    if not config.DESTRUCTIVE:
        skip("POST /settings", "destructive disabled")
        skip("PATCH /settings/{id}", "destructive disabled")
        skip("DELETE /settings/{id}", "destructive disabled")
        return

    section("System Settings — destructive")
    suffix = int(time.time())
    create = api.post(
        "/api/v1/settings",
        {
            "group_name": "py_http_test",
            "key_name": f"key_{suffix}",
            "value": "hello",
            "type": "string",
            "label": "PyHttp test key",
            "description": "created by py-http test",
            "is_encrypted": False,
            "is_public": False,
        },
    )
    if check("POST /settings", create):
        new_id = (create.data or {}).get("id")
        info(f"created setting id={new_id}")
        if new_id:
            check(
                f"PATCH /settings/{new_id}",
                api.patch(f"/api/v1/settings/{new_id}", {"value": "world"}),
            )
            check(f"DELETE /settings/{new_id}", api.delete(f"/api/v1/settings/{new_id}"))


if __name__ == "__main__":
    Tally.reset()
    run()
    raise SystemExit(print_summary())
