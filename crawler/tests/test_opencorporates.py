from __future__ import annotations

import httpx
import pytest
import respx

from adapters.api.opencorporates import OpenCorporatesAdapter


@pytest.fixture(autouse=True)
def _clear_env(monkeypatch: pytest.MonkeyPatch) -> None:
    monkeypatch.delenv("OPENCORPORATES_API_KEY", raising=False)


@respx.mock
async def test_oc_maps_active_record() -> None:
    payload = {
        "results": {
            "companies": [
                {
                    "company": {
                        "name": "EXAMPLE LTD",
                        "company_number": "12345678",
                        "jurisdiction_code": "gb",
                        "current_status": "Active",
                        "inactive": False,
                    }
                }
            ],
            "total_count": 100000,
            "page": 1,
            "per_page": 100,
        }
    }
    route = respx.get("https://api.opencorporates.com/v0.4/companies/search").mock(
        return_value=httpx.Response(200, json=payload)
    )

    adapter = OpenCorporatesAdapter()
    resp = await adapter.crawl(since=None, cursor=None, page=1)

    assert route.called
    params = dict(route.calls[0].request.url.params)
    assert params["q"] == "*"
    assert params["per_page"] == "100"
    assert "api_token" not in params

    rec = resp.records[0]
    assert rec.name == "EXAMPLE LTD"
    assert rec.country_iso2 == "GB"
    assert rec.registration_number == "12345678"
    assert rec.status == "active"
    assert resp.total == 100_000
    assert resp.has_more is True


@respx.mock
async def test_oc_passes_api_token_when_present(monkeypatch: pytest.MonkeyPatch) -> None:
    monkeypatch.setenv("OPENCORPORATES_API_KEY", "secret")
    respx.get("https://api.opencorporates.com/v0.4/companies/search").mock(
        return_value=httpx.Response(
            200,
            json={"results": {"companies": [], "total_count": 0, "page": 1, "per_page": 100}},
        )
    )

    adapter = OpenCorporatesAdapter()
    await adapter.crawl(since=None, cursor=None, page=1)

    request = respx.calls[0].request
    assert request.url.params["api_token"] == "secret"


@respx.mock
async def test_oc_status_mapping() -> None:
    def make(current_status: str, inactive: bool) -> dict:
        return {
            "results": {
                "companies": [
                    {
                        "company": {
                            "name": "x",
                            "company_number": "1",
                            "jurisdiction_code": "no",
                            "current_status": current_status,
                            "inactive": inactive,
                        }
                    }
                ],
                "total_count": 1,
                "page": 1,
                "per_page": 100,
            }
        }

    cases = [
        ("Active", False, "active"),
        ("Dissolved", False, "dissolved"),
        ("Active", True, "inactive"),
    ]
    for current, inactive, want in cases:
        respx.get("https://api.opencorporates.com/v0.4/companies/search").mock(
            return_value=httpx.Response(200, json=make(current, inactive))
        )
        adapter = OpenCorporatesAdapter()
        resp = await adapter.crawl(since=None, cursor=None, page=1)
        assert resp.records[0].status == want, (current, inactive)
        respx.reset()
