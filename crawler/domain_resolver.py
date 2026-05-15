from __future__ import annotations

import asyncio
import ipaddress
import logging
import urllib.parse
from typing import ClassVar

import httpx

from adapters.base import DomainCandidate

logger = logging.getLogger("corpscout.resolver")

_USER_AGENT = "corpscout/1.0 (https://github.com/pulsarpoint/corpscout; ops@pulsarpoint.com)"

_WIKIDATA_TEMPLATE_BY_NAME = """
SELECT ?company ?website WHERE {{
  ?company wdt:P856 ?website .
  ?company rdfs:label "{name}"@en .
}}
LIMIT 5
""".strip()

_WIKIDATA_TEMPLATE_BY_LEI = """
SELECT ?company ?website WHERE {{
  ?company wdt:P1278 "{lei}" .
  ?company wdt:P856 ?website .
}}
LIMIT 5
""".strip()


def _safe_domain(url: str) -> str | None:
    """Extract a usable hostname from a URL.

    Returns None for IP literals, localhost, empty values, or schemes
    that we do not care about.
    """
    if not url:
        return None
    parsed = urllib.parse.urlparse(url)
    host = parsed.netloc or parsed.path
    if not host:
        return None
    host = host.split("@")[-1].split(":")[0].lower().strip()
    host = host.removeprefix("*.")
    if not host or host == "localhost":
        return None
    try:
        ipaddress.ip_address(host)
        return None  # raw IPs are not domains
    except ValueError:
        pass
    if "." not in host:
        return None
    return host


async def wikidata_signal(company_name: str, lei: str | None) -> list[DomainCandidate]:
    queries: list[str] = []
    if lei:
        queries.append(_WIKIDATA_TEMPLATE_BY_LEI.format(lei=_sparql_literal(lei)))
    queries.append(_WIKIDATA_TEMPLATE_BY_NAME.format(name=_sparql_literal(company_name)))

    candidates: list[DomainCandidate] = []
    seen: set[str] = set()
    async with httpx.AsyncClient(timeout=30.0) as client:
        for query in queries:
            try:
                resp = await client.get(
                    "https://query.wikidata.org/sparql",
                    params={"query": query, "format": "json"},
                    headers={"User-Agent": _USER_AGENT, "Accept": "application/sparql-results+json"},
                )
                resp.raise_for_status()
            except httpx.HTTPError as e:
                logger.warning("wikidata signal failed: %s", e)
                continue
            data = resp.json()
            for binding in (data.get("results") or {}).get("bindings") or []:
                company_uri = (binding.get("company") or {}).get("value", "")
                website = (binding.get("website") or {}).get("value", "")
                domain = _safe_domain(website)
                if not domain or domain in seen:
                    continue
                seen.add(domain)
                candidates.append(
                    DomainCandidate(
                        domain=domain,
                        signal="wikidata",
                        confidence=85,
                        evidence={"wikidata_uri": company_uri, "website": website},
                    )
                )
            if candidates:
                # Returned the LEI-based or name-based hits; we are done.
                break
    return candidates


async def certsh_signal(company_name: str) -> list[DomainCandidate]:
    candidates: list[DomainCandidate] = []
    seen: set[str] = set()
    async with httpx.AsyncClient(timeout=30.0) as client:
        try:
            resp = await client.get(
                "https://crt.sh/",
                params={"q": company_name, "output": "json"},
                headers={"User-Agent": _USER_AGENT, "Accept": "application/json"},
            )
            resp.raise_for_status()
            entries = resp.json()
        except (httpx.HTTPError, ValueError) as e:
            logger.warning("crt.sh signal failed: %s", e)
            return candidates

    if not isinstance(entries, list):
        return candidates

    for entry in entries:
        name_value = entry.get("name_value") or ""
        for raw in str(name_value).splitlines():
            domain = _safe_domain(raw.strip())
            if not domain or domain in seen:
                continue
            seen.add(domain)
            candidates.append(
                DomainCandidate(
                    domain=domain,
                    signal="crtsh",
                    confidence=60,
                    evidence={
                        "cert_id": entry.get("id"),
                        "issuer": entry.get("issuer_name", ""),
                    },
                )
            )

    await asyncio.sleep(0.5)  # honour 2 req/s
    return candidates


async def duckduckgo_signal(company_name: str) -> list[DomainCandidate]:
    candidates: list[DomainCandidate] = []
    seen: set[str] = set()
    async with httpx.AsyncClient(timeout=30.0) as client:
        try:
            resp = await client.get(
                "https://api.duckduckgo.com/",
                params={
                    "q": f"{company_name} official site",
                    "format": "json",
                    "no_redirect": "1",
                    "no_html": "1",
                },
                headers={"User-Agent": _USER_AGENT, "Accept": "application/json"},
            )
            resp.raise_for_status()
            data = resp.json()
        except (httpx.HTTPError, ValueError) as e:
            logger.warning("duckduckgo signal failed: %s", e)
            return candidates

    abstract_url = data.get("AbstractURL")
    urls: list[str] = []
    if abstract_url:
        urls.append(abstract_url)
    for result in data.get("Results") or []:
        first = result.get("FirstURL")
        if first:
            urls.append(first)

    for url in urls:
        domain = _safe_domain(url)
        if not domain or domain in seen:
            continue
        seen.add(domain)
        candidates.append(
            DomainCandidate(
                domain=domain,
                signal="duckduckgo",
                confidence=30,
                evidence={"source": "duckduckgo", "abstract_url": abstract_url},
            )
        )

    await asyncio.sleep(1.0)  # honour 1 req/s
    return candidates


def _sparql_literal(value: str) -> str:
    """Escape a string for safe inclusion as a SPARQL literal body."""
    return value.replace("\\", "\\\\").replace('"', '\\"')


class DomainResolver:
    """Run domain-discovery signals in confidence order, with early exit."""

    early_exit_threshold: ClassVar[int] = 85

    async def resolve(
        self,
        company_name: str,
        lei: str | None,
        country: str,
    ) -> list[DomainCandidate]:
        # Wikidata first (confidence 85): if present we trust it and exit.
        wikidata_results = await wikidata_signal(company_name, lei)
        if wikidata_results:
            return wikidata_results

        certsh_results = await certsh_signal(company_name)
        search_results = await duckduckgo_signal(company_name)
        return certsh_results + search_results
