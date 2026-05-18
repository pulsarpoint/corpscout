from __future__ import annotations

import io
import zipfile

import httpx
import respx

from adapters.api.countries.ee import EstoniaAdapter

_DATA_URL = (
    "https://avaandmed.ariregister.rik.ee/sites/default/files/avaandmed/"
    "ettevotja_rekvisiidid__lihtandmed.csv.zip"
)
_CUSTOM_URL = "https://custom.example.com/ariregister.zip"

CSV_CONTENT = (
    "ariregistri_kood;nimi;asukoha_ettevotja_aadress;tegevusala_emtak_tekst;staatus\n"
    "12345678;Test OÜ;Testmnt 1 Tallinn;Software;R\n"
)


def _make_zip(csv_content: str) -> bytes:
    buf = io.BytesIO()
    with zipfile.ZipFile(buf, "w", zipfile.ZIP_DEFLATED) as zf:
        zf.writestr("companies.csv", csv_content)
    return buf.getvalue()


@respx.mock
async def test_crawl_with_db_config_returns_records(ariregister_config: dict) -> None:
    respx.get(_DATA_URL).mock(return_value=httpx.Response(200, content=_make_zip(CSV_CONTENT)))

    adapter = EstoniaAdapter()
    resp = await adapter.crawl(since=None, cursor=None, page=1, config=ariregister_config)

    assert len(resp.records) >= 1
    rec = resp.records[0]
    assert rec.country_iso2 == "EE"
    assert rec.name != ""


@respx.mock
async def test_crawl_config_overrides_hardcoded_url(ariregister_config: dict) -> None:
    custom_config = {**ariregister_config, "api_url": _CUSTOM_URL}

    custom_route = respx.get(_CUSTOM_URL).mock(
        return_value=httpx.Response(200, content=_make_zip(CSV_CONTENT))
    )
    default_route = respx.get(_DATA_URL).mock(
        return_value=httpx.Response(200, content=_make_zip(CSV_CONTENT))
    )

    adapter = EstoniaAdapter()
    await adapter.crawl(since=None, cursor=None, page=1, config=custom_config)

    assert custom_route.called
    assert not default_route.called
