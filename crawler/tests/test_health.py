import pytest
from fastapi.testclient import TestClient
from main import app

client = TestClient(app)


def test_health_returns_ok():
    response = client.get("/health")
    assert response.status_code == 200
    assert response.json() == {"status": "ok"}


def test_crawl_unknown_source_returns_404():
    response = client.post("/crawl/nonexistent", json={
        "since": "2026-01-01T00:00:00Z",
        "page": 1,
    })
    assert response.status_code == 404


def test_resolve_domain_returns_empty_candidates():
    response = client.post("/resolve/domain", json={
        "company_name": "Test Corp",
        "country": "GB",
    })
    assert response.status_code == 200
    assert response.json()["candidates"] == []
