from __future__ import annotations

import os
from datetime import datetime
from typing import Any, ClassVar

import httpx

from ...base import CompanyLocation, CompanyRecord, CrawlResponse, SourceAdapter, compute_hash

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
            locations = []
            addr = item.get("registered_office_address") or {}
            if addr:
                locations.append(CompanyLocation(
                    location_type="registered_address",
                    address_line1=addr.get("address_line_1"),
                    address_line2=addr.get("address_line_2"),
                    city=addr.get("locality"),
                    region=addr.get("region"),
                    postal_code=addr.get("postal_code"),
                    country=addr.get("country"),
                    country_code="GB",
                ))

            founded_year: int | None = None
            date_of_creation = item.get("date_of_creation")
            if date_of_creation:
                try:
                    founded_year = int(str(date_of_creation)[:4])
                except (ValueError, TypeError):
                    pass

            industries = []
            for sic in (item.get("sic_codes") or []):
                if sic:
                    industries.append(str(sic))

            records.append(
                CompanyRecord(
                    name=str(item.get("company_name") or ""),
                    country_iso2="GB",
                    registration_number=str(item.get("company_number") or ""),
                    status=_map_status(item.get("company_status")),
                    raw_data=item,
                    snapshot_hash=compute_hash(item),
                    locations=locations,
                    founded_year=founded_year,
                    industries=industries,
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
