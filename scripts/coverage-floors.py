#!/usr/bin/env python3
"""Per-package Go coverage floors.

Reads a Go coverage profile and fails if any package listed in the floors
file is below its floor. Floors only ratchet up: run with --suggest to print
the current values and raise the floors file in the same PR that adds tests.
"""
import argparse
import json
import sys
from collections import defaultdict


def package_coverage(profile_path, module):
    blocks = {}
    with open(profile_path, encoding="utf-8") as f:
        for line in f:
            line = line.strip()
            if not line or line.startswith("mode:"):
                continue
            try:
                loc, statements, count = line.rsplit(" ", 2)
                statements, count = int(statements), int(count)
            except ValueError:
                continue
            # The same block can appear once per test binary; keep the max hit count.
            prev = blocks.get(loc)
            blocks[loc] = (statements, max(count, prev[1] if prev else 0))

    totals = defaultdict(lambda: [0, 0])
    for loc, (statements, count) in blocks.items():
        path = loc.split(":", 1)[0]
        pkg = path.rsplit("/", 1)[0]
        if module and pkg.startswith(module + "/"):
            pkg = pkg[len(module) + 1:]
        totals[pkg][1] += statements
        if count > 0:
            totals[pkg][0] += statements
    return {pkg: (100.0 * c / t if t else 100.0) for pkg, (c, t) in totals.items()}


def main():
    ap = argparse.ArgumentParser(description=__doc__)
    ap.add_argument("--coverage", default="coverage.out")
    ap.add_argument("--floors", default="quality/coverage-floors.json")
    ap.add_argument("--suggest", action="store_true", help="print current coverage for every package and exit 0")
    args = ap.parse_args()

    with open(args.floors, encoding="utf-8") as f:
        cfg = json.load(f)
    module = cfg.get("module", "")
    floors = cfg["floors"]
    targets = cfg.get("targets", {})

    cov = package_coverage(args.coverage, module)

    if args.suggest:
        for pkg in sorted(cov):
            print(f"{pkg:40s} {cov[pkg]:6.1f}%")
        return 0

    failures = []
    print(f"{'package':32s} {'coverage':>9s} {'floor':>7s} {'target':>7s}")
    for pkg in sorted(floors):
        floor = float(floors[pkg])
        got = cov.get(pkg)
        target = targets.get(pkg)
        tgt = f"{target:6.1f}%" if target is not None else "      -"
        if got is None:
            failures.append(f"{pkg}: no coverage data (package missing from profile)")
            print(f"{pkg:32s} {'missing':>9s} {floor:6.1f}% {tgt}")
            continue
        status = "ok" if got + 1e-9 >= floor else "BELOW FLOOR"
        print(f"{pkg:32s} {got:8.1f}% {floor:6.1f}% {tgt}  {status}")
        if status != "ok":
            failures.append(f"{pkg}: {got:.1f}% < floor {floor:.1f}%")
        elif got - floor >= 2.0:
            print(f"  ↑ {pkg} is {got - floor:.1f} points above its floor; consider raising it")

    if failures:
        print("\nCoverage floors failed:")
        for msg in failures:
            print(f"  - {msg}")
        return 1
    print("\nCoverage floors passed.")
    return 0


if __name__ == "__main__":
    sys.exit(main())
