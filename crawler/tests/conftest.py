import sys
import os
sys.path.insert(0, os.path.join(os.path.dirname(__file__), ".."))

import pytest


@pytest.fixture
def companies_house_config() -> dict:
    return {
        "api_url": "https://api.company-information.service.gov.uk/advanced-search/companies",
        "page_size": 100,
        "auth_env": "COMPANIES_HOUSE_API_KEY",
    }


@pytest.fixture
def brreg_config() -> dict:
    return {
        "api_url": "https://data.brreg.no/enhetsregisteret/api/enheter",
        "page_size": 200,
        "auth_env": None,
    }


@pytest.fixture
def cvr_config() -> dict:
    return {
        "api_url": "https://cvrapi.dk/api",
        "page_size": 100,
        "auth_env": "CVR_API_TOKEN",
    }


@pytest.fixture
def ariregister_config() -> dict:
    return {
        "api_url": (
            "https://avaandmed.ariregister.rik.ee/sites/default/files/avaandmed/"
            "ettevotja_rekvisiidid__lihtandmed.csv.zip"
        ),
        "page_size": None,
        "auth_env": None,
    }
