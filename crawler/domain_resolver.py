from __future__ import annotations

import asyncio
import ipaddress
import logging
import re
import socket
import urllib.parse
from typing import ClassVar

import httpx

from adapters.base import DomainCandidate

logger = logging.getLogger("corpscout.resolver")

_USER_AGENT = "corpscout/1.0 (https://github.com/pulsarpoint/corpscout; ops@pulsarpoint.com)"

# Per-service locks serialise concurrent resolver calls so the asyncio.sleep
# rate-limiting delays actually gate all callers, not just sequential ones
# within a single task.  Without these, N concurrent domain_resolve workers
# all reach the external API simultaneously and trigger rate-limit errors.
_CERTSH_LOCK: asyncio.Lock | None = None
_WIKIDATA_LOCK: asyncio.Lock | None = None
_SEARCH_LOCK: asyncio.Lock | None = None


def _get_lock(ref: str) -> asyncio.Lock:
    """Return (creating lazily) the named module-level lock."""
    global _CERTSH_LOCK, _WIKIDATA_LOCK, _SEARCH_LOCK  # noqa: PLW0603
    if ref == "certsh":
        if _CERTSH_LOCK is None:
            _CERTSH_LOCK = asyncio.Lock()
        return _CERTSH_LOCK
    if ref == "wikidata":
        if _WIKIDATA_LOCK is None:
            _WIKIDATA_LOCK = asyncio.Lock()
        return _WIKIDATA_LOCK
    if _SEARCH_LOCK is None:
        _SEARCH_LOCK = asyncio.Lock()
    return _SEARCH_LOCK

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

# Legal-form suffixes to strip before generating domain candidates, ordered
# longest-first so "private limited company" matches before "limited".
_LEGAL_SUFFIXES = [
    r"\bprivate limited company\b", r"\bpublic limited company\b",
    r"\bgesellschaft mit beschränkter haftung\b",
    r"\baksjeselskap\b", r"\bannpartselskab\b", r"\banpartsselskab\b",
    r"\baktieselskab\b", r"\baksjonærselskap\b",
    r"\bsociété anonyme\b", r"\bsociété à responsabilité limitée\b",
    r"\bsociedad anónima\b", r"\bsociedad limitada\b",
    r"\bsociedad de responsabilidad limitada\b",
    r"\bcompagnie\b",
    r"\bincorporated\b", r"\bcorporation\b",
    r"\blimited liability company\b", r"\blimited liability partnership\b",
    r"\bllc\b", r"\bllp\b", r"\binc\b", r"\bltd\b", r"\bplc\b", r"\bcorp\b",
    r"\bco\b",
    r"\b(as|asa|ans|da|ba|sa|nuf|ks|sf)\b",  # Norwegian
    r"\b(a/s|a\.s)\b",                         # Danish/Norwegian
    r"\b(ab|hb|kb|ek)\b",                      # Swedish
    r"\b(bv|nv|vof|cv|ov|vve)\b",              # Dutch
    r"\b(gmbh|ag|kg|ohg|kgaa|eg|gbr|ug)\b",   # German
    r"\b(srl|spa|sas|snc|sapa|scarl)\b",       # Italian
    r"\b(sarl|sas|sc|snc|sca)\b",              # French
    r"\b(sl|sa|cb|scp)\b",                     # Spanish
    r"\b(oy|oyj|ky|ay|osk)\b",                # Finnish
    r"\b(as|ou|mtü|tü|uü)\b",                 # Estonian
    r"\b(sp\.? z\.? o\.? o\.?)\b",            # Polish
    r"\b(lda|s\.a\.)\b",                       # Portuguese
]
_LEGAL_RE = re.compile(
    "|".join(_LEGAL_SUFFIXES),
    re.IGNORECASE,
)

# Country ISO-2 → primary ccTLD mapping.
_COUNTRY_TLD: dict[str, str] = {
    "NO": ".no", "DK": ".dk", "SE": ".se", "FI": ".fi",
    "GB": ".co.uk", "DE": ".de", "FR": ".fr", "NL": ".nl",
    "IT": ".it", "ES": ".es", "PT": ".pt", "PL": ".pl",
    "AT": ".at", "CH": ".ch", "BE": ".be", "CZ": ".cz",
    "HU": ".hu", "RO": ".ro", "SK": ".sk", "BG": ".bg",
    "HR": ".hr", "SI": ".si", "EE": ".ee", "LV": ".lv",
    "LT": ".lt", "US": ".com", "CA": ".ca", "AU": ".com.au",
    "NZ": ".co.nz", "JP": ".co.jp", "KR": ".co.kr",
    "CN": ".cn", "IN": ".in", "BR": ".com.br", "MX": ".com.mx",
    "AR": ".com.ar", "ZA": ".co.za",
}


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
    host = host.rstrip(".")
    while host.startswith("*."):
        host = host[2:]
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


