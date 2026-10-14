"""Inclusion filters over matrix cells, shared by both tiers.

The manual workflow's `traceloop_versions` / `frameworks` /
`framework_versions` / `python_versions` inputs arrive as FILTER_* env vars.
`scripts/expand_filter.py` applies them to the emission matrix, and the heavy
driver and `scripts/resolve_heavy_shards.py` apply them to the heavy subset,
so one input narrows both tiers the same way.
"""
from __future__ import annotations

import os
from dataclasses import dataclass
from typing import Mapping, Optional

from harness.manifest import Cell


def parse_filter(value: str) -> Optional[set[str]]:
    """Parse a comma-separated filter into a set, or return None for
    match-everything.

    Treats empty input and any token equal to `all` (case-insensitive) as
    "match everything" — that way an operator who types `all,langchain`
    intending the union doesn't accidentally restrict to just `langchain`,
    and a blank input doesn't silently match nothing.
    """
    tokens = {v.strip() for v in value.split(",") if v.strip()}
    if not tokens or any(t.lower() == "all" for t in tokens):
        return None
    return tokens


@dataclass(frozen=True)
class CellFilter:
    traceloop_versions: Optional[set[str]] = None
    frameworks: Optional[set[str]] = None
    framework_versions: Optional[set[str]] = None
    python_versions: Optional[set[str]] = None

    @classmethod
    def from_env(cls, env: Mapping[str, str] = os.environ) -> "CellFilter":
        return cls(
            traceloop_versions=parse_filter(env.get("FILTER_TRACELOOP_VERSIONS", "all")),
            frameworks=parse_filter(env.get("FILTER_FRAMEWORKS", "all")),
            framework_versions=parse_filter(env.get("FILTER_FRAMEWORK_VERSIONS", "all")),
            python_versions=parse_filter(env.get("FILTER_PYTHON_VERSIONS", "all")),
        )

    def matches(self, c: Cell) -> bool:
        return (
            (self.traceloop_versions is None or c.provider_version in self.traceloop_versions)
            and (self.frameworks is None or c.framework_name in self.frameworks)
            and (self.framework_versions is None or c.framework_version in self.framework_versions)
            and (self.python_versions is None or c.python in self.python_versions)
        )

    def apply(self, cells: list[Cell]) -> list[Cell]:
        return [c for c in cells if self.matches(c)]
