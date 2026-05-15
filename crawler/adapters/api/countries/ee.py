from __future__ import annotations

import asyncio
import csv
import io
import zipfile
from datetime import datetime
from typing import ClassVar

import httpx

from ...base import CompanyLocation, CompanyRecord, CrawlResponse, SourceAdapter, compute_hash

_USER_AGENT = "corpscout/1.0 (https://github.com/pulsarpoint/corpscout; ops@pulsarpoint.com)"

# Status codes in the Estonian business register CSV export.
_STATUS_MAP = {
    "R": "active",      # Registered
    "K": "dissolved",   # Kustutatud (struck off)
    "L": "inactive",    # Likvideeritav (in liquidation)
    "N": "inactive",
}


def _get_field(row: dict, *keys: str) -> str:
    """Return the first non-empty value matching any of the given column names."""
    for key in keys:
        v = (row.get(key) or "").strip()
        if v:
            return v
    return ""


def _parse_csv_zip(raw: bytes) -> CrawlResponse:
    with zipfile.ZipFile(io.BytesIO(raw)) as zf:
        csv_names = [f for f in zf.namelist() if f.lower().endswith(".csv")]
        if not csv_names:
            return CrawlResponse(records=[], has_more=False, total=0)
        csv_bytes = zf.read(csv_names[0])

    # Try UTF-8-BOM (used by many Estonian government exports), then latin-1.
    for encoding in ("utf-8-sig", "latin-1"):
        try:
            text = csv_bytes.decode(encoding)
            break
        except (UnicodeDecodeError, ValueError):
            continue
    else:
        text = csv_bytes.decode("latin-1", errors="replace")

    # Detect delimiter: Estonian government CSVs commonly use semicolons.
    first_line = text.split("\n")[0] if "\n" in text else text
    delimiter = ";" if first_line.count(";") >= first_line.count(",") else ","

    reader = csv.DictReader(io.StringIO(text), delimiter=delimiter)
    records: list[CompanyRecord] = []
    for row in reader:
        # Support both Estonian (ärinimi) and transliterated (ariregistri_kood) column names.
        name = _get_field(row, "ärinimi", "Ärinimi", "arinimi", "nimi", "evnimi")
        reg_nr = _get_field(row, "registrikood", "Registrikood", "ariregistri_kood") or None
        raw_status = _get_field(row, "staatus", "Staatus").upper()
        status = _STATUS_MAP.get(raw_status[:1] if raw_status else "", "active")

        if not name:
            continue

        rec = dict(row)

        locations = []
        street = _get_field(rec, "tänav, maja ja korteri nr", "tänav", "aadress", "Aadress")
        city = _get_field(rec, "linn/vald", "linn", "vald", "asustusüksus")
        postal = _get_field(rec, "postiindeks", "Postiindeks", "indeks")
        county = _get_field(rec, "maakond", "Maakond")
        if street or city:
            locations.append(CompanyLocation(
                location_type="registered_address",
                address_line1=street or None,
                city=city or None,
                region=county or None,
                postal_code=postal or None,
                country="Estonia",
                country_code="EE",
            ))

        industries = []
        emtak = _get_field(rec, "põhitegevusala tekst", "emtak2008_tekstina", "emtak_tekst", "tegevusala")
        if emtak:
            industries.append(emtak)

        records.append(
            CompanyRecord(
                name=name,
                country_iso2="EE",
                registration_number=reg_nr,
                status=status,
                raw_data=rec,
                snapshot_hash=compute_hash(rec),
                locations=locations,
                industries=industries,
            )
        )

    return CrawlResponse(records=records, has_more=False, total=len(records))


class EstoniaAdapter(SourceAdapter):
    """Fetches Estonian company records from the Business Register bulk CSV download.

    The register publishes a daily-refreshed open-data ZIP at a stable public URL
    with no authentication required. Since/cursor/page are ignored — every crawl
    returns the full current dataset as a single response.
    """

    source_name: ClassVar[str] = "ariregister"
    data_url: ClassVar[str] = (
        "https://avaandmed.ariregister.rik.ee/sites/default/files/avaandmed/"
        "ettevotja_rekvisiidid__lihtandmed.csv.zip"
    )

    async def crawl(
        self,
        since: datetime | None,
        cursor: str | None,
        page: int,
    ) -> CrawlResponse:
        async with httpx.AsyncClient(timeout=180.0, follow_redirects=True) as client:
            resp = await client.get(
                self.data_url,
                headers={"User-Agent": _USER_AGENT},
            )
            resp.raise_for_status()

        return await asyncio.to_thread(_parse_csv_zip, resp.content)
