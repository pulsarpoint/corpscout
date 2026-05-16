from __future__ import annotations

from datetime import datetime
from typing import Any, ClassVar
from urllib.parse import parse_qs, urlparse

import httpx

from ..base import CompanyLocation, CompanyRecord, CrawlResponse, SourceAdapter, compute_hash

_USER_AGENT = "corpscout/1.0 (https://github.com/pulsarpoint/corpscout; ops@pulsarpoint.com)"

_STATUS_MAP = {
    "ACTIVE": "active",
    "INACTIVE": "inactive",
    "ANNULLED": "dissolved",
}


def _extract_cursor_from_url(url: str) -> str | None:
    """Extract page[cursor] value from a GLEIF pagination URL."""
    try:
        qs = parse_qs(urlparse(url).query)
        values = qs.get("page[cursor]") or qs.get("page%5Bcursor%5D")
        return values[0] if values else None
    except Exception:
        return None


class GLEIFAdapter(SourceAdapter):
    source_name: ClassVar[str] = "gleif"
    base_url: ClassVar[str] = "https://api.gleif.org/api/v1/lei-records"
    page_size: ClassVar[int] = 200

    async def crawl(
        self,
        since: datetime | None,
        cursor: str | None,
        page: int,
    ) -> CrawlResponse:
        # GLEIF only supports cursor-based pagination (page-based stops at 10,000 results).
        # cursor=None or a legacy page-number string → start from the beginning with cursor=*.
        # cursor=<opaque string> → continue from that cursor.
        params: dict[str, Any] = {"page[size]": str(self.page_size)}
        if cursor and not cursor.isdigit():
            params["page[cursor]"] = cursor
        else:
            params["page[cursor]"] = "*"

        async with httpx.AsyncClient(timeout=30.0) as client:
            resp = await client.get(self.base_url, params=params, headers={"Accept": "application/json", "User-Agent": _USER_AGENT})
            resp.raise_for_status()
            data = resp.json()

        records: list[CompanyRecord] = []
        for item in data.get("data", []):
            attrs = item.get("attributes", {}) or {}
            entity = attrs.get("entity", {}) or {}
            legal_name = (entity.get("legalName") or {}).get("name") or ""
            raw_status = (entity.get("status") or "").upper()
            status = _STATUS_MAP.get(raw_status, "active")
            lei = item.get("id")

            locations = []
            legal_addr = entity.get("legalAddress") or {}
            country = (legal_addr.get("country") or "").upper()
            if legal_addr:
                addr_lines = legal_addr.get("addressLines") or []
                street_name = legal_addr.get("streetName") or ""
                street_num = legal_addr.get("streetNumber") or ""
                line1 = f"{street_name} {street_num}".strip() or (addr_lines[0] if addr_lines else None)
                locations.append(CompanyLocation(
                    location_type="registered_address",
                    address_line1=line1 or None,
                    city=legal_addr.get("city"),
                    region=legal_addr.get("region"),
                    postal_code=legal_addr.get("postalCode"),
                    country=legal_addr.get("country"),
                    country_code=country or None,
                ))

            hq_addr = entity.get("headquartersAddress") or {}
            if hq_addr and hq_addr != legal_addr:
                hq_lines = hq_addr.get("addressLines") or []
                hq_street = hq_addr.get("streetName") or ""
                hq_num = hq_addr.get("streetNumber") or ""
                hq_line1 = f"{hq_street} {hq_num}".strip() or (hq_lines[0] if hq_lines else None)
                hq_country = (hq_addr.get("country") or "").upper()
                locations.append(CompanyLocation(
                    location_type="headquarters",
                    address_line1=hq_line1 or None,
                    city=hq_addr.get("city"),
                    region=hq_addr.get("region"),
                    postal_code=hq_addr.get("postalCode"),
                    country=hq_addr.get("country"),
                    country_code=hq_country or None,
                ))

            # GLEIF otherNames can supplement aliases
            aliases = []
            for other_name in (entity.get("otherNames") or []):
                n = (other_name.get("name") or "").strip()
                if n and n != legal_name:
                    aliases.append(n)

            records.append(
                CompanyRecord(
                    name=legal_name,
                    country_iso2=country,
                    lei=lei,
                    status=status,
                    aliases=aliases,
                    raw_data=item,
                    snapshot_hash=compute_hash(item),
                    locations=locations,
                )
            )

        links = data.get("links") or {}
        next_url = links.get("next")
        has_more = bool(next_url)
        next_cursor = _extract_cursor_from_url(next_url) if next_url else None

        pagination = (data.get("meta") or {}).get("pagination") or {}
        total = int(pagination.get("total") or 0)

        return CrawlResponse(
            records=records,
            has_more=has_more,
            total=total,
            next_cursor=next_cursor,
        )
