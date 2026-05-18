from __future__ import annotations

from datetime import datetime
from typing import Any, ClassVar

import httpx

from ...base import CompanyLocation, CompanyRecord, CrawlResponse, SourceAdapter, compute_hash

_USER_AGENT = "corpscout/1.0 (https://github.com/pulsarpoint/corpscout; ops@pulsarpoint.com)"


# Brreg API rejects size*(page+1) > 10_000. With size=200 that means page 0-49 max.
# We use date-cursor pagination: cursor = "YYYY-MM-DD,<page_offset>" so we can
# restart the page counter for each date bucket and cover all 1.1M records.
_BRREG_MAX_PAGE = 49  # 200 * (49+1) = 10_000 — safe upper bound


class BrregAdapter(SourceAdapter):
    source_name: ClassVar[str] = "brreg"
    endpoint: ClassVar[str] = "https://data.brreg.no/enhetsregisteret/api/enheter"
    page_size: ClassVar[int] = 200

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

        # cursor = "YYYY-MM-DD,N" (date bucket + 0-indexed page within bucket)
        # or None (start from beginning)
        # Legacy integer cursors (page numbers) are reset to the beginning.
        date_cursor: str | None = None
        page_offset: int = 0

        if cursor and "," in cursor:
            parts = cursor.split(",", 1)
            date_cursor = parts[0] or None
            try:
                page_offset = int(parts[1])
            except ValueError:
                page_offset = 0
        # else: legacy int cursor or None → start from page 0 with no date filter

        params: dict[str, Any] = {
            "page": str(page_offset),
            "size": str(page_size),
            "sort": "registreringsdatoEnhetsregisteret,asc",
        }
        if date_cursor:
            params["fraRegistreringsdatoEnhetsregisteret"] = date_cursor

        async with httpx.AsyncClient(timeout=30.0) as client:
            resp = await client.get(api_url, params=params, headers={"Accept": "application/json", "User-Agent": _USER_AGENT})
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
        current = int(page_info.get("number") or page_offset)

        has_more = (current + 1) < total_pages
        next_cursor: str | None = None
        if has_more:
            if current < _BRREG_MAX_PAGE:
                # Stay in the same date bucket, advance page
                next_cursor = f"{date_cursor or ''},{current + 1}"
            else:
                # At page limit — advance date bucket to last record's registration date
                last_date = (embedded[-1].get("registreringsdatoEnhetsregisteret") or "") if embedded else ""
                next_cursor = f"{last_date},0"

        return CrawlResponse(
            records=records,
            has_more=has_more,
            total=total,
            next_cursor=next_cursor,
        )
