from __future__ import annotations

import os
from datetime import datetime
from typing import ClassVar

from ..base import CrawlResponse, SourceAdapter


class Crawl4AIUnconfiguredError(RuntimeError):
    """Raised when Crawl4AIGenericAdapter is invoked without an API key."""


class Crawl4AIGenericAdapter(SourceAdapter):
    """Generic LLM-driven scraper.

    The real implementation lives in Plan 3; here we only expose the
    surface so main.py can register it and reject calls cleanly when no
    key is configured.
    """

    source_name: ClassVar[str] = "crawl4ai_generic"
    env_var: ClassVar[str] = "CRAWLER_OPENAI_API_KEY"

    def is_configured(self) -> bool:
        return bool(os.getenv(self.env_var))

    async def crawl(
        self,
        since: datetime | None,
        cursor: str | None,
        page: int,
    ) -> CrawlResponse:
        if not self.is_configured():
            raise Crawl4AIUnconfiguredError(
                f"crawl4ai generic adapter requires {self.env_var}"
            )
        raise NotImplementedError("crawl4ai generic scraping lands in Plan 3")
