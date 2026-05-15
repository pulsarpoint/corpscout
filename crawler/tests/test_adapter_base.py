from __future__ import annotations

import pytest

from adapters import registry
from adapters.base import (
    CompanyRecord,
    CrawlResponse,
    SourceAdapter,
    compute_hash,
)


def test_compute_hash_is_stable_for_equivalent_dicts() -> None:
    a = {"name": "Acme", "country": "GB", "extra": [1, 2, 3]}
    b = {"extra": [1, 2, 3], "country": "GB", "name": "Acme"}
    assert compute_hash(a) == compute_hash(b)


def test_compute_hash_changes_when_payload_changes() -> None:
    base = {"name": "Acme", "country": "GB"}
    other = {"name": "Acme", "country": "FR"}
    assert compute_hash(base) != compute_hash(other)


def test_compute_hash_handles_unicode() -> None:
    payload = {"name": "Æther OÜ", "country": "EE"}
    h = compute_hash(payload)
    assert isinstance(h, str)
    assert len(h) == 64


class _DummyAdapter(SourceAdapter):
    source_name = "dummy"

    async def crawl(self, since, cursor, page):  # type: ignore[override]
        rec = CompanyRecord(
            name="Acme",
            country_iso2="GB",
            raw_data={"x": 1},
            snapshot_hash=compute_hash({"x": 1}),
        )
        return CrawlResponse(records=[rec], has_more=False, total=1, next_cursor=None)


def test_source_adapter_requires_source_name() -> None:
    class _NoName(SourceAdapter):  # type: ignore[misc]
        async def crawl(self, since, cursor, page):  # type: ignore[override]
            raise NotImplementedError

    with pytest.raises(TypeError):
        _NoName()  # type: ignore[abstract]


async def test_registry_register_and_lookup() -> None:
    reg = registry.AdapterRegistry()
    adapter = _DummyAdapter()
    reg.register(adapter)
    assert reg.get("dummy") is adapter
    assert reg.get("missing") is None
    assert reg.list_sources() == ["dummy"]


async def test_registry_rejects_duplicate_source_name() -> None:
    reg = registry.AdapterRegistry()
    reg.register(_DummyAdapter())
    with pytest.raises(ValueError):
        reg.register(_DummyAdapter())


async def test_module_level_registry_get_returns_none_for_unknown() -> None:
    assert registry.get_adapter("definitely-not-a-source") is None
