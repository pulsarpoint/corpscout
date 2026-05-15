from __future__ import annotations

import os
from datetime import datetime
from typing import Any, ClassVar

import httpx

from ...base import CompanyRecord, CrawlResponse, SourceAdapter, compute_hash

_USER_AGENT = "corpscout/1.0 (https://github.com/pulsarpoint/corpscout; ops@pulsarpoint.com)"


class CVRAdapter(SourceAdapter):
    source_name: ClassVar[str] = "cvr"
    endpoint: ClassVar[str] = "https://cvrapi.dk/api"
    page_size: ClassVar[int] = 100

    async def crawl(
        self,
        since: datetime | None,
        cursor: str | None,
        page: int,
    ) -> CrawlResponse:
        token = os.getenv("CVR_API_TOKEN")

        offset = int(cursor) if cursor else 0
        params: dict[str, Any] = {
            "search": "",
            "country": "dk",
            "start": str(offset),
        }
        if token:
            params["token"] = token

        async with httpx.AsyncClient(timeout=30.0) as client:
            resp = await client.get(self.endpoint, params=params, headers={"Accept": "application/json", "User-Agent": _USER_AGENT})
            resp.raise_for_status()
            data = resp.json()

        results = data if isinstance(data, list) else []
        records: list[CompanyRecord] = []
        for item in results:
            enddate = item.get("enddate")
            status = "dissolved" if enddate else "active"
            records.append(
                CompanyRecord(
                    name=str(item.get("name") or ""),
                    country_iso2="DK",
                    registration_number=str(item.get("vat") or ""),
                    status=status,
                    website=item.get("website"),
                    raw_data=item,
                    snapshot_hash=compute_hash(item),
                )
            )

        has_more = len(results) >= self.page_size
        next_cursor = str(offset + self.page_size) if has_more else None

        return CrawlResponse(
            records=records,
            has_more=has_more,
            total=-1,  # API does not return a total
            next_cursor=next_cursor,
        )
