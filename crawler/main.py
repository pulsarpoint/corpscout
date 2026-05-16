from __future__ import annotations

from datetime import datetime

import logging

from fastapi import FastAPI, HTTPException, Request
from fastapi.responses import JSONResponse
import httpx as _httpx
from pydantic import BaseModel

_logger = logging.getLogger("corpscout.crawler")

from adapters import registry
from adapters.api.gleif import GLEIFAdapter
from adapters.api.opencorporates import OpenCorporatesAdapter
from adapters.api.wikidata import WikidataAdapter
from adapters.api.countries.dk import CVRAdapter
from adapters.api.countries.ee import EstoniaAdapter
from adapters.api.countries.no import BrregAdapter
from adapters.api.countries.uk import CompaniesHouseAdapter
from adapters.base import CrawlResponse, ResolveResponse
from adapters.crawl4ai.generic import Crawl4AIGenericAdapter, Crawl4AIUnconfiguredError
from domain_resolver import DomainResolver

app = FastAPI(title="corpscout-crawler", version="0.2.0")
resolver = DomainResolver()


@app.exception_handler(_httpx.HTTPError)
async def _upstream_http_error(request: Request, exc: _httpx.HTTPError) -> JSONResponse:
    response = getattr(exc, "response", None)
    status = getattr(response, "status_code", None)
    # Log with URL redacted to avoid leaking API tokens from query params.
    req = getattr(exc, "request", None)
    safe_url = str(req.url.copy_with(params={})) if req is not None else "unknown"
    _logger.error("upstream request failed: %s %s (status=%s)", type(exc).__name__, safe_url, status)
    return JSONResponse(
        status_code=502,
        content={"error": "upstream_failed", "upstream_status": status},
    )


class CrawlRequest(BaseModel):
    since: datetime | None = None
    cursor: str | None = None
    page: int = 1


class ResolveRequest(BaseModel):
    company_name: str
    lei: str | None = None
    country: str = ""


class GLEIFRelationshipRequest(BaseModel):
    lei: str


class GLEIFRelationshipResponse(BaseModel):
    lei: str
    direct_parent_lei: str | None = None
    ultimate_parent_lei: str | None = None


def register_default_adapters() -> None:
    """(Re)register all default adapters. Idempotent."""
    registry.reset()
    registry.register(GLEIFAdapter())
    registry.register(WikidataAdapter())
    registry.register(OpenCorporatesAdapter())
    registry.register(CompaniesHouseAdapter())
    registry.register(BrregAdapter())
    registry.register(CVRAdapter())
    registry.register(EstoniaAdapter())
    registry.register(Crawl4AIGenericAdapter())


register_default_adapters()


@app.get("/health")
async def health() -> dict[str, str]:
    return {"status": "ok"}


@app.get("/sources")
async def list_sources() -> dict[str, list[str]]:
    return {"sources": registry.list_sources()}


@app.post("/crawl/{source_name}", response_model=CrawlResponse)
async def crawl(source_name: str, req: CrawlRequest) -> CrawlResponse:
    adapter = registry.get_adapter(source_name)
    if adapter is None:
        raise HTTPException(status_code=404, detail=f"source not found: {source_name}")
    try:
        return await adapter.crawl(req.since, req.cursor, req.page)
    except Crawl4AIUnconfiguredError as e:
        raise HTTPException(status_code=501, detail=str(e))
    except RuntimeError as e:
        raise HTTPException(status_code=500, detail=str(e))


@app.post("/resolve/domain", response_model=ResolveResponse)
async def resolve_domain(req: ResolveRequest) -> ResolveResponse:
    candidates = await resolver.resolve(req.company_name, req.lei, req.country)
    return ResolveResponse(candidates=candidates)


_GLEIF_BASE = "https://api.gleif.org/api/v1/lei-records"
_GLEIF_USER_AGENT = "corpscout/1.0 (https://github.com/pulsarpoint/corpscout; ops@pulsarpoint.com)"


def _extract_lei_from_relationship(data: dict) -> str | None:
    """Extract end-node LEI from a GLEIF relationship record response."""
    try:
        end_node = data["data"]["relationships"]["end-node"]["links"]["related"]
        # URL ends with /lei-records/{LEI}
        return end_node.rstrip("/").split("/")[-1]
    except (KeyError, TypeError, AttributeError):
        return None


@app.post("/enrich/gleif/relationship", response_model=GLEIFRelationshipResponse)
async def enrich_gleif_relationship(req: GLEIFRelationshipRequest) -> GLEIFRelationshipResponse:
    headers = {"Accept": "application/json", "User-Agent": _GLEIF_USER_AGENT}
    direct_parent_lei: str | None = None
    ultimate_parent_lei: str | None = None

    async with _httpx.AsyncClient(timeout=20.0) as client:
        direct_url = f"{_GLEIF_BASE}/{req.lei}/direct-parent-relationship"
        try:
            r = await client.get(direct_url, headers=headers)
            if r.status_code == 200:
                direct_parent_lei = _extract_lei_from_relationship(r.json())
        except _httpx.HTTPError:
            pass

        ultimate_url = f"{_GLEIF_BASE}/{req.lei}/ultimate-parent-relationship"
        try:
            r = await client.get(ultimate_url, headers=headers)
            if r.status_code == 200:
                ultimate_parent_lei = _extract_lei_from_relationship(r.json())
        except _httpx.HTTPError:
            pass

    return GLEIFRelationshipResponse(
        lei=req.lei,
        direct_parent_lei=direct_parent_lei,
        ultimate_parent_lei=ultimate_parent_lei,
    )
