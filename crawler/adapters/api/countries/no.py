from __future__ import annotations

from datetime import datetime
from typing import Any, ClassVar

import httpx

from ...base import CompanyLocation, CompanyRecord, CrawlResponse, SourceAdapter, compute_hash

_USER_AGENT = "corpscout/1.0 (https://github.com/pulsarpoint/corpscout; ops@pulsarpoint.com)"


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
        effective_page = int(cursor) if cursor else max(page, 1)
        zero_page = max(effective_page - 1, 0)
        params: dict[str, Any] = {
            "page": str(zero_page),
            "size": str(self.page_size),
            "sort": "registreringsdatoEnhetsregisteret,asc",
        }

        async with httpx.AsyncClient(timeout=30.0) as client:
            resp = await client.get(self.endpoint, params=params, headers={"Accept": "application/json", "User-Agent": _USER_AGENT})
            resp.raise_for_status()
            data = resp.json()

        embedded = (data.get("_embedded") or {}).get("enheter") or []
        records: list[CompanyRecord] = []
        for item in embedded:
            konkurs = bool(item.get("konkurs"))
            under_avvikling = bool(item.get("underAvvikling"))
            status = "dissolved" if (konkurs or under_avvikling) else "active"

            locations = []
            addr = item.get("forretningsadresse") or {}
            if addr:
                addr_lines = addr.get("adresse") or []
                locations.append(CompanyLocation(
                    location_type="registered_address",
                    address_line1=addr_lines[0] if addr_lines else None,
                    address_line2=addr_lines[1] if len(addr_lines) > 1 else None,
                    city=addr.get("poststed"),
                    postal_code=str(addr.get("postnummer")) if addr.get("postnummer") else None,
                    country="Norway",
                    country_code=addr.get("landkode") or "NO",
                ))

            industries = []
            for code_key in ("naeringskode1", "naeringskode2", "naeringskode3"):
                code = item.get(code_key) or {}
                desc = code.get("beskrivelse")
                if desc:
                    industries.append(desc)

            founded_year: int | None = None
            stiftelse = item.get("stiftelsesdato")
            if stiftelse:
                try:
                    founded_year = int(str(stiftelse)[:4])
                except (ValueError, TypeError):
                    pass

            ansatte = item.get("antallAnsatte")
            employee_estimate = {"count": int(ansatte)} if ansatte is not None else {}

            records.append(
                CompanyRecord(
                    name=str(item.get("navn") or ""),
                    country_iso2="NO",
                    registration_number=str(item.get("organisasjonsnummer") or ""),
                    status=status,
                    website=item.get("hjemmeside"),
                    raw_data=item,
                    snapshot_hash=compute_hash(item),
                    locations=locations,
                    industries=industries,
                    founded_year=founded_year,
                    employee_estimate=employee_estimate,
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
