from __future__ import annotations

import httpx
import pytest
import respx

from adapters.api.countries.no import BrregAdapter


@pytest.fixture
def adapter() -> BrregAdapter:
    return BrregAdapter()


@respx.mock
async def test_brreg_maps_active_company(adapter: BrregAdapter) -> None:
    payload = {
        "_embedded": {
            "enheter": [
                {
                    "organisasjonsnummer": "123456789",
                    "navn": "Company AS",
                    "hjemmeside": "https://example.no",
                    "konkurs": False,
                    "underAvvikling": False,
                }
            ]
        },
        "page": {"size": 200, "totalElements": 1000000, "totalPages": 5000, "number": 0},
    }
    route = respx.get("https://data.brreg.no/enhetsregisteret/api/enheter").mock(
        return_value=httpx.Response(200, json=payload)
    )

    resp = await adapter.crawl(since=None, cursor=None, page=1)

    assert route.called
    request = route.calls[0].request
    assert request.url.params["page"] == "0"
    assert request.url.params["size"] == "200"
    assert request.headers["Accept"] == "application/json"

    assert resp.total == 1_000_000
    assert resp.has_more is True
    rec = resp.records[0]
    assert rec.name == "Company AS"
    assert rec.country_iso2 == "NO"
    assert rec.registration_number == "123456789"
    assert rec.website == "https://example.no"
    assert rec.status == "active"


@respx.mock
async def test_brreg_status_dissolved_when_bankrupt(adapter: BrregAdapter) -> None:
    payload = {
        "_embedded": {
            "enheter": [
                {
                    "organisasjonsnummer": "111111111",
                    "navn": "Dead AS",
                    "konkurs": True,
                    "underAvvikling": False,
                }
            ]
        },
        "page": {"size": 200, "totalElements": 1, "totalPages": 1, "number": 0},
    }
    respx.get("https://data.brreg.no/enhetsregisteret/api/enheter").mock(
        return_value=httpx.Response(200, json=payload)
    )

    resp = await adapter.crawl(since=None, cursor=None, page=1)

    assert resp.records[0].status == "dissolved"
    assert resp.has_more is False


@respx.mock
async def test_brreg_passes_since_filter(adapter: BrregAdapter) -> None:
    from datetime import datetime, timezone

    captured: dict = {}

    def handler(request: httpx.Request) -> httpx.Response:
        captured["params"] = dict(request.url.params)
        return httpx.Response(
            200,
            json={"_embedded": {"enheter": []}, "page": {"size": 200, "totalElements": 0, "totalPages": 0, "number": 0}},
        )

    respx.get("https://data.brreg.no/enhetsregisteret/api/enheter").mock(side_effect=handler)

    await adapter.crawl(since=datetime(2024, 3, 1, tzinfo=timezone.utc), cursor=None, page=2)

    assert captured["params"]["page"] == "1"  # zero-indexed (page=2 → 1)
    assert captured["params"]["fraRegistreringsdato"] == "2024-03-01"
