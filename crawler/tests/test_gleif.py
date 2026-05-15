from __future__ import annotations

import json

import httpx
import pytest
import respx

from adapters.api.gleif import GLEIFAdapter


@pytest.fixture
def adapter() -> GLEIFAdapter:
    return GLEIFAdapter()


@respx.mock
async def test_gleif_maps_active_record(adapter: GLEIFAdapter) -> None:
    payload = {
        "data": [
            {
                "type": "lei-records",
                "id": "GLEIF0000000000001",
                "attributes": {
                    "entity": {
                        "legalName": {"name": "Test Corp Ltd"},
                        "status": "ACTIVE",
                        "legalAddress": {"country": "GB"},
                    },
                    "registration": {"lastUpdateTime": "2024-01-15T12:00:00.000Z"},
                },
            }
        ],
        "meta": {
            "pagination": {
                "total": 2500000,
                "currentPage": 1,
                "lastPage": 12500,
                "nextPage": 2,
                "perPage": 200,
            }
        },
    }
    route = respx.get("https://api.gleif.org/api/v1/lei-records").mock(
        return_value=httpx.Response(200, json=payload)
    )

    resp = await adapter.crawl(since=None, cursor=None, page=1)

    assert route.called
    assert resp.total == 2500000
    assert resp.has_more is True
    assert len(resp.records) == 1
    rec = resp.records[0]
    assert rec.name == "Test Corp Ltd"
    assert rec.country_iso2 == "GB"
    assert rec.lei == "GLEIF0000000000001"
    assert rec.status == "active"
    assert rec.snapshot_hash and len(rec.snapshot_hash) == 64


@respx.mock
async def test_gleif_status_mapping(adapter: GLEIFAdapter) -> None:
    def make(status: str) -> dict:
        return {
            "data": [
                {
                    "type": "lei-records",
                    "id": f"LEI-{status}",
                    "attributes": {
                        "entity": {
                            "legalName": {"name": "x"},
                            "status": status,
                            "legalAddress": {"country": "DE"},
                        },
                        "registration": {"lastUpdateTime": "2024-01-15T12:00:00.000Z"},
                    },
                }
            ],
            "meta": {
                "pagination": {
                    "total": 1,
                    "currentPage": 1,
                    "lastPage": 1,
                    "nextPage": None,
                    "perPage": 200,
                }
            },
        }

    expected = {"ACTIVE": "active", "INACTIVE": "inactive", "ANNULLED": "dissolved", "OTHER": "active"}
    for status, want in expected.items():
        respx.get("https://api.gleif.org/api/v1/lei-records").mock(
            return_value=httpx.Response(200, json=make(status))
        )
        resp = await adapter.crawl(since=None, cursor=None, page=1)
        assert resp.records[0].status == want, status
        respx.reset()


@respx.mock
async def test_gleif_passes_since_filter(adapter: GLEIFAdapter) -> None:
    captured: dict = {}

    def handler(request: httpx.Request) -> httpx.Response:
        captured["params"] = dict(request.url.params)
        return httpx.Response(
            200,
            json={
                "data": [],
                "meta": {
                    "pagination": {
                        "total": 0,
                        "currentPage": 1,
                        "lastPage": 1,
                        "nextPage": None,
                        "perPage": 200,
                    }
                },
            },
        )

    respx.get("https://api.gleif.org/api/v1/lei-records").mock(side_effect=handler)

    from datetime import datetime, timezone

    await adapter.crawl(since=datetime(2024, 1, 1, tzinfo=timezone.utc), cursor=None, page=3)

    assert captured["params"]["page[number]"] == "3"
    assert captured["params"]["page[size]"] == "200"
    assert captured["params"]["filter[lastUpdateTime]"] == "2024-01-01T00:00:00+00:00"


@respx.mock
async def test_gleif_has_more_false_at_last_page(adapter: GLEIFAdapter) -> None:
    respx.get("https://api.gleif.org/api/v1/lei-records").mock(
        return_value=httpx.Response(
            200,
            json={
                "data": [],
                "meta": {
                    "pagination": {
                        "total": 100,
                        "currentPage": 1,
                        "lastPage": 1,
                        "nextPage": None,
                        "perPage": 200,
                    }
                },
            },
        )
    )
    resp = await adapter.crawl(since=None, cursor=None, page=1)
    assert resp.has_more is False
    assert resp.next_cursor is None
