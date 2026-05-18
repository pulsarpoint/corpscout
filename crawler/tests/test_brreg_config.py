from __future__ import annotations

import httpx
import respx

from adapters.api.countries.no import BrregAdapter

_BRREG_URL = "https://data.brreg.no/enhetsregisteret/api/enheter"
_CUSTOM_URL = "https://custom.example.com/brreg-api"

FIXTURE_COMPANY = {
    "organisasjonsnummer": "987654321",
    "navn": "Test Norsk AS",
    "registreringsdatoEnhetsregisteret": "2019-03-10",
    "organisasjonsform": {"kode": "AS"},
    "naeringskode1": {"beskrivelse": "Software"},
    "forretningsadresse": {
        "adresse": ["Testveien 1"],
        "postnummer": "0150",
        "poststed": "Oslo",
        "landkode": "NO",
    },
    "antallAnsatte": 5,
    "hjemmeside": "https://testnorsk.no",
    "konkurs": False,
    "underAvvikling": False,
}
API_RESPONSE = {
    "_embedded": {"enheter": [FIXTURE_COMPANY]},
    "page": {"totalElements": 1, "totalPages": 1, "size": 200, "number": 0},
}


@respx.mock
async def test_crawl_with_db_config_returns_records(brreg_config: dict) -> None:
    respx.get(_BRREG_URL).mock(return_value=httpx.Response(200, json=API_RESPONSE))

    adapter = BrregAdapter()
    resp = await adapter.crawl(since=None, cursor=None, page=1, config=brreg_config)

    assert len(resp.records) >= 1
    rec = resp.records[0]
    assert rec.country_iso2 == "NO"
    assert rec.name != ""


@respx.mock
async def test_crawl_config_overrides_hardcoded_url(brreg_config: dict) -> None:
    custom_config = {**brreg_config, "api_url": _CUSTOM_URL}

    custom_route = respx.get(_CUSTOM_URL).mock(return_value=httpx.Response(200, json=API_RESPONSE))
    default_route = respx.get(_BRREG_URL).mock(return_value=httpx.Response(200, json=API_RESPONSE))

    adapter = BrregAdapter()
    await adapter.crawl(since=None, cursor=None, page=1, config=custom_config)

    assert custom_route.called
    assert not default_route.called
