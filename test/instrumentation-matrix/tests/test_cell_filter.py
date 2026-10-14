from harness.cell_filter import CellFilter, parse_filter
from harness.manifest import Cell


def _cell(tl="0.61.0", fw="langchain", fwv="0.3.27", py="3.11") -> Cell:
    return Cell(
        provider_name="traceloop",
        provider_version=tl,
        framework_name=fw,
        framework_version=fwv,
        framework_package=fw,
        sample_path=f"cells/{fw}_sample.py",
        span_kinds=["llm"],
        python=py,
        contract_schema="v1",
    )


def test_parse_filter_all_or_blank_matches_everything():
    assert parse_filter("all") is None
    assert parse_filter("") is None
    assert parse_filter(" , ") is None
    # `all` anywhere means the union, not a restriction to the other tokens.
    assert parse_filter("all,langchain") is None


def test_parse_filter_splits_and_strips():
    assert parse_filter(" 0.61.0 ,0.62.4") == {"0.61.0", "0.62.4"}


def test_from_env_defaults_to_match_everything():
    f = CellFilter.from_env({})
    assert f == CellFilter()
    assert f.matches(_cell())


def test_from_env_reads_every_axis():
    f = CellFilter.from_env(
        {
            "FILTER_TRACELOOP_VERSIONS": "0.62.4",
            "FILTER_FRAMEWORKS": "crewai",
            "FILTER_FRAMEWORK_VERSIONS": "1.1.0",
            "FILTER_PYTHON_VERSIONS": "3.12",
        }
    )
    assert f.matches(_cell("0.62.4", "crewai", "1.1.0", "3.12"))
    assert not f.matches(_cell("0.62.3", "crewai", "1.1.0", "3.12"))
    assert not f.matches(_cell("0.62.4", "langchain", "1.1.0", "3.12"))
    assert not f.matches(_cell("0.62.4", "crewai", "1.2.0", "3.12"))
    assert not f.matches(_cell("0.62.4", "crewai", "1.1.0", "3.11"))
