// Package crawlerclient provides a typed HTTP client for the Python crawler service.
package crawlerclient

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/cockroachdb/errors"
)

// CompanyLocation mirrors the Python CompanyLocation model.
type CompanyLocation struct {
	LocationType string  `json:"location_type"`
	AddressLine1 *string `json:"address_line1,omitempty"`
	AddressLine2 *string `json:"address_line2,omitempty"`
	City         *string `json:"city,omitempty"`
	Region       *string `json:"region,omitempty"`
	PostalCode   *string `json:"postal_code,omitempty"`
	Country      *string `json:"country,omitempty"`
	CountryCode  *string `json:"country_code,omitempty"`
	Source       string  `json:"source"`
}

// CompanyPhone mirrors the Python CompanyPhone model.
type CompanyPhone struct {
	Phone   string `json:"phone"`
	Purpose string `json:"purpose"`
	Source  string `json:"source"`
}

// CompanyEmail mirrors the Python CompanyEmail model.
type CompanyEmail struct {
	Email   string `json:"email"`
	Purpose string `json:"purpose"`
	Source  string `json:"source"`
}

// CompanyRecord mirrors the Python CompanyRecord data model returned by the crawler.
type CompanyRecord struct {
	Name               string           `json:"name"`
	CountryISO2        string           `json:"country_iso2"`
	RegistrationNumber *string          `json:"registration_number,omitempty"`
	LEI                *string          `json:"lei,omitempty"`
	Status             string           `json:"status"`
	Website            *string          `json:"website,omitempty"`
	Aliases            []string         `json:"aliases,omitempty"`
	RawData            map[string]any   `json:"raw_data,omitempty"`
	SnapshotHash       string           `json:"snapshot_hash"`
	Locations          []CompanyLocation `json:"locations,omitempty"`
	Phones             []CompanyPhone    `json:"phones,omitempty"`
	Emails             []CompanyEmail    `json:"emails,omitempty"`
	Industries         []string          `json:"industries,omitempty"`
	FoundedYear        *int32            `json:"founded_year,omitempty"`
	EmployeeEstimate   map[string]any    `json:"employee_estimate,omitempty"`
}

// CrawlResponse is returned by POST /crawl/{source_name}.
type CrawlResponse struct {
	Records    []CompanyRecord `json:"records"`
	HasMore    bool            `json:"has_more"`
	Total      int             `json:"total"`
	NextCursor *string         `json:"next_cursor,omitempty"`
}

// DomainCandidate is a single domain resolution candidate returned by the crawler.
type DomainCandidate struct {
	Domain     string                 `json:"domain"`
	Signal     string                 `json:"signal"`
	Confidence int                    `json:"confidence"`
	Evidence   map[string]any `json:"evidence,omitempty"`
}

// ResolveResponse is returned by POST /resolve/domain.
type ResolveResponse struct {
	Candidates []DomainCandidate `json:"candidates"`
}

// Client is a typed HTTP client for the Python crawler service.
type Client struct {
	baseURL string
	http    *http.Client
}

// New creates a new Client with a 120-second timeout.
func New(baseURL string) *Client {
	return &Client{
		baseURL: baseURL,
		http: &http.Client{
			Timeout: 120 * time.Second,
		},
	}
}

// BaseURL returns the base URL the client is configured to use.
func (c *Client) BaseURL() string {
	return c.baseURL
}

// Crawl calls POST /crawl/{source} to fetch a page of company records.
// since is formatted as RFC3339 UTC and omitted if zero. cursor is omitted if nil.
func (c *Client) Crawl(ctx context.Context, source string, since time.Time, cursor *string, page int) (*CrawlResponse, error) {
	const context = "crawler POST /crawl/"

	body := map[string]any{
		"page": page,
	}
	if !since.IsZero() {
		body["since"] = since.UTC().Format(time.RFC3339)
	}
	if cursor != nil {
		body["cursor"] = *cursor
	}

	path := fmt.Sprintf("/crawl/%s", source)
	var result CrawlResponse
	if err := c.postJSON(ctx, path, body, &result); err != nil {
		return nil, errors.Wrap(err, context+source)
	}
	return &result, nil
}

// ResolveDomain calls POST /resolve/domain to resolve candidate domains for a company.
// lei is omitted from the request body if it is an empty string.
func (c *Client) ResolveDomain(ctx context.Context, companyName, lei, country string) (*ResolveResponse, error) {
	const context = "crawler POST /resolve/domain"

	body := map[string]any{
		"company_name": companyName,
		"country":      country,
	}
	if lei != "" {
		body["lei"] = lei
	}

	var result ResolveResponse
	if err := c.postJSON(ctx, "/resolve/domain", body, &result); err != nil {
		return nil, errors.Wrap(err, context)
	}
	return &result, nil
}

// postJSON sends a POST request with a JSON body and decodes the JSON response into dest.
// On non-200 status codes it returns an error containing the status code and response body.
func (c *Client) postJSON(ctx context.Context, path string, body any, dest any) error {
	encoded, err := json.Marshal(body)
	if err != nil {
		return errors.Wrap(err, "marshal request body")
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+path, bytes.NewReader(encoded))
	if err != nil {
		return errors.Wrap(err, "build request")
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return errors.Wrap(err, "execute request")
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return errors.Newf("non-200 response: status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	if err := json.NewDecoder(resp.Body).Decode(dest); err != nil {
		return errors.Wrap(err, "decode response")
	}
	return nil
}
