from __future__ import annotations

import os
from datetime import datetime
from typing import Any, ClassVar

import httpx

from ...base import CompanyLocation, CompanyRecord, CrawlResponse, SourceAdapter, compute_hash

_USER_AGENT = "corpscout/1.0 (https://github.com/pulsarpoint/corpscout; ops@pulsarpoint.com)"

# The CH Advanced Search API rejects start_index + size > 10_000.
# With page_size=100, pages 0-99 are safe (page 99: start_index=9900).
_CH_MAX_PAGE = 99


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
        config: dict[str, Any] | None = None,
    ) -> CrawlResponse:
        _cfg = config or {}
        api_url = _cfg.get("api_url") or self.endpoint
        page_size = int(_cfg.get("page_size") or self.page_size)
        auth_env = _cfg.get("auth_env") or "COMPANIES_HOUSE_API_KEY"

        api_key = os.getenv(auth_env)
        if not api_key:
            raise RuntimeError(f"{auth_env} is not set — API requires HTTP Basic auth")

        # cursor = "YYYY-MM-DD,N" (incorporated_from date + 0-indexed page within bucket)
        # or None / legacy integer → start from page 0 with no date filter.
        date_cursor: str | None = None
        page_offset: int = 0

        if cursor and "," in cursor:
            parts = cursor.split(",", 1)
            date_cursor = parts[0] or None
            try:
                page_offset = int(parts[1])
            except ValueError:
                page_offset = 0

        start_index = page_offset * page_size
        params: dict[str, Any] = {
            "size": str(page_size),
            "start_index": str(start_index),
            "company_status": "active",
        }
        if date_cursor:
            params["incorporated_from"] = date_cursor

        async with httpx.AsyncClient(timeout=30.0, auth=(api_key, "")) as client:
            resp = await client.get(api_url, params=params, headers={"Accept": "application/json", "User-Agent": _USER_AGENT})
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

        # CH API never returns total_results; use `hits` for the total count.
        total = int(data.get("hits") or 0)
        has_more = len(items) == page_size

        next_cursor: str | None = None
        if has_more:
            if page_offset < _CH_MAX_PAGE:
                next_cursor = f"{date_cursor or ''},{page_offset + 1}"
            else:
                last_date = (items[-1].get("date_of_creation") or "") if items else ""
                next_cursor = f"{last_date},0"

        return CrawlResponse(
            records=records,
            has_more=has_more,
            total=total,
            next_cursor=next_cursor,
        )
