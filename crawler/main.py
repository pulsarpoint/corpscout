from fastapi import FastAPI, HTTPException
from adapters.base import CrawlResponse, ResolveResponse
from pydantic import BaseModel
from datetime import datetime

app = FastAPI(title="corpscout-crawler", version="0.1.0")


class CrawlRequest(BaseModel):
    since: datetime
    page: int = 1
    cursor: str | None = None


class ResolveRequest(BaseModel):
    company_name: str
    lei: str | None = None
    country: str


@app.get("/health")
async def health():
    return {"status": "ok"}


@app.post("/crawl/{source_name}", response_model=CrawlResponse)
async def crawl(source_name: str, req: CrawlRequest):
    # Adapters registered in Plan 2
    raise HTTPException(status_code=404, detail=f"source not found: {source_name}")


@app.post("/resolve/domain", response_model=ResolveResponse)
async def resolve_domain(req: ResolveRequest):
    # Domain resolver implemented in Plan 2
    return ResolveResponse(candidates=[])
