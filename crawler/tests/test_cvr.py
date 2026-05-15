from __future__ import annotations

import httpx
import pytest
import respx

from adapters.api.countries.dk import CVRAdapter


@pytest.fixture(autouse=True)
def _clear_env(monkeypatch: pytest.MonkeyPatch) -> None:
    monkeypatch.delenv("CVR_API_TOKEN", raising=False)


@respx.mock
async def test_cvr_works_without_token() -> None:
    # Token is optional — unauthenticated requests are allowed with lower rate limits
    payload = [{"vat": 99999999, "name": "Test ApS", "enddate": None, "website": None}]
    route = respx.get("https://cvrapi.dk/api").mock(return_value=httpx.Response(200, json=payload))

    adapter = CVRAdapter()
    resp = await adapter.crawl(since=None, cursor=None, page=1)

    assert route.called
    params = dict(route.calls[0].request.url.params)
    assert "token" not in params
    assert len(resp.records) == 1


@respx.mock
async def test_cvr_maps_active_record(monkeypatch: pytest.MonkeyPatch) -> None:
    monkeypatch.setenv("CVR_API_TOKEN", "tok")
    payload = [
        {
            "vat": 12345678,
            "name": "Company ApS",
            "address": "Main 1",
            "zipcode": "1234",
            "city": "Copenhagen",
            "phone": "+45 123",
            "email": "x@example.dk",
            "website": "https://example.dk",
            "startdate": "2020-01-01",
            "enddate": None,
        }
    ]
    route = respx.get("https://cvrapi.dk/api").mock(return_value=httpx.Response(200, json=payload))

    adapter = CVRAdapter()
    resp = await adapter.crawl(since=None, cursor=None, page=1)

    assert route.called
    params = dict(route.calls[0].request.url.params)
    assert params["country"] == "dk"
    assert params["token"] == "tok"
    assert params["start"] == "0"

    rec = resp.records[0]
    assert rec.name == "Company ApS"
    assert rec.country_iso2 == "DK"
    assert rec.registration_number == "12345678"
    assert rec.website == "https://example.dk"
    assert rec.status == "active"


@respx.mock
async def test_cvr_dissolved_when_enddate_set(monkeypatch: pytest.MonkeyPatch) -> None:
    monkeypatch.setenv("CVR_API_TOKEN", "tok")
    payload = [
        {"vat": 1, "name": "x", "enddate": "2024-01-01", "website": None},
    ]
    respx.get("https://cvrapi.dk/api").mock(return_value=httpx.Response(200, json=payload))

    adapter = CVRAdapter()
    resp = await adapter.crawl(since=None, cursor=None, page=1)
    assert resp.records[0].status == "dissolved"


@respx.mock
async def test_cvr_has_more_when_full_page(monkeypatch: pytest.MonkeyPatch) -> None:
    monkeypatch.setenv("CVR_API_TOKEN", "tok")
    payload = [{"vat": i, "name": f"c{i}"} for i in range(100)]
    respx.get("https://cvrapi.dk/api").mock(return_value=httpx.Response(200, json=payload))

    adapter = CVRAdapter()
    resp = await adapter.crawl(since=None, cursor="200", page=1)

    assert resp.has_more is True
    assert resp.next_cursor == "300"
