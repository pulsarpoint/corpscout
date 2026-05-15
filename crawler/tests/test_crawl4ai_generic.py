from __future__ import annotations

import pytest

from adapters.crawl4ai.generic import (
    Crawl4AIGenericAdapter,
    Crawl4AIUnconfiguredError,
)


@pytest.fixture(autouse=True)
def _clear_env(monkeypatch: pytest.MonkeyPatch) -> None:
    monkeypatch.delenv("CRAWLER_OPENAI_API_KEY", raising=False)


def test_is_configured_false_without_env() -> None:
    adapter = Crawl4AIGenericAdapter()
    assert adapter.is_configured() is False


def test_is_configured_true_with_env(monkeypatch: pytest.MonkeyPatch) -> None:
    monkeypatch.setenv("CRAWLER_OPENAI_API_KEY", "sk-x")
    adapter = Crawl4AIGenericAdapter()
    assert adapter.is_configured() is True


async def test_crawl_raises_when_unconfigured() -> None:
    adapter = Crawl4AIGenericAdapter()
    with pytest.raises(Crawl4AIUnconfiguredError):
        await adapter.crawl(since=None, cursor=None, page=1)


async def test_crawl_raises_not_implemented_when_configured(monkeypatch: pytest.MonkeyPatch) -> None:
    monkeypatch.setenv("CRAWLER_OPENAI_API_KEY", "sk-x")
    adapter = Crawl4AIGenericAdapter()
    with pytest.raises(NotImplementedError):
        await adapter.crawl(since=None, cursor=None, page=1)
