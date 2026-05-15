from __future__ import annotations

import httpx
import pytest
import respx

from adapters.api.wikidata import WikidataAdapter


@pytest.fixture
def adapter() -> WikidataAdapter:
    return WikidataAdapter()


def _binding(value: str, kind: str = "literal") -> dict:
    return {"type": kind, "value": value}


@respx.mock
async def test_wikidata_basic_mapping(adapter: WikidataAdapter) -> None:
    response = {
        "results": {
            "bindings": [
                {
                    "company": _binding("http://www.wikidata.org/entity/Q12345", "uri"),
                    "companyLabel": _binding("Acme Inc"),
                    "website": _binding("https://acme.example.com", "uri"),
                    "countryCode": _binding("US"),
                },
                {
                    "company": _binding("http://www.wikidata.org/entity/Q67890", "uri"),
                    "companyLabel": _binding("Beta Ltd"),
                },
            ]
        }
    }
    route = respx.get("https://query.wikidata.org/sparql").mock(
        return_value=httpx.Response(200, json=response)
    )

    resp = await adapter.crawl(since=None, cursor=None, page=1)

    assert route.called
    request = route.calls[0].request
    assert request.headers["User-Agent"].startswith("corpscout/")
    assert "format=json" in str(request.url)

    assert len(resp.records) == 2
    first, second = resp.records
    assert first.name == "Acme Inc"
    assert first.website == "https://acme.example.com"
    assert first.country_iso2 == "US"
    assert first.raw_data["qid"] == "Q12345"
    assert second.name == "Beta Ltd"
    assert second.website is None
    assert second.country_iso2 == ""


@respx.mock
async def test_wikidata_cursor_paging_uses_offset(adapter: WikidataAdapter) -> None:
    captured: dict = {}

    def handler(request: httpx.Request) -> httpx.Response:
        captured["query"] = request.url.params["query"]
        return httpx.Response(200, json={"results": {"bindings": []}})

    respx.get("https://query.wikidata.org/sparql").mock(side_effect=handler)

    await adapter.crawl(since=None, cursor="500", page=1)

    assert "OFFSET 500" in captured["query"]


@respx.mock
async def test_wikidata_has_more_when_full_page(adapter: WikidataAdapter) -> None:
    # WikidataAdapter requests LIMIT+1 to detect tail.
    page_size = WikidataAdapter.page_size
    bindings = [
        {
            "company": _binding(f"http://www.wikidata.org/entity/Q{i}", "uri"),
            "companyLabel": _binding(f"Co{i}"),
        }
        for i in range(page_size + 1)
    ]
    respx.get("https://query.wikidata.org/sparql").mock(
        return_value=httpx.Response(200, json={"results": {"bindings": bindings}})
    )

    resp = await adapter.crawl(since=None, cursor=None, page=1)

    assert resp.has_more is True
    assert len(resp.records) == page_size
    assert resp.next_cursor == str(page_size)
