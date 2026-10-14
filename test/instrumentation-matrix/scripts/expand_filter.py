#!/usr/bin/env python3
"""Expand the matrix and filter cells by comma-separated inclusion lists.

Reads filter inputs from env vars (avoids shell-quoting hazards when called
from a GitHub Actions step with `${{ inputs.X }}` interpolations). Writes
a single line `cell_ids=<json>` to GITHUB_OUTPUT.

Env vars:
- FILTER_TRACELOOP_VERSIONS, FILTER_FRAMEWORKS, FILTER_FRAMEWORK_VERSIONS,
  FILTER_PYTHON_VERSIONS — each comma-separated, or the literal string "all".

Used by .github/workflows/instrumentation-matrix-manual.yaml.
"""
from __future__ import annotations

import json
import os
import sys
from pathlib import Path

HERE = Path(__file__).resolve().parent.parent
sys.path.insert(0, str(HERE))

from harness.cell_filter import CellFilter  # noqa: E402
from harness.manifest import expand_matrix, load_manifest  # noqa: E402


def main() -> int:
    cells = expand_matrix(load_manifest(HERE / "matrix.yaml"))
    filtered = CellFilter.from_env().apply(cells)

    ids = [c.id for c in filtered]
    payload = json.dumps(ids)

    gh_output = os.environ.get("GITHUB_OUTPUT")
    if gh_output:
        with open(gh_output, "a") as fh:
            fh.write(f"cell_ids={payload}\n")
            fh.write(f"count={len(ids)}\n")
    else:
        # Local dev: print so the operator can sanity-check filter inputs.
        print(payload)
    print(f"resolved {len(ids)} cell(s)", file=sys.stderr)
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
