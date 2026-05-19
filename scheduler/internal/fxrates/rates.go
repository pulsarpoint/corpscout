package fxrates

import (
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"math"
	"net/http"
)

const ecbURL = "https://www.ecb.europa.eu/stats/eurofxref/eurofxref-daily.xml"

type Rates struct {
	// eurPer maps currency code → how many of that currency equal 1 EUR
	// e.g. eurPer["USD"] = 1.09 means 1 EUR = 1.09 USD
	eurPer map[string]float64
}

// Load fetches rates from the ECB daily feed.
func Load(ctx context.Context) (*Rates, error) {
	return LoadFrom(ctx, ecbURL)
}

// LoadFrom fetches rates from the given URL (for testing).
func LoadFrom(ctx context.Context, url string) (*Rates, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch ecb: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("ecb returned %d", resp.StatusCode)
	}
	return parse(resp.Body)
}

type envelope struct {
	Cube outerCube `xml:"Cube"`
}

type outerCube struct {
	Daily dailyCube `xml:"Cube"`
}

type dailyCube struct {
	Rates []rateCube `xml:"Cube"`
}

type rateCube struct {
	Currency string  `xml:"currency,attr"`
	Rate     float64 `xml:"rate,attr"`
}

func parse(r io.Reader) (*Rates, error) {
	var env envelope
	if err := xml.NewDecoder(r).Decode(&env); err != nil {
		return nil, fmt.Errorf("parse ecb xml: %w", err)
	}
	m := make(map[string]float64, len(env.Cube.Daily.Rates)+1)
	m["EUR"] = 1.0
	for _, rc := range env.Cube.Daily.Rates {
		m[rc.Currency] = rc.Rate
	}
	return &Rates{eurPer: m}, nil
}

// ToUSD converts amount (in the smallest unit of currency, e.g. cents/øre) to
// USD cents. Both input and output are integer cent-scale values.
func (r *Rates) ToUSD(amount int64, currency string) (int64, error) {
	if currency == "USD" {
		return amount, nil
	}
	usdRate, ok := r.eurPer["USD"]
	if !ok {
		return 0, fmt.Errorf("USD rate not found in ECB feed")
	}
	srcRate, ok := r.eurPer[currency]
	if !ok {
		return 0, fmt.Errorf("currency %q not found in ECB feed", currency)
	}
	// amount (in src-cents) / srcRate = EUR-cents
	// EUR-cents * usdRate = USD-cents
	usd := (float64(amount) / srcRate) * usdRate
	return int64(math.Round(usd)), nil
}
