from __future__ import annotations

import httpx
import pytest
import respx

from adapters.api.countries.dk import CVRAdapter

_CVR_URL = "https://cvrapi.dk/api"
_CUSTOM_URL = "https://custom.example.com/cvr-api"

FIXTURE_COMPANY = {
    "vat": "12345678",
    "name": "Test Dansk ApS",
    "address": "Testvej 1",
    "zipcode": "1000",
    "city": "Copenhagen",
    "phone": "12345678",
    "email": "test@testdansk.dk",
    "industrydesc": "IT services",
    "startdate": "2018-06-01",
    "enddate": None,
}
API_RESPONSE = [FIXTURE_COMPANY]


@respx.mock
async def test_crawl_with_db_config_returns_records(
    cvr_config: dict,
    monkeypatch: pytest.MonkeyPatch,
) -> None:
    monkeypatch.setenv("CVR_API_TOKEN", "testtoken")
    respx.get(_CVR_URL).mock(return_value=httpx.Response(200, json=API_RESPONSE))

    adapter = CVRAdapter()
    resp = await adapter.crawl(since=None, cursor=None, page=1, config=cvr_config)

    assert len(resp.records) >= 1
    rec = resp.records[0]
    assert rec.country_iso2 == "DK"
    assert rec.name != ""


@respx.mock
async def test_crawl_config_overrides_hardcoded_url(
    cvr_config: dict,
    monkeypatch: pytest.MonkeyPatch,
) -> None:
    monkeypatch.setenv("CVR_API_TOKEN", "testtoken")
    custom_config = {**cvr_config, "api_url": _CUSTOM_URL}

    custom_route = respx.get(_CUSTOM_URL).mock(return_value=httpx.Response(200, json=API_RESPONSE))
    default_route = respx.get(_CVR_URL).mock(return_value=httpx.Response(200, json=API_RESPONSE))

    adapter = CVRAdapter()
    await adapter.crawl(since=None, cursor=None, page=1, config=custom_config)

    assert custom_route.called
    assert not default_route.called
