from __future__ import annotations

import httpx
import pytest
import respx

from adapters.api.countries.ee import EstoniaAdapter


@pytest.fixture
def adapter() -> EstoniaAdapter:
    return EstoniaAdapter()


@respx.mock
async def test_estonia_maps_registered(adapter: EstoniaAdapter) -> None:
    payload = {
        "results": [
            {"ariregistri_kood": "12345678", "nimi": "Company OÜ", "aadress": "Tallinn", "staatus": "R"}
        ],
        "total": 100000,
    }
    route = respx.get("https://ariregister.rik.ee/api/1/").mock(return_value=httpx.Response(200, json=payload))

    resp = await adapter.crawl(since=None, cursor=None, page=1)

    assert route.called
    params = dict(route.calls[0].request.url.params)
    assert params["meetod"] == "searchcompany_v2"
    assert params["keel"] == "eng"
    assert params["limit"] == "200"
    assert params["offset"] == "0"

    rec = resp.records[0]
    assert rec.name == "Company OÜ"
    assert rec.country_iso2 == "EE"
    assert rec.registration_number == "12345678"
    assert rec.status == "active"
    assert resp.has_more is True


@respx.mock
async def test_estonia_status_map(adapter: EstoniaAdapter) -> None:
    def make(status: str) -> dict:
        return {"results": [{"ariregistri_kood": "1", "nimi": "x", "staatus": status}], "total": 1}

    expected = {"R": "active", "K": "dissolved", "L": "inactive", "X": "active"}
    for status, want in expected.items():
        respx.get("https://ariregister.rik.ee/api/1/").mock(
            return_value=httpx.Response(200, json=make(status))
        )
        resp = await adapter.crawl(since=None, cursor=None, page=1)
        assert resp.records[0].status == want, status
        respx.reset()


@respx.mock
async def test_estonia_cursor_offset(adapter: EstoniaAdapter) -> None:
    captured: dict = {}

    def handler(request: httpx.Request) -> httpx.Response:
        captured["offset"] = request.url.params["offset"]
        return httpx.Response(200, json={"results": [], "total": 0})

    respx.get("https://ariregister.rik.ee/api/1/").mock(side_effect=handler)

    await adapter.crawl(since=None, cursor="400", page=1)
    assert captured["offset"] == "400"
