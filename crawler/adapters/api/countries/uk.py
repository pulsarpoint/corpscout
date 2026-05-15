from __future__ import annotations

import os
from datetime import datetime
from typing import Any, ClassVar

import httpx

from ...base import CompanyRecord, CrawlResponse, SourceAdapter, compute_hash

_USER_AGENT = "corpscout/1.0 (https://github.com/pulsarpoint/corpscout; ops@pulsarpoint.com)"


def _map_status(value: str | None) -> str:
    v = (value or "").lower()
    if v == "dissolved":
        return "dissolved"
    if v in {"liquidation", "receivership"}:
        return "inactive"
    return "active"


class CompaniesHouseAdapter(SourceAdapter):
    source_name: ClassVar[str] = "companies_house"
    endpoint: ClassVar[str] = (
        "https://api.company-information.service.gov.uk/advanced-search/companies"
    )
    page_size: ClassVar[int] = 100

    async def crawl(
        self,
        since: datetime | None,
        cursor: str | None,
        page: int,
    ) -> CrawlResponse:
        api_key = os.getenv("COMPANIES_HOUSE_API_KEY")
        if not api_key:
            raise RuntimeError("COMPANIES_HOUSE_API_KEY is not set — API requires HTTP Basic auth")

        effective_page = int(cursor) if cursor else max(page, 1)
        start_index = (effective_page - 1) * self.page_size
        params: dict[str, Any] = {
            "size": str(self.page_size),
            "start_index": str(start_index),
            "company_status": "active",
        }

        async with httpx.AsyncClient(timeout=30.0, auth=(api_key, "")) as client:
            resp = await client.get(self.endpoint, params=params, headers={"Accept": "application/json", "User-Agent": _USER_AGENT})
            resp.raise_for_status()
            data = resp.json()

        items = data.get("items") or []
        records: list[CompanyRecord] = []
        for item in items:
            records.append(
                CompanyRecord(
                    name=str(item.get("company_name") or ""),
                    country_iso2="GB",
                    registration_number=str(item.get("company_number") or ""),
                    status=_map_status(item.get("company_status")),
                    raw_data=item,
                    snapshot_hash=compute_hash(item),
                )
            )

        total = int(data.get("total_results") or 0)
        start = int(data.get("start_index") or start_index)
        has_more = (start + len(items)) < total
        next_cursor = str(effective_page + 1) if has_more else None

        return CrawlResponse(
            records=records,
            has_more=has_more,
            total=total,
            next_cursor=next_cursor,
        )
