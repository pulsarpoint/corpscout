from __future__ import annotations

import hashlib
import json
from abc import ABC, abstractmethod
from datetime import datetime
from typing import Any, ClassVar

from pydantic import BaseModel


class CompanyLocation(BaseModel):
    location_type: str  # "registered_address", "headquarters", "office"
    address_line1: str | None = None
    address_line2: str | None = None
    city: str | None = None
    region: str | None = None
    postal_code: str | None = None
    country: str | None = None
    country_code: str | None = None
    source: str = "registry"


class CompanyPhone(BaseModel):
    phone: str
    purpose: str = "main"  # "main", "fax", "support"
    source: str = "registry"


class CompanyEmail(BaseModel):
    email: str
    purpose: str = "general"  # "general", "support", "billing"
    source: str = "registry"


class CompanyRecord(BaseModel):
    name: str
    country_iso2: str
    registration_number: str | None = None
    lei: str | None = None
    status: str = "active"
    website: str | None = None
    aliases: list[str] = []
    raw_data: dict
    snapshot_hash: str  # SHA-256 of canonical raw_data JSON
    # Enrichment fields — populated when the raw API response contains them
    locations: list[CompanyLocation] = []
    phones: list[CompanyPhone] = []
    emails: list[CompanyEmail] = []
    industries: list[str] = []
    founded_year: int | None = None
    employee_estimate: dict = {}


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


def compute_hash(payload: dict[str, Any]) -> str:
    """SHA-256 of a canonical JSON encoding of payload.

    Keys are sorted, separators are tight, and non-ASCII is preserved so
    that two semantically-equal payloads always hash to the same value.
    """
    try:
        encoded = json.dumps(payload, sort_keys=True, ensure_ascii=False, separators=(",", ":"))
    except TypeError as exc:
        raise TypeError(f"compute_hash: payload is not JSON-serialisable — {exc}") from exc
    return hashlib.sha256(encoded.encode("utf-8")).hexdigest()


class SourceAdapter(ABC):
    """Common interface for every corpscout source.

    Subclasses must set ``source_name`` and implement ``crawl``.
    """

    source_name: ClassVar[str]

    def __init__(self, **kwargs: Any) -> None:
        super().__init__(**kwargs)
        if not getattr(self, "source_name", None):
            raise TypeError(f"{type(self).__name__} must define class attribute source_name")

    @abstractmethod
    async def crawl(
        self,
        since: datetime | None,
        cursor: str | None,
        page: int,
        config: dict[str, Any] | None = None,
    ) -> CrawlResponse:
        """Fetch one page of records. Must be deterministic for a given input."""
        raise NotImplementedError
