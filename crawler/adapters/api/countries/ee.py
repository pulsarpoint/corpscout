from __future__ import annotations

from datetime import datetime
from typing import Any, ClassVar

import httpx

from ...base import CompanyRecord, CrawlResponse, SourceAdapter, compute_hash

_USER_AGENT = "corpscout/1.0 (https://github.com/pulsarpoint/corpscout; ops@pulsarpoint.com)"

_STATUS_MAP = {"R": "active", "K": "dissolved", "L": "inactive"}


class EstoniaAdapter(SourceAdapter):
    source_name: ClassVar[str] = "ariregister"
    endpoint: ClassVar[str] = "https://ariregister.rik.ee/api/1/"
    page_size: ClassVar[int] = 200

    async def crawl(
        self,
        since: datetime | None,
        cursor: str | None,
        page: int,
    ) -> CrawlResponse:
        offset = int(cursor) if cursor else 0
        params: dict[str, Any] = {
            "meetod": "searchcompany_v2",
            "keel": "eng",
            "type": "simple",
            "q": "",
            "offset": str(offset),
            "limit": str(self.page_size),
        }

        async with httpx.AsyncClient(timeout=30.0) as client:
            resp = await client.get(self.endpoint, params=params, headers={"Accept": "application/json", "User-Agent": _USER_AGENT})
            resp.raise_for_status()
            data = resp.json()

        results = data.get("results") or []
        records: list[CompanyRecord] = []
        for item in results:
            staatus = (item.get("staatus") or "").upper()
            status = _STATUS_MAP.get(staatus, "active")
            records.append(
                CompanyRecord(
                    name=str(item.get("nimi") or ""),
                    country_iso2="EE",
                    registration_number=str(item.get("ariregistri_kood") or ""),
                    status=status,
                    raw_data=item,
                    snapshot_hash=compute_hash(item),
                )
            )

        total = int(data.get("total") or 0)
        has_more = (offset + self.page_size) < total
        next_cursor = str(offset + self.page_size) if has_more else None

        return CrawlResponse(
            records=records,
            has_more=has_more,
            total=total,
            next_cursor=next_cursor,
        )
