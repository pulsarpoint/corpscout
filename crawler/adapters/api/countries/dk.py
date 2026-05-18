from __future__ import annotations

import os
from datetime import datetime
from typing import Any, ClassVar

import httpx

from ...base import CompanyEmail, CompanyLocation, CompanyPhone, CompanyRecord, CrawlResponse, SourceAdapter, compute_hash

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
        config: dict[str, Any] | None = None,
    ) -> CrawlResponse:
        _cfg = config or {}
        api_url = _cfg.get("api_url") or self.endpoint
        page_size = int(_cfg.get("page_size") or self.page_size)
        auth_env = _cfg.get("auth_env") or "CVR_API_TOKEN"

        token = (os.getenv(auth_env) or "").strip()
        if not token or token.startswith("#"):
            return CrawlResponse(records=[], has_more=False, total=0, next_cursor=None)

        offset = int(cursor) if cursor else 0
        params: dict[str, Any] = {
            "search": "",
            "country": "dk",
            "start": str(offset),
            "token": token,
        }

        async with httpx.AsyncClient(timeout=30.0) as client:
            resp = await client.get(api_url, params=params, headers={"Accept": "application/json", "User-Agent": _USER_AGENT})
            resp.raise_for_status()
            data = resp.json()

        results = data if isinstance(data, list) else []
        records: list[CompanyRecord] = []
        for item in results:
            enddate = item.get("enddate")
            status = "dissolved" if enddate else "active"

            locations = []
            street = item.get("address")
            city = item.get("city")
            zipcode = item.get("zipcode")
            if street or city:
                locations.append(CompanyLocation(
                    location_type="registered_address",
                    address_line1=street,
                    city=city,
                    postal_code=str(zipcode) if zipcode else None,
                    country="Denmark",
                    country_code="DK",
                ))

            phones = []
            phone = item.get("phone")
            if phone:
                phones.append(CompanyPhone(phone=str(phone), purpose="main"))

            emails = []
            email = item.get("email")
            if email:
                emails.append(CompanyEmail(email=str(email), purpose="general"))

            industries = []
            industry_desc = item.get("industrydesc")
            if industry_desc:
                industries.append(str(industry_desc))

            records.append(
                CompanyRecord(
                    name=str(item.get("name") or ""),
                    country_iso2="DK",
                    registration_number=str(item.get("vat") or ""),
                    status=status,
                    website=item.get("website"),
                    raw_data=item,
                    snapshot_hash=compute_hash(item),
                    locations=locations,
                    phones=phones,
                    emails=emails,
                    industries=industries,
                )
            )

        has_more = len(results) >= page_size
        next_cursor = str(offset + page_size) if has_more else None

        return CrawlResponse(
            records=records,
            has_more=has_more,
            total=-1,  # API does not return a total
            next_cursor=next_cursor,
        )
