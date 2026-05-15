from __future__ import annotations

from datetime import datetime

from fastapi import FastAPI, HTTPException
from pydantic import BaseModel

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


class CrawlRequest(BaseModel):
    since: datetime | None = None
    cursor: str | None = None
    page: int = 1


class ResolveRequest(BaseModel):
    company_name: str
    lei: str | None = None
    country: str = ""


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


@app.post("/resolve/domain", response_model=ResolveResponse)
async def resolve_domain(req: ResolveRequest) -> ResolveResponse:
    candidates = await resolver.resolve(req.company_name, req.lei, req.country)
    return ResolveResponse(candidates=candidates)
