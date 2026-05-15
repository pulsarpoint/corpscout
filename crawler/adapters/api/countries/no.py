from __future__ import annotations

from datetime import datetime
from typing import Any, ClassVar

import httpx

from ...base import CompanyRecord, CrawlResponse, SourceAdapter, compute_hash


class BrregAdapter(SourceAdapter):
    source_name: ClassVar[str] = "brreg"
    endpoint: ClassVar[str] = "https://data.brreg.no/enhetsregisteret/api/enheter"
    page_size: ClassVar[int] = 200

    async def crawl(
        self,
        since: datetime | None,
        cursor: str | None,
        page: int,
    ) -> CrawlResponse:
        zero_page = max(page - 1, 0)
        params: dict[str, Any] = {
            "page": str(zero_page),
            "size": str(self.page_size),
            "sort": "registreringsdatoEnhetsregisteret,asc",
        }
        if since is not None:
            params["fraRegistreringsdato"] = since.date().isoformat()

        async with httpx.AsyncClient(timeout=30.0) as client:
            resp = await client.get(self.endpoint, params=params, headers={"Accept": "application/json"})
            resp.raise_for_status()
            data = resp.json()

        embedded = (data.get("_embedded") or {}).get("enheter") or []
        records: list[CompanyRecord] = []
        for item in embedded:
            konkurs = bool(item.get("konkurs"))
            under_avvikling = bool(item.get("underAvvikling"))
            status = "dissolved" if (konkurs or under_avvikling) else "active"

            records.append(
                CompanyRecord(
                    name=str(item.get("navn") or ""),
                    country_iso2="NO",
                    registration_number=str(item.get("organisasjonsnummer") or ""),
                    status=status,
                    website=item.get("hjemmeside"),
                    raw_data=item,
                    snapshot_hash=compute_hash(item),
                )
            )

        page_info = data.get("page") or {}
        total = int(page_info.get("totalElements") or 0)
        total_pages = int(page_info.get("totalPages") or 0)
        current = int(page_info.get("number") or zero_page)
        has_more = (current + 1) < total_pages
        next_cursor = str(current + 2) if has_more else None  # convert back to 1-indexed

        return CrawlResponse(
            records=records,
            has_more=has_more,
            total=total,
            next_cursor=next_cursor,
        )
