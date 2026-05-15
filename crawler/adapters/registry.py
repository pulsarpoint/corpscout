from __future__ import annotations

from typing import Dict

from .base import SourceAdapter


class AdapterRegistry:
    """In-process registry of source adapters keyed by ``source_name``."""

    def __init__(self) -> None:
        self._by_name: Dict[str, SourceAdapter] = {}

    def register(self, adapter: SourceAdapter) -> None:
        name = adapter.source_name
        if name in self._by_name:
            raise ValueError(f"adapter already registered: {name}")
        self._by_name[name] = adapter

    def get(self, name: str) -> SourceAdapter | None:
        return self._by_name.get(name)

    def list_sources(self) -> list[str]:
        return sorted(self._by_name.keys())


# Module-level singleton used by main.py.
_registry = AdapterRegistry()


def register(adapter: SourceAdapter) -> None:
    _registry.register(adapter)


def get_adapter(name: str) -> SourceAdapter | None:
    return _registry.get(name)


def list_sources() -> list[str]:
    return _registry.list_sources()


def reset() -> None:
    """Drop all registrations. Used by tests."""
    _registry._by_name.clear()  # noqa: SLF001 (intentional for test reset)
