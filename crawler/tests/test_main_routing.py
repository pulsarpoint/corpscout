from __future__ import annotations

import httpx
import pytest
import respx
from fastapi.testclient import TestClient

import main
from adapters import registry
from adapters.base import CompanyRecord, CrawlResponse, SourceAdapter, compute_hash


class _EchoAdapter(SourceAdapter):
    source_name = "echo"

    async def crawl(self, since, cursor, page, config=None):  # type: ignore[override]
        rec = CompanyRecord(
            name="Echo",
            country_iso2="GB",
            raw_data={"page": page, "cursor": cursor},
            snapshot_hash=compute_hash({"page": page, "cursor": cursor}),
        )
        return CrawlResponse(records=[rec], has_more=False, total=1, next_cursor=None)


@pytest.fixture
def client(monkeypatch: pytest.MonkeyPatch) -> TestClient:
    monkeypatch.delenv("COMPANIES_HOUSE_API_KEY", raising=False)
    monkeypatch.delenv("CVR_API_TOKEN", raising=False)
    monkeypatch.delenv("CRAWLER_OPENAI_API_KEY", raising=False)
    monkeypatch.delenv("CRAWLER_OPENCORPORATES_API_KEY", raising=False)
    return TestClient(main.app)


def test_lists_all_default_sources(client: TestClient) -> None:
    resp = client.get("/sources")
    assert resp.status_code == 200
    sources = resp.json()["sources"]
    for name in [
        "ariregister",
        "brreg",
        "companies_house",
        "crawl4ai_generic",
        "cvr",
        "gleif",
        "opencorporates",
        "wikidata",
    ]:
        assert name in sources


def test_unknown_source_returns_404(client: TestClient) -> None:
    resp = client.post("/crawl/no-such-source", json={"page": 1})
    assert resp.status_code == 404
    assert "not found" in resp.json()["detail"]


def test_crawl4ai_generic_returns_501_when_unconfigured(client: TestClient) -> None:
    resp = client.post("/crawl/crawl4ai_generic", json={"page": 1})
    assert resp.status_code == 501


def test_companies_house_returns_error_when_unconfigured(client: TestClient) -> None:
    resp = client.post("/crawl/companies_house", json={"page": 1})
    assert resp.status_code == 500


def test_registered_adapter_is_dispatched(client: TestClient) -> None:
    registry.register(_EchoAdapter())
    try:
        resp = client.post("/crawl/echo", json={"page": 7, "cursor": "abc"})
        assert resp.status_code == 200
        body = resp.json()
        assert body["records"][0]["raw_data"] == {"page": 7, "cursor": "abc"}
    finally:
        registry.reset()
        main.register_default_adapters()


@respx.mock
def test_resolve_domain_calls_resolver(client: TestClient) -> None:
    respx.get("https://query.wikidata.org/sparql").mock(
        return_value=httpx.Response(
            200,
            json={
                "results": {
                    "bindings": [
                        {
                            "company": {"type": "uri", "value": "http://www.wikidata.org/entity/Q1"},
                            "website": {"type": "uri", "value": "https://example.com"},
                        }
                    ]
                }
            },
        )
    )

    resp = client.post(
        "/resolve/domain",
        json={"company_name": "Acme Inc", "country": "US"},
    )
    assert resp.status_code == 200
    body = resp.json()
    assert body["candidates"][0]["domain"] == "example.com"
    assert body["candidates"][0]["signal"] == "wikidata"
