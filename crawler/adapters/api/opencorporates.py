from __future__ import annotations

import os
from datetime import datetime
from typing import Any, ClassVar

import httpx

from ..base import CompanyLocation, CompanyRecord, CrawlResponse, SourceAdapter, compute_hash

_USER_AGENT = "corpscout/1.0 (https://github.com/pulsarpoint/corpscout; ops@pulsarpoint.com)"


def _map_status(current_status: str | None, inactive: bool) -> str:
    if inactive:
        return "inactive"
    s = (current_status or "").lower()
    if "dissolved" in s:
        return "dissolved"
    return "active"


class OpenCorporatesAdapter(SourceAdapter):
    source_name: ClassVar[str] = "opencorporates"
    endpoint: ClassVar[str] = "https://api.opencorporates.com/v0.4/companies/search"
    page_size: ClassVar[int] = 100

    async def crawl(
        self,
        since: datetime | None,
        cursor: str | None,
        page: int,
        config: dict[str, Any] | None = None,
    ) -> CrawlResponse:
        api_key = (os.getenv("CRAWLER_OPENCORPORATES_API_KEY") or "").strip()
        if not api_key or api_key.startswith("#"):
            return CrawlResponse(records=[], has_more=False, total=0, next_cursor=None)

        params: dict[str, Any] = {
            "q": "*",
            "inactive": "false",
            "per_page": str(self.page_size),
            "page": str(int(cursor) if cursor else max(page, 1)),
            "api_token": api_key,
        }

        async with httpx.AsyncClient(timeout=30.0) as client:
            resp = await client.get(self.endpoint, params=params, headers={"Accept": "application/json", "User-Agent": _USER_AGENT})
            resp.raise_for_status()
            data = resp.json()

        results = (data.get("results") or {})
        companies = results.get("companies") or []
        records: list[CompanyRecord] = []
        for wrapper in companies:
            company = wrapper.get("company") or {}
            jurisdiction = (company.get("jurisdiction_code") or "").lower()
            country = jurisdiction.upper() if len(jurisdiction) == 2 else jurisdiction[:2].upper()

            locations = []
            addr = company.get("registered_address") or {}
            if addr:
                locations.append(CompanyLocation(
                    location_type="registered_address",
                    address_line1=addr.get("street_address"),
                    city=addr.get("locality"),
                    region=addr.get("region"),
                    postal_code=addr.get("postal_code"),
                    country=addr.get("country"),
                    country_code=country or None,
                ))

            founded_year: int | None = None
            inc_date = company.get("incorporation_date")
            if inc_date:
                try:
                    founded_year = int(str(inc_date)[:4])
                except (ValueError, TypeError):
                    pass

            industries = []
            for code_entry in (company.get("industry_codes") or []):
                desc = (code_entry.get("industry_code") or {}).get("description") or code_entry.get("description")
                if desc:
                    industries.append(str(desc))

            records.append(
                CompanyRecord(
                    name=str(company.get("name") or ""),
                    country_iso2=country,
                    registration_number=str(company.get("company_number") or ""),
                    status=_map_status(company.get("current_status"), bool(company.get("inactive"))),
                    raw_data=company,
                    snapshot_hash=compute_hash(company),
                    locations=locations,
                    founded_year=founded_year,
                    industries=industries,
                )
            )

        total = int(results.get("total_count") or 0)
        per_page = int(results.get("per_page") or self.page_size)
        current_page = int(results.get("page") or page)
        has_more = (current_page * per_page) < total
        next_cursor = str(current_page + 1) if has_more else None

        return CrawlResponse(
            records=records,
            has_more=has_more,
            total=total,
            next_cursor=next_cursor,
        )
