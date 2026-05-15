package crawlerclient_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pulsarpoint/corpscout/scheduler/internal/crawlerclient"
)

func TestCrawl_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "/crawl/test_source", r.URL.Path)

		var body map[string]any
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		assert.Equal(t, float64(1), body["page"])
		// since and cursor should be present when provided
		assert.Contains(t, body, "since")
		assert.Contains(t, body, "cursor")

		resp := crawlerclient.CrawlResponse{
			Records: []crawlerclient.CompanyRecord{
				{
					Name:               "Acme Corp",
					CountryISO2:        "US",
					Status:             "active",
					SnapshotHash:       "abc123",
					Aliases:            []string{"Acme"},
					RawData:            map[string]any{"source_id": "123"},
				},
			},
			HasMore:    false,
			Total:      1,
			NextCursor: nil,
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	client := crawlerclient.New(srv.URL)
	assert.Equal(t, srv.URL, client.BaseURL())

	cursor := "some_cursor"
	since := time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC)
	result, err := client.Crawl(context.Background(), "test_source", since, &cursor, 1)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Len(t, result.Records, 1)
	assert.Equal(t, "Acme Corp", result.Records[0].Name)
	assert.Equal(t, "US", result.Records[0].CountryISO2)
	assert.Equal(t, "active", result.Records[0].Status)
	assert.Equal(t, "abc123", result.Records[0].SnapshotHash)
	assert.Equal(t, []string{"Acme"}, result.Records[0].Aliases)
	assert.False(t, result.HasMore)
	assert.Equal(t, 1, result.Total)
	assert.Nil(t, result.NextCursor)
}

func TestCrawl_OmitsSinceAndCursorWhenZeroNil(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		// since should be absent when zero time
		assert.NotContains(t, body, "since")
		// cursor should be absent when nil
		assert.NotContains(t, body, "cursor")

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(crawlerclient.CrawlResponse{})
	}))
	defer srv.Close()

	client := crawlerclient.New(srv.URL)
	_, err := client.Crawl(context.Background(), "test_source", time.Time{}, nil, 2)
	require.NoError(t, err)
}

func TestCrawl_NonOKReturnsWrappedError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "internal crawler error", http.StatusInternalServerError)
	}))
	defer srv.Close()

	client := crawlerclient.New(srv.URL)
	result, err := client.Crawl(context.Background(), "bad_source", time.Time{}, nil, 1)
	assert.Nil(t, result)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "crawler POST /crawl/bad_source")
	assert.Contains(t, err.Error(), "500")
}

func TestCrawl_404ReturnsWrappedError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "not found", http.StatusNotFound)
	}))
	defer srv.Close()

	client := crawlerclient.New(srv.URL)
	result, err := client.Crawl(context.Background(), "missing_source", time.Time{}, nil, 1)
	assert.Nil(t, result)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "404")
}

func TestResolveDomain_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "/resolve/domain", r.URL.Path)

		var body map[string]any
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		assert.Equal(t, "Acme Corp", body["company_name"])
		assert.Equal(t, "US", body["country"])
		assert.Equal(t, "LEI12345", body["lei"])

		resp := crawlerclient.ResolveResponse{
			Candidates: []crawlerclient.DomainCandidate{
				{
					Domain:     "acme.com",
					Signal:     "homepage",
					Confidence: 90,
					Evidence:   map[string]any{"matched": "acme corp"},
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	client := crawlerclient.New(srv.URL)
	result, err := client.ResolveDomain(context.Background(), "Acme Corp", "LEI12345", "US")
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Len(t, result.Candidates, 1)
	assert.Equal(t, "acme.com", result.Candidates[0].Domain)
	assert.Equal(t, "homepage", result.Candidates[0].Signal)
	assert.Equal(t, 90, result.Candidates[0].Confidence)
}

func TestResolveDomain_OmitsLEIWhenEmpty(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		// lei should be absent when empty string
		assert.NotContains(t, body, "lei")
		assert.Equal(t, "Widget Inc", body["company_name"])
		assert.Equal(t, "GB", body["country"])

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(crawlerclient.ResolveResponse{Candidates: []crawlerclient.DomainCandidate{}})
	}))
	defer srv.Close()

	client := crawlerclient.New(srv.URL)
	result, err := client.ResolveDomain(context.Background(), "Widget Inc", "", "GB")
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Empty(t, result.Candidates)
}

func TestResolveDomain_NonOKReturnsWrappedError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "bad gateway", http.StatusBadGateway)
	}))
	defer srv.Close()

	client := crawlerclient.New(srv.URL)
	result, err := client.ResolveDomain(context.Background(), "Foo", "", "DE")
	assert.Nil(t, result)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "crawler POST /resolve/domain")
	assert.Contains(t, err.Error(), "502")
}