def _company_slug(name: str) -> str:
    """Normalise a company name into a domain-name slug.

    Strips legal suffixes, converts to lowercase, replaces non-alphanumeric
    runs with hyphens, and trims leading/trailing hyphens.
    """
    s = _LEGAL_RE.sub("", name)
    s = s.lower()
    s = re.sub(r"[^a-z0-9]+", "-", s)
    s = s.strip("-")
    return s


def _candidate_domains(name: str, country: str) -> list[str]:
    """Generate plausible domain candidates for a company name + country."""
    slug = _company_slug(name)
    if not slug or len(slug) < 2:
        return []
    # Remove any leading numbers/hyphens that make invalid domains
    slug_clean = slug.lstrip("0123456789-")
    if not slug_clean or len(slug_clean) < 2:
        slug_clean = slug

    tlds: list[str] = [".com"]
    country_tld = _COUNTRY_TLD.get(country.upper())
    if country_tld and country_tld != ".com":
        tlds = [country_tld, ".com"]

    seen: set[str] = set()
    candidates: list[str] = []
    for tld in tlds:
        domain = slug_clean + tld
        if domain not in seen and "." in domain:
            seen.add(domain)
            candidates.append(domain)
        # Also try slug without hyphens for short names
        slug_nohyphen = slug_clean.replace("-", "")
        if slug_nohyphen != slug_clean and len(slug_nohyphen) >= 3:
            domain2 = slug_nohyphen + tld
            if domain2 not in seen:
                seen.add(domain2)
                candidates.append(domain2)
    return candidates


def _dns_resolve(domain: str) -> bool:
    """Return True if the domain resolves via DNS (has an A or AAAA record)."""
    try:
        socket.getaddrinfo(domain, None, proto=socket.IPPROTO_TCP)
        return True
    except OSError:
        return False


async def wikidata_signal(company_name: str, lei: str | None) -> list[DomainCandidate]:
    queries: list[str] = []
    if lei:
        queries.append(_WIKIDATA_TEMPLATE_BY_LEI.format(lei=_sparql_literal(lei)))
    queries.append(_WIKIDATA_TEMPLATE_BY_NAME.format(name=_sparql_literal(company_name)))

    candidates: list[DomainCandidate] = []
    seen: set[str] = set()
    async with _get_lock("wikidata"):
        async with httpx.AsyncClient(timeout=30.0) as client:
            for query in queries:
                try:
                    resp = await client.get(
                        "https://query.wikidata.org/sparql",
                        params={"query": query, "format": "json"},
                        headers={"User-Agent": _USER_AGENT, "Accept": "application/sparql-results+json"},
                    )
                    if resp.status_code == 429:
                        # Wikidata rate-limit: back off and skip remaining queries.
                        retry_after = int(resp.headers.get("retry-after", "60"))
                        logger.warning("wikidata 429 — backing off %ds", retry_after)
                        await asyncio.sleep(min(retry_after, 120))
                        break
                    resp.raise_for_status()
                except httpx.HTTPError as e:
                    logger.warning("wikidata signal failed: %s", e)
                    await asyncio.sleep(2.0)
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
                await asyncio.sleep(1.0)  # honour Wikidata ~1 req/s
                if candidates:
                    break
    return candidates


async def certsh_signal(company_name: str) -> list[DomainCandidate]:
    candidates: list[DomainCandidate] = []
    seen: set[str] = set()
    async with _get_lock("certsh"):
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
                await asyncio.sleep(1.0)
                return candidates

        if isinstance(entries, list):
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
                            signal="certsh",
                            confidence=60,
                            evidence={
                                "cert_id": entry.get("id"),
                                "issuer": entry.get("issuer_name", ""),
                            },
                        )
                    )

        await asyncio.sleep(0.5)  # honour 2 req/s; sleep inside lock so it gates all callers
    return candidates


async def heuristic_signal(company_name: str, country: str) -> list[DomainCandidate]:
    """Generate domain candidates from company name + country TLD and verify via DNS."""
    candidates_raw = _candidate_domains(company_name, country)
    if not candidates_raw:
        return []

    results: list[DomainCandidate] = []
    loop = asyncio.get_event_loop()
    for domain in candidates_raw:
        try:
            resolves = await loop.run_in_executor(None, _dns_resolve, domain)
        except Exception:
            resolves = False
        if resolves:
            results.append(
                DomainCandidate(
                    domain=domain,
                    signal="search",
                    confidence=40,
                    evidence={"method": "name_heuristic", "verified": "dns"},
                )
            )
    return results


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

        certsh_results, heuristic_results = await asyncio.gather(
            certsh_signal(company_name),
            heuristic_signal(company_name, country),
        )
        return certsh_results + heuristic_results
