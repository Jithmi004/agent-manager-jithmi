#!/usr/bin/env python3
"""List the heavy-tier CI shards (one per Traceloop version) for a run.

Reads the same env the heavy driver does — FILTER_* and HEAVY_CELL_ID, all
optional — and writes `versions=<json>` and `count=<cells>` to
GITHUB_OUTPUT. Each listed version becomes one heavy job, which the driver
narrows to via HEAVY_PROVIDER_VERSION.

Used by the nightly and manual instrumentation-matrix workflows.
"""
from __future__ import annotations

import json
import os
import sys
from pathlib import Path

HERE = Path(__file__).resolve().parent.parent
sys.path.insert(0, str(HERE))

from harness.cell_filter import CellFilter  # noqa: E402
from harness.heavy_subset import heavy_shards, select_heavy_run  # noqa: E402
from harness.manifest import load_manifest  # noqa: E402


def main() -> int:
    cells = select_heavy_run(
        load_manifest(HERE / "matrix.yaml"),
        CellFilter.from_env(),
        os.environ.get("HEAVY_CELL_ID", "").strip(),
    )
    versions = heavy_shards(cells)

    gh_output = os.environ.get("GITHUB_OUTPUT")
    if gh_output:
        with open(gh_output, "a") as fh:
            fh.write(f"versions={json.dumps(versions)}\n")
            fh.write(f"count={len(cells)}\n")
    else:
        print(json.dumps(versions))
    print(f"resolved {len(cells)} heavy cell(s) in {len(versions)} shard(s)", file=sys.stderr)
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
