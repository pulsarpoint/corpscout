from __future__ import annotations

from datetime import datetime
from typing import ClassVar

import httpx

from ..base import CompanyRecord, CrawlResponse, SourceAdapter, compute_hash

_USER_AGENT = "corpscout/1.0 (https://github.com/pulsarpoint/corpscout; ops@pulsarpoint.com)"

_SPARQL_TEMPLATE = """
SELECT ?company ?companyLabel ?website ?countryCode WHERE {{
  VALUES ?type {{ wd:Q4830453 wd:Q783794 wd:Q891723 }}
  ?company wdt:P31 ?type .
  OPTIONAL {{ ?company wdt:P856 ?website . }}
  OPTIONAL {{ ?company wdt:P17 ?country .
               ?country wdt:P297 ?countryCode . }}
  SERVICE wikibase:label {{ bd:serviceParam wikibase:language "en" }}
}}
ORDER BY ?company
LIMIT {limit}
OFFSET {offset}
""".strip()


class WikidataAdapter(SourceAdapter):
    source_name: ClassVar[str] = "wikidata"
    endpoint: ClassVar[str] = "https://query.wikidata.org/sparql"
    page_size: ClassVar[int] = 500

    async def crawl(
        self,
        since: datetime | None,
        cursor: str | None,
        page: int,
    ) -> CrawlResponse:
        offset = int(cursor) if cursor is not None and cursor != "" else 0
        # Request page_size + 1 so we can detect whether more pages exist
        # without paying for a COUNT query.
        query = _SPARQL_TEMPLATE.format(limit=self.page_size + 1, offset=offset)

        async with httpx.AsyncClient(timeout=60.0) as client:
            resp = await client.get(
                self.endpoint,
                params={"query": query, "format": "json"},
                headers={"User-Agent": _USER_AGENT, "Accept": "application/sparql-results+json"},
            )
            resp.raise_for_status()
            data = resp.json()

        bindings = (data.get("results") or {}).get("bindings") or []
        has_more = len(bindings) > self.page_size
        if has_more:
            bindings = bindings[: self.page_size]

        records: list[CompanyRecord] = []
        for binding in bindings:
            company_uri = (binding.get("company") or {}).get("value", "")
            qid = company_uri.rsplit("/", 1)[-1] if company_uri else ""
            label = (binding.get("companyLabel") or {}).get("value", "") or qid
            website = (binding.get("website") or {}).get("value")
            country = ((binding.get("countryCode") or {}).get("value") or "").upper()

            raw = {
                "qid": qid,
                "uri": company_uri,
                "label": label,
                "website": website,
                "country": country,
            }
            records.append(
                CompanyRecord(
                    name=label,
                    country_iso2=country,
                    website=website,
                    raw_data=raw,
                    snapshot_hash=compute_hash(raw),
                )
            )

        next_cursor = str(offset + self.page_size) if has_more else None
        # Wikidata never returns a real "total"; expose -1 to signal unknown.
        return CrawlResponse(
            records=records,
            has_more=has_more,
            total=-1,
            next_cursor=next_cursor,
        )
