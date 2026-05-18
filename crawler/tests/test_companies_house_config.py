from __future__ import annotations

import httpx
import pytest
import respx

from adapters.api.countries.uk import CompaniesHouseAdapter

_CH_URL = "https://api.company-information.service.gov.uk/advanced-search/companies"
_CUSTOM_URL = "https://custom.example.com/ch-api"

FIXTURE_COMPANY = {
    "company_name": "Test Company Ltd",
    "company_number": "12345678",
    "company_status": "active",
    "company_type": "ltd",
    "date_of_creation": "2020-01-15",
    "registered_office_address": {
        "address_line_1": "1 Test Street",
        "locality": "London",
        "postal_code": "EC1A 1BB",
        "country": "England",
    },
    "sic_codes": ["62012"],
}
API_RESPONSE = {"items": [FIXTURE_COMPANY], "hits": 1}


@respx.mock
async def test_crawl_with_db_config_returns_records(
    companies_house_config: dict,
    monkeypatch: pytest.MonkeyPatch,
) -> None:
    monkeypatch.setenv("COMPANIES_HOUSE_API_KEY", "testkey")
    respx.get(_CH_URL).mock(return_value=httpx.Response(200, json=API_RESPONSE))

    adapter = CompaniesHouseAdapter()
    resp = await adapter.crawl(since=None, cursor=None, page=1, config=companies_house_config)

    assert len(resp.records) >= 1
    rec = resp.records[0]
    assert rec.country_iso2 == "GB"
    assert rec.name != ""


@respx.mock
async def test_crawl_config_overrides_hardcoded_url(
    companies_house_config: dict,
    monkeypatch: pytest.MonkeyPatch,
) -> None:
    monkeypatch.setenv("COMPANIES_HOUSE_API_KEY", "testkey")
    custom_config = {**companies_house_config, "api_url": _CUSTOM_URL}

    custom_route = respx.get(_CUSTOM_URL).mock(return_value=httpx.Response(200, json=API_RESPONSE))
    default_route = respx.get(_CH_URL).mock(return_value=httpx.Response(200, json=API_RESPONSE))

    adapter = CompaniesHouseAdapter()
    await adapter.crawl(since=None, cursor=None, page=1, config=custom_config)

    assert custom_route.called
    assert not default_route.called
