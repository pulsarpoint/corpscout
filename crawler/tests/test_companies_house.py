from __future__ import annotations

import base64

import httpx
import pytest
import respx

from adapters.api.countries.uk import CompaniesHouseAdapter


@pytest.fixture(autouse=True)
def _clear_env(monkeypatch: pytest.MonkeyPatch) -> None:
    monkeypatch.delenv("COMPANIES_HOUSE_API_KEY", raising=False)


async def test_companies_house_raises_when_unconfigured() -> None:
    adapter = CompaniesHouseAdapter()
    with pytest.raises(RuntimeError, match="COMPANIES_HOUSE_API_KEY"):
        await adapter.crawl(since=None, cursor=None, page=1)


@respx.mock
async def test_companies_house_uses_basic_auth(monkeypatch: pytest.MonkeyPatch) -> None:
    monkeypatch.setenv("COMPANIES_HOUSE_API_KEY", "abc123")
    payload = {
        "items": [
            {
                "company_number": "12345678",
                "company_name": "EXAMPLE LTD",
                "company_status": "active",
                "company_type": "ltd",
            }
        ],
        "total_results": 5_000_000,
        "start_index": 0,
        "items_per_page": 100,
    }
    route = respx.get(
        "https://api.company-information.service.gov.uk/advanced-search/companies"
    ).mock(return_value=httpx.Response(200, json=payload))

    adapter = CompaniesHouseAdapter()
    resp = await adapter.crawl(since=None, cursor=None, page=1)

    assert route.called
    auth_header = route.calls[0].request.headers["Authorization"]
    decoded = base64.b64decode(auth_header.removeprefix("Basic ")).decode()
    assert decoded == "abc123:"

    assert resp.total == 5_000_000
    assert resp.has_more is True
    rec = resp.records[0]
    assert rec.name == "EXAMPLE LTD"
    assert rec.country_iso2 == "GB"
    assert rec.registration_number == "12345678"
    assert rec.status == "active"


@respx.mock
async def test_companies_house_status_mapping(monkeypatch: pytest.MonkeyPatch) -> None:
    monkeypatch.setenv("COMPANIES_HOUSE_API_KEY", "key")

    def make(status: str) -> dict:
        return {
            "items": [
                {
                    "company_number": "1",
                    "company_name": "x",
                    "company_status": status,
                    "company_type": "ltd",
                }
            ],
            "total_results": 1,
            "start_index": 0,
            "items_per_page": 100,
        }

    expected = {
        "active": "active",
        "dissolved": "dissolved",
        "liquidation": "inactive",
        "receivership": "inactive",
        "anything-else": "active",
    }
    for status, want in expected.items():
        respx.get(
            "https://api.company-information.service.gov.uk/advanced-search/companies"
        ).mock(return_value=httpx.Response(200, json=make(status)))
        adapter = CompaniesHouseAdapter()
        resp = await adapter.crawl(since=None, cursor=None, page=1)
        assert resp.records[0].status == want, status
        respx.reset()


@respx.mock
async def test_companies_house_paging(monkeypatch: pytest.MonkeyPatch) -> None:
    monkeypatch.setenv("COMPANIES_HOUSE_API_KEY", "key")
    captured: dict = {}

    def handler(request: httpx.Request) -> httpx.Response:
        captured["params"] = dict(request.url.params)
        return httpx.Response(
            200,
            json={
                "items": [{"company_number": "1", "company_name": "x", "company_status": "active", "company_type": "ltd"}],
                "total_results": 250,
                "start_index": 200,
                "items_per_page": 100,
            },
        )

    respx.get("https://api.company-information.service.gov.uk/advanced-search/companies").mock(side_effect=handler)

    adapter = CompaniesHouseAdapter()
    resp = await adapter.crawl(since=None, cursor=None, page=3)

    assert captured["params"]["start_index"] == "200"
    assert captured["params"]["size"] == "100"
    # 200 + 1 < 250 -> has_more
    assert resp.has_more is True
    assert resp.next_cursor == "4"


@respx.mock
async def test_companies_house_cursor_overrides_page(monkeypatch: pytest.MonkeyPatch) -> None:
    monkeypatch.setenv("COMPANIES_HOUSE_API_KEY", "key")
    captured: dict = {}

    def handler(request: httpx.Request) -> httpx.Response:
        captured["params"] = dict(request.url.params)
        return httpx.Response(
            200,
            json={"items": [], "total_results": 0, "start_index": 400, "items_per_page": 100},
        )

    respx.get("https://api.company-information.service.gov.uk/advanced-search/companies").mock(side_effect=handler)

    adapter = CompaniesHouseAdapter()
    await adapter.crawl(since=None, cursor="5", page=1)

    assert captured["params"]["start_index"] == "400"  # cursor=5 → page 5 → start_index=400
