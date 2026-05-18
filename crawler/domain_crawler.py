from __future__ import annotations

import asyncio
import base64
import re
from typing import Literal

import httpx
from crawl4ai import AsyncWebCrawler, BFSDeepCrawlStrategy, BrowserConfig, CrawlerRunConfig
from pydantic import BaseModel


class DomainCrawlRequest(BaseModel):
    domain: str
    mode: Literal["homepage", "deep"] = "deep"
    max_pages: int = 10


class CrawledPage(BaseModel):
    url: str
    title: str | None
    markdown: str
    html: str
    headers: dict[str, str]
    status_code: int
    content_type: str | None


class DomainCrawlResponse(BaseModel):
    pages: list[CrawledPage]
    total_pages: int
    favicon_url: str | None
    favicon_bytes: str | None  # base64-encoded


def _extract_favicon_url(html: str, base_url: str) -> str | None:
    """Find favicon URL from <link rel="icon"> or fall back to /favicon.ico."""
    match = re.search(
        r'<link[^>]+rel=["\'](?:shortcut )?icon["\'][^>]+href=["\']([^"\']+)["\']',
        html,
        re.IGNORECASE,
    )
    if match:
        href = match.group(1)
        if href.startswith("http"):
            return href
        from urllib.parse import urljoin
        return urljoin(base_url, href)
    # fallback to /favicon.ico
    from urllib.parse import urlparse
    parsed = urlparse(base_url)
    return f"{parsed.scheme}://{parsed.netloc}/favicon.ico"


async def _fetch_favicon(url: str) -> str | None:
    """Download favicon and return base64-encoded bytes, or None on failure."""
    try:
        async with httpx.AsyncClient(timeout=10, follow_redirects=True) as client:
            resp = await client.get(url)
            if resp.status_code == 200:
                return base64.b64encode(resp.content).decode()
    except Exception:
        pass
    return None


async def crawl_domain(req: DomainCrawlRequest) -> DomainCrawlResponse:
    browser_cfg = BrowserConfig(headless=True, verbose=False)
    start_url = f"https://{req.domain}"

    if req.mode == "homepage":
        run_cfg = CrawlerRunConfig(page_timeout=30000)
        async with AsyncWebCrawler(config=browser_cfg) as crawler:
            result = await crawler.arun(url=start_url, config=run_cfg)
        results = [result] if result.success else []
    else:
        strategy = BFSDeepCrawlStrategy(
            max_depth=3,
            max_pages=req.max_pages,
        )
        run_cfg = CrawlerRunConfig(
            deep_crawl_strategy=strategy,
            page_timeout=30000,
        )
        async with AsyncWebCrawler(config=browser_cfg) as crawler:
            results = await crawler.arun(url=start_url, config=run_cfg)
        if not isinstance(results, list):
            results = [results] if results.success else []

    pages: list[CrawledPage] = []
    favicon_url: str | None = None
    favicon_bytes: str | None = None

    for r in results:
        if not r.success:
            continue
        # Extract headers as dict[str, str]
        headers: dict[str, str] = {}
        if hasattr(r, "response_headers") and r.response_headers:
            for k, v in r.response_headers.items():
                headers[k] = str(v)
        ct = headers.get("content-type") or headers.get("Content-Type")

        title = None
        if hasattr(r, "metadata") and r.metadata:
            title = r.metadata.get("title")

        # markdown is a StringCompatibleMarkdown property; str() yields raw_markdown
        markdown_text = str(r.markdown) if r.markdown is not None else ""

        pages.append(CrawledPage(
            url=r.url,
            title=title,
            markdown=markdown_text,
            html=r.html or "",
            headers=headers,
            status_code=getattr(r, "status_code", 200) or 200,
            content_type=ct,
        ))

        # Grab favicon from first successful page
        if not favicon_url and r.html:
            favicon_url = _extract_favicon_url(r.html, r.url)

    if favicon_url:
        favicon_bytes = await _fetch_favicon(favicon_url)

    return DomainCrawlResponse(
        pages=pages,
        total_pages=len(pages),
        favicon_url=favicon_url,
        favicon_bytes=favicon_bytes,
    )
