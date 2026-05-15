from __future__ import annotations

import io
import zipfile

import httpx
import pytest
import respx

from adapters.api.countries.ee import EstoniaAdapter, _parse_csv_zip


def _make_zip(csv_content: str) -> bytes:
    """Build an in-memory ZIP containing a single companies.csv file."""
    buf = io.BytesIO()
    with zipfile.ZipFile(buf, "w", zipfile.ZIP_DEFLATED) as zf:
        zf.writestr("companies.csv", csv_content)
    return buf.getvalue()


_DATA_URL = (
    "https://avaandmed.ariregister.rik.ee/sites/default/files/avaandmed/"
    "ettevotja_rekvisiidid__lihtandmed.csv.zip"
)


@pytest.fixture
def adapter() -> EstoniaAdapter:
    return EstoniaAdapter()


# ---------------------------------------------------------------------------
# Unit tests for the CSV parser
# ---------------------------------------------------------------------------


def test_parse_csv_zip_semicolon_delimiter() -> None:
    csv = "ärinimi;registrikood;staatus\nCompany OÜ;12345678;R\n"
    result = _parse_csv_zip(_make_zip(csv))
    assert len(result.records) == 1
    rec = result.records[0]
    assert rec.name == "Company OÜ"
    assert rec.registration_number == "12345678"
    assert rec.country_iso2 == "EE"
    assert rec.status == "active"
    assert result.has_more is False
    assert result.total == 1


def test_parse_csv_zip_comma_delimiter() -> None:
    csv = "ärinimi,registrikood,staatus\nTest AS,98765432,K\n"
    result = _parse_csv_zip(_make_zip(csv))
    assert result.records[0].status == "dissolved"


def test_parse_status_map() -> None:
    cases = {"R": "active", "K": "dissolved", "L": "inactive", "X": "active"}
    for code, want in cases.items():
        csv = f"ärinimi;registrikood;staatus\nFirma OÜ;1;{code}\n"
        result = _parse_csv_zip(_make_zip(csv))
        assert result.records[0].status == want, f"status code {code!r}"


def test_parse_skips_empty_names() -> None:
    csv = "ärinimi;registrikood;staatus\n;12345;R\nValid OÜ;99999;R\n"
    result = _parse_csv_zip(_make_zip(csv))
    assert len(result.records) == 1
    assert result.records[0].name == "Valid OÜ"


def test_parse_empty_zip_returns_empty() -> None:
    buf = io.BytesIO()
    with zipfile.ZipFile(buf, "w"):
        pass
    result = _parse_csv_zip(buf.getvalue())
    assert result.records == []
    assert result.has_more is False


def test_parse_snapshot_hash_is_stable() -> None:
    csv = "ärinimi;registrikood;staatus\nFoo OÜ;111;R\n"
    r1 = _parse_csv_zip(_make_zip(csv))
    r2 = _parse_csv_zip(_make_zip(csv))
    assert r1.records[0].snapshot_hash == r2.records[0].snapshot_hash


# ---------------------------------------------------------------------------
# Integration-style tests for the adapter (mocked HTTP)
# ---------------------------------------------------------------------------


@respx.mock
async def test_adapter_fetches_data_url(adapter: EstoniaAdapter) -> None:
    csv = "ärinimi;registrikood;staatus\nEesti OÜ;10000001;R\n"
    route = respx.get(_DATA_URL).mock(return_value=httpx.Response(200, content=_make_zip(csv)))

    resp = await adapter.crawl(since=None, cursor=None, page=1)

    assert route.called
    assert len(resp.records) == 1
    assert resp.records[0].name == "Eesti OÜ"
    assert resp.has_more is False


@respx.mock
async def test_adapter_ignores_cursor_and_page(adapter: EstoniaAdapter) -> None:
    """Cursor and page do not affect the request — bulk download is always fresh."""
    csv = "ärinimi;registrikood;staatus\nOÜ Test;20000001;R\n"
    route = respx.get(_DATA_URL).mock(return_value=httpx.Response(200, content=_make_zip(csv)))

    for page, cursor in [(1, None), (2, "400"), (3, "some_cursor")]:
        resp = await adapter.crawl(since=None, cursor=cursor, page=page)
        assert resp.has_more is False

    assert route.call_count == 3


@respx.mock
async def test_adapter_propagates_http_error(adapter: EstoniaAdapter) -> None:
    respx.get(_DATA_URL).mock(return_value=httpx.Response(503))

    with pytest.raises(httpx.HTTPStatusError):
        await adapter.crawl(since=None, cursor=None, page=1)
