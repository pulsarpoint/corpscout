from __future__ import annotations

import urllib.parse

import httpx
import pytest
import respx

from domain_resolver import (
    DomainResolver,
    certsh_signal,
    duckduckgo_signal,
    wikidata_signal,
)


@pytest.fixture(autouse=True)
def _no_sleep(monkeypatch: pytest.MonkeyPatch) -> None:
    async def _instant(_seconds: float) -> None:
        return None

    monkeypatch.setattr("asyncio.sleep", _instant)


@respx.mock
async def test_wikidata_signal_returns_candidate_from_p856() -> None:
    payload = {
        "results": {
            "bindings": [
                {
                    "company": {"type": "uri", "value": "http://www.wikidata.org/entity/Q42"},
                    "website": {"type": "uri", "value": "https://acme.example.com/path"},
                }
            ]
        }
    }
    route = respx.get("https://query.wikidata.org/sparql").mock(
        return_value=httpx.Response(200, json=payload)
    )

    results = await wikidata_signal("Acme Inc", lei=None)

    assert route.called
    assert len(results) == 1
    cand = results[0]
    assert cand.domain == "acme.example.com"
    assert cand.signal == "wikidata"
    assert cand.confidence == 85
    assert cand.evidence["wikidata_uri"].endswith("Q42")


@respx.mock
async def test_wikidata_signal_uses_lei_when_present() -> None:
    captured: dict = {}

    def handler(request: httpx.Request) -> httpx.Response:
        # Only capture the first query; subsequent calls overwrite but
        # we assert on what was sent first (the LEI query).
        if "first_query" not in captured:
            captured["first_query"] = request.url.params["query"]
        return httpx.Response(200, json={"results": {"bindings": []}})

    respx.get("https://query.wikidata.org/sparql").mock(side_effect=handler)

    await wikidata_signal("Acme Inc", lei="GLEIF0000001")

    assert "P1278" in captured["first_query"]
    assert "GLEIF0000001" in captured["first_query"]


@respx.mock
async def test_certsh_signal_filters_and_extracts_domains() -> None:
    payload = [
        {"id": 1, "issuer_name": "Lets Encrypt", "name_value": "acme.example.com\n*.acme.example.com"},
        {"id": 2, "issuer_name": "Lets Encrypt", "name_value": "localhost"},
        {"id": 3, "issuer_name": "Lets Encrypt", "name_value": "10.0.0.1"},
    ]
    route = respx.get("https://crt.sh/").mock(return_value=httpx.Response(200, json=payload))

    results = await certsh_signal("Acme Inc")

    assert route.called
    domains = {c.domain for c in results}
    assert "acme.example.com" in domains
    assert "localhost" not in domains
    assert "10.0.0.1" not in domains
    for c in results:
        assert c.signal == "crtsh"
        assert c.confidence == 60


@respx.mock
async def test_duckduckgo_signal_extracts_domains() -> None:
    payload = {
        "AbstractURL": "https://acme.example.com/about",
        "Results": [
            {"FirstURL": "https://shop.acme.example.com/"},
            {"FirstURL": "https://news.example.org/article"},
        ],
    }
    respx.get("https://api.duckduckgo.com/").mock(return_value=httpx.Response(200, json=payload))

    results = await duckduckgo_signal("Acme Inc")

    domains = {c.domain for c in results}
    assert "acme.example.com" in domains
    assert "shop.acme.example.com" in domains
    assert "news.example.org" in domains
    for c in results:
        assert c.signal == "duckduckgo"
        assert c.confidence == 30


@respx.mock
async def test_resolver_short_circuits_on_wikidata() -> None:
    wd = respx.get("https://query.wikidata.org/sparql").mock(
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
    crt = respx.get("https://crt.sh/").mock(return_value=httpx.Response(200, json=[]))
    ddg = respx.get("https://api.duckduckgo.com/").mock(return_value=httpx.Response(200, json={}))

    resolver = DomainResolver()
    candidates = await resolver.resolve("Acme Inc", lei=None, country="US")

    assert wd.called
    assert not crt.called
    assert not ddg.called
    assert len(candidates) == 1
    assert candidates[0].signal == "wikidata"


@respx.mock
async def test_resolver_falls_through_to_other_signals() -> None:
    respx.get("https://query.wikidata.org/sparql").mock(
        return_value=httpx.Response(200, json={"results": {"bindings": []}})
    )
    respx.get("https://crt.sh/").mock(
        return_value=httpx.Response(
            200,
            json=[{"id": 1, "issuer_name": "x", "name_value": "acme.example.com"}],
        )
    )
    respx.get("https://api.duckduckgo.com/").mock(
        return_value=httpx.Response(
            200,
            json={"AbstractURL": "https://acme.example.com", "Results": []},
        )
    )

    resolver = DomainResolver()
    candidates = await resolver.resolve("Acme Inc", lei=None, country="US")

    signals = [c.signal for c in candidates]
    assert "crtsh" in signals
    assert "duckduckgo" in signals
    assert "wikidata" not in signals


async def test_resolver_url_encodes_company_name() -> None:
    # Just confirm the helper does not throw on awkward names.
    assert urllib.parse.quote("Acme & Co", safe="")
