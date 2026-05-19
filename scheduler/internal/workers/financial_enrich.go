package workers

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"

	"github.com/google/uuid"
	"github.com/riverqueue/river"

	db "github.com/pulsarpoint/corpscout/scheduler/internal/db/gen"
	"github.com/pulsarpoint/corpscout/scheduler/internal/fxrates"
)

type FinancialEnrichWorker struct {
	river.WorkerDefaults[EnrichCompanyFinancialsArgs]
	db db.Querier
}

func NewFinancialEnrichWorker(q db.Querier) *FinancialEnrichWorker {
	return &FinancialEnrichWorker{db: q}
}

func (w *FinancialEnrichWorker) Work(ctx context.Context, job *river.Job[EnrichCompanyFinancialsArgs]) error {
	args := job.Args
	companyID, err := uuid.Parse(args.CompanyID)
	if err != nil {
		return fmt.Errorf("parse company id: %w", err)
	}

	accounts, err := fetchBrregAccounts(ctx, args.OrgNumber)
	if err != nil {
		slog.Warn("brreg regnskap fetch failed", "org", args.OrgNumber, "error", err)
		return nil
	}
	if len(accounts) == 0 {
		slog.Info("no regnskap accounts found", "org", args.OrgNumber)
		return nil
	}

	acc := accounts[0] // most recent

	revenueOrig := int64(acc.Revenue * 100)
	profitOrig := int64(acc.Profit * 100)

	var revenueUSDPtr, profitUSDPtr *int64
	rates, err := fxrates.Load(ctx)
	if err != nil {
		slog.Warn("fxrates load failed — storing without USD conversion", "error", err)
	} else {
		if rev, err := rates.ToUSD(revenueOrig, "NOK"); err == nil {
			revenueUSDPtr = &rev
		}
		if prf, err := rates.ToUSD(profitOrig, "NOK"); err == nil {
			profitUSDPtr = &prf
		}
	}

	year := int32(acc.Year)
	currency := "NOK"
	_, err = w.db.CreateCompanyFinancial(ctx, db.CreateCompanyFinancialParams{
		CompanyID:       companyID,
		Year:            year,
		SourceName:      args.SourceName,
		RevenueAmount:   &revenueOrig,
		RevenueCurrency: &currency,
		RevenueUsd:      revenueUSDPtr,
		ProfitAmount:    &profitOrig,
		ProfitUsd:       profitUSDPtr,
	})
	if err != nil {
		return fmt.Errorf("create company financial: %w", err)
	}
	slog.Info("company financial suggestion created",
		"company_id", args.CompanyID,
		"org_number", args.OrgNumber,
		"year", year,
		"revenue_orig_cents", revenueOrig,
	)
	return nil
}

type brregAccount struct {
	Year    int
	Revenue float64
	Profit  float64
}

type brregRegnskapDTO struct {
	Regnskapsperiode struct {
		TilDato string `json:"tilDato"`
	} `json:"regnskapsperiode"`
	ResultatregnskapResultat struct {
		Driftsresultat struct {
			Driftsinntekter struct {
				SumDriftsinntekter *float64 `json:"sumDriftsinntekter"`
			} `json:"driftsinntekter"`
		} `json:"driftsresultat"`
		OrdinaertResultatFoerSkattekostnad *float64 `json:"ordinaertResultatFoerSkattekostnad"`
	} `json:"resultatregnskapResultat"`
}

func fetchBrregAccounts(ctx context.Context, orgNumber string) ([]brregAccount, error) {
	url := fmt.Sprintf("https://data.brreg.no/regnskapsregisteret/regnskap/%s", orgNumber)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return nil, nil
	}
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("brreg returned %d: %s", resp.StatusCode, string(b))
	}

	var dtos []brregRegnskapDTO
	if err := json.NewDecoder(resp.Body).Decode(&dtos); err != nil {
		return nil, fmt.Errorf("decode brreg response: %w", err)
	}

	accounts := make([]brregAccount, 0, len(dtos))
	for _, d := range dtos {
		year := 0
		if len(d.Regnskapsperiode.TilDato) >= 4 {
			fmt.Sscanf(d.Regnskapsperiode.TilDato[:4], "%d", &year)
		}
		var revenue, profit float64
		if v := d.ResultatregnskapResultat.Driftsresultat.Driftsinntekter.SumDriftsinntekter; v != nil {
			revenue = *v
		}
		if v := d.ResultatregnskapResultat.OrdinaertResultatFoerSkattekostnad; v != nil {
			profit = *v
		}
		accounts = append(accounts, brregAccount{Year: year, Revenue: revenue, Profit: profit})
	}
	return accounts, nil
}
