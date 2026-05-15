from __future__ import annotations
from pydantic import BaseModel


class CompanyRecord(BaseModel):
    name: str
    country_iso2: str
    registration_number: str | None = None
    lei: str | None = None
    status: str = "active"
    website: str | None = None
    aliases: list[str] = []
    raw_data: dict
    snapshot_hash: str


class CrawlResponse(BaseModel):
    records: list[CompanyRecord]
    has_more: bool
    total: int
    next_cursor: str | None = None


class DomainCandidate(BaseModel):
    domain: str
    signal: str
    confidence: int
    evidence: dict


class ResolveResponse(BaseModel):
    candidates: list[DomainCandidate]
