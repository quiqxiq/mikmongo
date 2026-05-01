"""Run every test_*.py in tests/py-http in alphabetical order.

Each test module exposes a top-level `run()` function. We import the module,
call run(), and tally results via client.Tally. Exits non-zero if any check
failed so this can be wired into CI.

Usage:
    python run_all.py
    MIKMONGO_DESTRUCTIVE=1 python run_all.py
"""
from __future__ import annotations

import importlib
import sys
from pathlib import Path

import client
from client import Tally, print_summary, BLUE, RESET, DIM


def discover() -> list[str]:
    here = Path(__file__).parent
    files = sorted(p.stem for p in here.glob("test_*.py"))
    return files


def main() -> int:
    sys.path.insert(0, str(Path(__file__).parent))
    Tally.reset()
    modules = discover()
    if not modules:
        print("no test_*.py files found")
        return 1

    print(f"{BLUE}Running {len(modules)} test modules{RESET}")
    print(f"{DIM}destructive={client.config.DESTRUCTIVE} verbose={client.config.VERBOSE}{RESET}")

    for name in modules:
        try:
            mod = importlib.import_module(name)
        except Exception as exc:
            Tally.failed += 1
            Tally.failures.append(f"{name}: import error: {exc}")
            print(f"\n[IMPORT FAIL] {name}: {exc}")
            continue
        run = getattr(mod, "run", None)
        if not callable(run):
            print(f"\n[skip] {name}: no run() function")
            continue
        try:
            run()
        except SystemExit:
            raise
        except Exception as exc:
            Tally.failed += 1
            Tally.failures.append(f"{name}: uncaught exception: {exc}")
            print(f"  [UNCAUGHT] {exc}")

    return print_summary()


if __name__ == "__main__":
    sys.exit(main())
