from __future__ import annotations

from datetime import datetime
from typing import Any, ClassVar

import httpx

from ..base import CompanyRecord, CrawlResponse, SourceAdapter, compute_hash

_STATUS_MAP = {
    "ACTIVE": "active",
    "INACTIVE": "inactive",
    "ANNULLED": "dissolved",
}


class GLEIFAdapter(SourceAdapter):
    source_name: ClassVar[str] = "gleif"
    base_url: ClassVar[str] = "https://api.gleif.org/api/v1/lei-records"
    page_size: ClassVar[int] = 200

    async def crawl(
        self,
        since: datetime | None,
        cursor: str | None,
        page: int,
    ) -> CrawlResponse:
        params: dict[str, Any] = {
            "page[number]": str(max(page, 1)),
            "page[size]": str(self.page_size),
        }
        if since is not None:
            params["filter[lastUpdateTime]"] = since.isoformat()

        async with httpx.AsyncClient(timeout=30.0) as client:
            resp = await client.get(self.base_url, params=params, headers={"Accept": "application/json"})
            resp.raise_for_status()
            data = resp.json()

        records: list[CompanyRecord] = []
        for item in data.get("data", []):
            attrs = item.get("attributes", {}) or {}
            entity = attrs.get("entity", {}) or {}
            legal_name = (entity.get("legalName") or {}).get("name") or ""
            country = ((entity.get("legalAddress") or {}).get("country") or "").upper()
            raw_status = (entity.get("status") or "").upper()
            status = _STATUS_MAP.get(raw_status, "active")
            lei = item.get("id")
            records.append(
                CompanyRecord(
                    name=legal_name,
                    country_iso2=country,
                    lei=lei,
                    status=status,
                    raw_data=item,
                    snapshot_hash=compute_hash(item),
                )
            )

        pagination = (data.get("meta") or {}).get("pagination") or {}
        total = int(pagination.get("total") or 0)
        current_page = int(pagination.get("currentPage") or page)
        last_page = int(pagination.get("lastPage") or current_page)
        has_more = current_page < last_page
        next_cursor = str(current_page + 1) if has_more else None

        return CrawlResponse(
            records=records,
            has_more=has_more,
            total=total,
            next_cursor=next_cursor,
        )
