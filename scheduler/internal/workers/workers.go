// Package workers contains River job worker implementations.
package workers

import "time"

// SourceCrawlArgs is the job argument for a source crawl task.
type SourceCrawlArgs struct {
	SourceName string    `json:"source_name"`
	Since      time.Time `json:"since"`
}

func (SourceCrawlArgs) Kind() string { return "source_crawl" }

// DomainResolveArgs is the job argument for a domain resolution task.
type DomainResolveArgs struct {
	CompanyID string `json:"company_id"`
}

func (DomainResolveArgs) Kind() string { return "domain_resolve" }
