package service

import (
	"context"
	"encoding/json"
	"strconv"
	"strings"

	"github.com/cockroachdb/errors"
	"github.com/google/uuid"
	pgx "github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	db "github.com/pulsarpoint/corpscout/scheduler/internal/db/gen"
	"github.com/pulsarpoint/corpscout/scheduler/internal/slug"
)

var (
	ErrRawInputNotFound            = errors.New("raw input not found")
	ErrRawInputNotApprovable       = errors.New("raw input is not approvable")
	ErrRawInputRequiresTranslation = errors.New("raw input translation is required before approval")
	ErrRawInputUnsupportedSource   = errors.New("raw input source is not supported for company approval")
	ErrRawInputCountryNotFound     = errors.New("raw input country not found")
)

type rawCompanyCandidate struct {
	id                 uuid.UUID
	sourceName         string
	sourceNativeID     string
	displayName        string
	countryISO2        string
	registrationNumber *string
	lei                *string
	website            *string
	registrationStatus *string
	parentLei          *string
	ultimateParentLei  *string
	processingStatus   string
	translated         bool
	emails             []rawCompanyContact
	phones             []rawCompanyContact
	financials         []rawCompanyFinancial
	ownership          []rawCompanyOwnership
}

type rawCompanyContact struct {
	Kind        string
	Value       string
	Description string
	Source      string
}

type rawCompanyFinancial struct {
	Year            int
	EmployeeCount   *int32
	RevenueAmount   *int64
	RevenueCurrency string
	ProfitAmount    *int64
}

type rawCompanyOwnership struct {
	Source string
	Data   map[string]any
}

// ApproveCompanyRawInput creates or returns a resolved company directly from a
// source raw input and marks that raw input processed. This replaces the retired
// source_process/company_suggestion path for registry raw inputs.
func ApproveCompanyRawInput(ctx context.Context, pool TxPool, sourceName string, rawInputID uuid.UUID, reviewedBy, reviewNote string) (db.Company, error) {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return db.Company{}, errors.Wrap(err, "begin tx")
	}
	defer func() { _ = tx.Rollback(ctx) }()

	qtx := db.New(tx)
	src, err := qtx.GetSourceByName(ctx, sourceName)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return db.Company{}, ErrRawInputUnsupportedSource
		}
		return db.Company{}, errors.Wrap(err, "get source")
	}

	candidate, err := loadRawCompanyCandidate(ctx, qtx, src, rawInputID)
	if err != nil {
		return db.Company{}, err
	}
	if !isRawInputStatusApprovable(candidate.processingStatus) {
		return db.Company{}, ErrRawInputNotApprovable
	}
	if candidate.countryISO2 == "" {
		return db.Company{}, ErrRawInputCountryNotFound
	}

	countryID, err := qtx.GetCountryIDByISO2(ctx, candidate.countryISO2)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return db.Company{}, ErrRawInputCountryNotFound
		}
		return db.Company{}, errors.Wrap(err, "get raw input country")
	}

	existing, err := findExistingRawInputCompany(ctx, qtx, candidate)
	if err == nil {
		company, err := persistRawCompanyEnrichment(ctx, qtx, existing, candidate, true)
		if err != nil {
			return db.Company{}, err
		}
		if err := markRawInputApproved(ctx, qtx, src.InputTableName, rawInputID); err != nil {
			return db.Company{}, err
		}
		if err := tx.Commit(ctx); err != nil {
			return db.Company{}, errors.Wrap(err, "commit")
		}
		return company, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return db.Company{}, errors.Wrap(err, "lookup existing company")
	}

	canonicalSlug := slug.Generate(candidate.displayName)
	if canonicalSlug == "" {
		canonicalSlug = "company-" + rawInputID.String()[:12]
	}
	if _, err := qtx.GetCompanyBySlug(ctx, canonicalSlug); err == nil {
		canonicalSlug = canonicalSlug + "-" + rawInputID.String()[:12]
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return db.Company{}, errors.Wrap(err, "check slug collision")
	}

	evidence, err := rawInputApprovalEvidence(candidate, reviewedBy, reviewNote)
	if err != nil {
		return db.Company{}, errors.Wrap(err, "build approval evidence")
	}
	status := normalizeResolvedCompanyStatus(candidate.registrationStatus)

	company, err := qtx.InsertCompanyFromRawInput(ctx, db.InsertCompanyFromRawInputParams{
		CanonicalSlug:      canonicalSlug,
		Name:               candidate.displayName,
		CountryID:          countryID,
		RegistrationNumber: candidate.registrationNumber,
		Lei:                candidate.lei,
		Status:             status,
		Website:            candidate.website,
		PrimarySourceID:    pgtype.UUID{Bytes: src.ID, Valid: true},
		ParentLei:          candidate.parentLei,
		UltimateParentLei:  candidate.ultimateParentLei,
		Evidence:           evidence,
	})
	if err != nil {
		return db.Company{}, errors.Wrap(err, "insert company from raw input")
	}

	company, err = persistRawCompanyEnrichment(ctx, qtx, company, candidate, false)
	if err != nil {
		return db.Company{}, err
	}

	if err := markRawInputApproved(ctx, qtx, src.InputTableName, rawInputID); err != nil {
		return db.Company{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return db.Company{}, errors.Wrap(err, "commit")
	}
	return company, nil
}

func loadRawCompanyCandidate(ctx context.Context, q *db.Queries, src db.DataSource, rawInputID uuid.UUID) (rawCompanyCandidate, error) {
	switch src.InputTableName {
	case "gleif_company_raw_inputs":
		row, err := q.GetGLEIFRawInputForCompanyApproval(ctx, rawInputID)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return rawCompanyCandidate{}, ErrRawInputNotFound
			}
			return rawCompanyCandidate{}, errors.Wrap(err, "get gleif raw input")
		}
		displayName := fallbackString(row.LegalName, row.Lei)
		return rawCompanyCandidate{
			id:                 row.ID,
			sourceName:         src.Name,
			sourceNativeID:     row.SourceNativeID,
			displayName:        displayName,
			countryISO2:        fallbackString(row.HeadquartersCountryCode, ""),
			lei:                &row.Lei,
			registrationStatus: row.RegistrationStatus,
			parentLei:          row.ParentLei,
			ultimateParentLei:  row.UltimateParentLei,
			processingStatus:   row.ProcessingStatus,
		}, nil
	case "companies_house_company_raw_inputs":
		row, err := q.GetCompaniesHouseRawInputForCompanyApproval(ctx, rawInputID)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return rawCompanyCandidate{}, ErrRawInputNotFound
			}
			return rawCompanyCandidate{}, errors.Wrap(err, "get companies house raw input")
		}
		displayName := fallbackString(row.CompanyName, row.CompanyNumber)
		countryISO2 := fallbackString(row.CountryIso2, "GB")
		return rawCompanyCandidate{
			id:                 row.ID,
			sourceName:         src.Name,
			sourceNativeID:     row.SourceNativeID,
			displayName:        displayName,
			countryISO2:        countryISO2,
			registrationNumber: &row.CompanyNumber,
			registrationStatus: row.CompanyStatus,
			processingStatus:   row.ProcessingStatus,
		}, nil
	case "brreg_company_raw_inputs":
		row, err := q.GetBrregRawInputForCompanyApproval(ctx, rawInputID)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return rawCompanyCandidate{}, ErrRawInputNotFound
			}
			return rawCompanyCandidate{}, errors.Wrap(err, "get brreg raw input")
		}
		if row.TranslationStatus != "translated" || len(row.RawPayloadEn) == 0 {
			return rawCompanyCandidate{}, ErrRawInputRequiresTranslation
		}
		displayName := fallbackString(row.OrganizationName, row.OrganizationNumber)
		countryISO2 := fallbackString(row.CountryIso2, "NO")
		return rawCompanyCandidate{
			id:                 row.ID,
			sourceName:         src.Name,
			sourceNativeID:     row.SourceNativeID,
			displayName:        displayName,
			countryISO2:        countryISO2,
			registrationNumber: &row.OrganizationNumber,
			website:            row.Website,
			registrationStatus: row.RegistrationStatus,
			processingStatus:   row.ProcessingStatus,
			translated:         true,
		}, nil
	case "cvr_company_raw_inputs":
		row, err := q.GetCVRRawInputForCompanyApproval(ctx, rawInputID)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return rawCompanyCandidate{}, ErrRawInputNotFound
			}
			return rawCompanyCandidate{}, errors.Wrap(err, "get cvr raw input")
		}
		return buildCVRRawCompanyCandidate(row, src)
	case "ariregister_company_raw_inputs":
		row, err := q.GetAriregisterRawInputForCompanyApproval(ctx, rawInputID)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return rawCompanyCandidate{}, ErrRawInputNotFound
			}
			return rawCompanyCandidate{}, errors.Wrap(err, "get ariregister raw input")
		}
		return buildAriregisterRawCompanyCandidate(row, src)
	default:
		return rawCompanyCandidate{}, ErrRawInputUnsupportedSource
	}
}

func buildCVRRawCompanyCandidate(row db.CvrCompanyRawInput, src db.DataSource) (rawCompanyCandidate, error) {
	if row.TranslationStatus != "translated" || len(row.RawPayloadEn) == 0 {
		return rawCompanyCandidate{}, ErrRawInputRequiresTranslation
	}
	payload, err := decodeRawCompanyPayload(row.RawPayloadEn)
	if err != nil {
		return rawCompanyCandidate{}, errors.Wrap(err, "decode cvr translated payload")
	}
	displayName := fallbackString(row.CompanyName, row.CvrNumber)
	countryISO2 := fallbackString(row.CountryIso2, "DK")
	website := firstStringPtr(row.Website, payloadString(payload, "website", "official_website", "homepage"))
	email := firstStringPtr(row.Email, payloadString(payload, "email", "official_email"))
	phone := firstStringPtr(row.Phone, payloadString(payload, "phone", "official_phone"))
	registrationStatus := firstStringPtr(row.RegistrationStatus, payloadString(payload, "registration_status", "status"))

	candidate := rawCompanyCandidate{
		id:                 row.ID,
		sourceName:         src.Name,
		sourceNativeID:     row.SourceNativeID,
		displayName:        displayName,
		countryISO2:        countryISO2,
		registrationNumber: &row.CvrNumber,
		website:            website,
		registrationStatus: registrationStatus,
		processingStatus:   row.ProcessingStatus,
		translated:         true,
		financials:         rawCompanyFinancialsFromPayload(payload),
		ownership:          rawCompanyOwnershipFromPayload(src.Name, payload, "ownership", "owners", "beneficial_owners"),
	}
	if email != nil {
		candidate.emails = append(candidate.emails, rawCompanyContact{Kind: "official", Value: *email, Source: src.Name})
	}
	if phone != nil {
		candidate.phones = append(candidate.phones, rawCompanyContact{Kind: "official", Value: *phone, Source: src.Name})
	}
	return candidate, nil
}

func buildAriregisterRawCompanyCandidate(row db.AriregisterCompanyRawInput, src db.DataSource) (rawCompanyCandidate, error) {
	if row.TranslationStatus != "translated" || len(row.RawPayloadEn) == 0 {
		return rawCompanyCandidate{}, ErrRawInputRequiresTranslation
	}
	payload, err := decodeRawCompanyPayload(row.RawPayloadEn)
	if err != nil {
		return rawCompanyCandidate{}, errors.Wrap(err, "decode ariregister translated payload")
	}
	displayName := fallbackString(row.LegalName, row.RegistryCode)
	countryISO2 := fallbackString(row.CountryIso2, "EE")
	website := firstStringPtr(row.Website, payloadString(payload, "website", "official_website", "homepage"))
	email := firstStringPtr(row.Email, payloadString(payload, "email", "official_email"))
	phone := firstStringPtr(row.Phone, payloadString(payload, "phone", "official_phone"))
	registrationStatus := firstStringPtr(row.RegistrationStatus, payloadString(payload, "registration_status", "status"))

	candidate := rawCompanyCandidate{
		id:                 row.ID,
		sourceName:         src.Name,
		sourceNativeID:     row.SourceNativeID,
		displayName:        displayName,
		countryISO2:        countryISO2,
		registrationNumber: &row.RegistryCode,
		website:            website,
		registrationStatus: registrationStatus,
		processingStatus:   row.ProcessingStatus,
		translated:         true,
		financials:         rawCompanyFinancialsFromPayload(payload),
		ownership:          rawCompanyOwnershipFromPayload(src.Name, payload, "beneficial_owners", "ownership", "owners"),
	}
	if email != nil {
		candidate.emails = append(candidate.emails, rawCompanyContact{Kind: "official", Value: *email, Source: src.Name})
	}
	if phone != nil {
		candidate.phones = append(candidate.phones, rawCompanyContact{Kind: "official", Value: *phone, Source: src.Name})
	}
	return candidate, nil
}

func findExistingRawInputCompany(ctx context.Context, q *db.Queries, candidate rawCompanyCandidate) (db.Company, error) {
	if candidate.lei != nil && *candidate.lei != "" {
		company, err := q.GetCompanyByLEI(ctx, candidate.lei)
		if err == nil || !errors.Is(err, pgx.ErrNoRows) {
			return company, err
		}
	}
	if candidate.registrationNumber != nil && *candidate.registrationNumber != "" {
		return q.GetCompanyByRegistrationAndCountry(ctx, db.GetCompanyByRegistrationAndCountryParams{
			RegistrationNumber: candidate.registrationNumber,
			IsoAlpha2:          candidate.countryISO2,
		})
	}
	return db.Company{}, pgx.ErrNoRows
}

func persistRawCompanyEnrichment(ctx context.Context, q *db.Queries, company db.Company, candidate rawCompanyCandidate, updateWebsite bool) (db.Company, error) {
	for _, email := range candidate.emails {
		if strings.TrimSpace(email.Value) == "" {
			continue
		}
		evidence, err := rawInputEnrichmentEvidence(candidate, email.Source, "email")
		if err != nil {
			return db.Company{}, errors.Wrap(err, "build email evidence")
		}
		description := optionalString(email.Description)
		if _, err := q.UpsertCompanyEmail(ctx, db.UpsertCompanyEmailParams{
			CompanyID:   company.ID,
			Email:       strings.TrimSpace(email.Value),
			Description: description,
			Purpose:     fallbackString(&email.Kind, "official"),
			Source:      fallbackString(&email.Source, candidate.sourceName),
			Confidence:  ptrFloat32(1),
			Evidence:    evidence,
		}); err != nil {
			return db.Company{}, errors.Wrap(err, "upsert company email")
		}
	}

	for _, phone := range candidate.phones {
		if strings.TrimSpace(phone.Value) == "" {
			continue
		}
		evidence, err := rawInputEnrichmentEvidence(candidate, phone.Source, "phone")
		if err != nil {
			return db.Company{}, errors.Wrap(err, "build phone evidence")
		}
		description := optionalString(phone.Description)
		if _, err := q.UpsertCompanyPhone(ctx, db.UpsertCompanyPhoneParams{
			CompanyID:   company.ID,
			Phone:       strings.TrimSpace(phone.Value),
			Description: description,
			Purpose:     fallbackString(&phone.Kind, "official"),
			Source:      fallbackString(&phone.Source, candidate.sourceName),
			Confidence:  ptrFloat32(1),
			Evidence:    evidence,
		}); err != nil {
			return db.Company{}, errors.Wrap(err, "upsert company phone")
		}
	}

	for _, financial := range candidate.financials {
		if financial.Year == 0 {
			continue
		}
		var currency *string
		if strings.TrimSpace(financial.RevenueCurrency) != "" {
			currency = ptrStringValue(strings.TrimSpace(financial.RevenueCurrency))
		}
		if _, err := q.CreateCompanyFinancial(ctx, db.CreateCompanyFinancialParams{
			CompanyID:       company.ID,
			Year:            int32(financial.Year),
			SourceName:      candidate.sourceName,
			EmployeeCount:   financial.EmployeeCount,
			RevenueAmount:   financial.RevenueAmount,
			RevenueCurrency: currency,
			ProfitAmount:    financial.ProfitAmount,
		}); err != nil {
			return db.Company{}, errors.Wrap(err, "create company financial")
		}
	}

	var ownership []byte
	var err error
	if len(candidate.ownership) > 0 {
		ownership, err = mergeRawCompanyOwnership(company.Ownership, candidate)
		if err != nil {
			return db.Company{}, errors.Wrap(err, "merge company ownership evidence")
		}
	}
	website := (*string)(nil)
	if updateWebsite {
		website = candidate.website
	}
	if website == nil && ownership == nil {
		return company, nil
	}
	updated, err := q.UpdateCompanyEnrichment(ctx, db.UpdateCompanyEnrichmentParams{
		Website:   website,
		Ownership: ownership,
		ID:        company.ID,
	})
	if err != nil {
		return db.Company{}, errors.Wrap(err, "update company enrichment")
	}
	return updated, nil
}

func decodeRawCompanyPayload(payload []byte) (map[string]any, error) {
	var data map[string]any
	decoder := json.NewDecoder(strings.NewReader(string(payload)))
	decoder.UseNumber()
	if err := decoder.Decode(&data); err != nil {
		return nil, err
	}
	return data, nil
}

func rawCompanyFinancialsFromPayload(payload map[string]any) []rawCompanyFinancial {
	var financials []rawCompanyFinancial
	financials = append(financials, financialsFromArray(payloadValue(payload, "financials"))...)
	financials = append(financials, financialsFromArray(payloadValue(payload, "annual_reports", "annual_reports_en"))...)
	return financials
}

func financialsFromArray(value any) []rawCompanyFinancial {
	items, ok := value.([]any)
	if !ok {
		return nil
	}
	financials := make([]rawCompanyFinancial, 0, len(items))
	for _, item := range items {
		m, ok := item.(map[string]any)
		if !ok {
			continue
		}
		indicators, _ := m["indicators"].(map[string]any)
		year := intFromAny(firstAny(m["year"], indicators["year"]))
		if year == 0 {
			continue
		}
		financials = append(financials, rawCompanyFinancial{
			Year:            year,
			EmployeeCount:   int32PtrFromAny(firstAny(m["employee_count"], m["employees"], indicators["employee_count"], indicators["employees"])),
			RevenueAmount:   int64PtrFromAny(firstAny(m["revenue_amount"], m["revenue"], m["sales_revenue"], indicators["revenue_amount"], indicators["revenue"], indicators["sales_revenue"])),
			RevenueCurrency: stringFromAny(firstAny(m["revenue_currency"], m["currency"], indicators["revenue_currency"], indicators["currency"])),
			ProfitAmount:    int64PtrFromAny(firstAny(m["profit_amount"], m["profit"], indicators["profit_amount"], indicators["profit"])),
		})
	}
	return financials
}

func rawCompanyOwnershipFromPayload(source string, payload map[string]any, keys ...string) []rawCompanyOwnership {
	value := payloadValue(payload, keys...)
	items, ok := value.([]any)
	if !ok {
		if m, ok := value.(map[string]any); ok {
			return []rawCompanyOwnership{{Source: source, Data: m}}
		}
		return nil
	}
	ownership := make([]rawCompanyOwnership, 0, len(items))
	for _, item := range items {
		if m, ok := item.(map[string]any); ok {
			ownership = append(ownership, rawCompanyOwnership{Source: source, Data: m})
		}
	}
	return ownership
}

func mergeRawCompanyOwnership(existing json.RawMessage, candidate rawCompanyCandidate) ([]byte, error) {
	payload := map[string]any{}
	if len(existing) > 0 && string(existing) != "null" {
		if err := json.Unmarshal(existing, &payload); err != nil {
			return nil, err
		}
	}
	unresolved, _ := payload["unresolved"].([]any)
	for _, item := range candidate.ownership {
		unresolved = append(unresolved, map[string]any{
			"source":           fallbackString(&item.Source, candidate.sourceName),
			"source_input_id":  candidate.id.String(),
			"source_native_id": candidate.sourceNativeID,
			"data":             item.Data,
		})
	}
	payload["unresolved"] = unresolved
	return json.Marshal(payload)
}

func rawInputEnrichmentEvidence(candidate rawCompanyCandidate, source, kind string) (json.RawMessage, error) {
	payload := map[string]any{
		"source":           fallbackString(&source, candidate.sourceName),
		"source_input_id":  candidate.id.String(),
		"source_native_id": candidate.sourceNativeID,
		"kind":             kind,
	}
	b, err := json.Marshal(payload)
	return json.RawMessage(b), err
}

func markRawInputApproved(ctx context.Context, q *db.Queries, inputTableName string, rawInputID uuid.UUID) error {
	switch inputTableName {
	case "gleif_company_raw_inputs":
		return errors.Wrap(q.MarkGLEIFRawInputProcessed(ctx, rawInputID), "mark gleif raw input processed")
	case "companies_house_company_raw_inputs":
		return errors.Wrap(q.MarkCompaniesHouseRawInputProcessed(ctx, rawInputID), "mark companies house raw input processed")
	case "brreg_company_raw_inputs":
		return errors.Wrap(q.MarkBrregRawInputProcessed(ctx, rawInputID), "mark brreg raw input processed")
	case "cvr_company_raw_inputs":
		return errors.Wrap(q.MarkCVRRawInputProcessed(ctx, rawInputID), "mark cvr raw input processed")
	case "ariregister_company_raw_inputs":
		return errors.Wrap(q.MarkAriregisterRawInputProcessed(ctx, rawInputID), "mark ariregister raw input processed")
	default:
		return ErrRawInputUnsupportedSource
	}
}

func rawInputApprovalEvidence(candidate rawCompanyCandidate, reviewedBy, reviewNote string) (json.RawMessage, error) {
	payload := map[string]any{
		"source":           candidate.sourceName,
		"source_input_id":  candidate.id.String(),
		"source_native_id": candidate.sourceNativeID,
		"reviewed_by":      reviewedBy,
		"translated":       candidate.translated,
	}
	if reviewNote != "" {
		payload["review_note"] = reviewNote
	}
	b, err := json.Marshal(payload)
	return json.RawMessage(b), err
}

func isRawInputStatusApprovable(status string) bool {
	return status == "pending" || status == "failed"
}

func normalizeResolvedCompanyStatus(rawStatus *string) string {
	if rawStatus == nil {
		return "active"
	}
	status := strings.ToLower(strings.TrimSpace(*rawStatus))
	switch {
	case status == "":
		return "active"
	case strings.Contains(status, "active"):
		return "active"
	case strings.Contains(status, "registered"):
		return "active"
	case strings.Contains(status, "dissolved"):
		return "dissolved"
	case strings.Contains(status, "deleted"):
		return "dissolved"
	case strings.Contains(status, "closed"):
		return "dissolved"
	default:
		return "inactive"
	}
}

func fallbackString(value *string, fallback string) string {
	if value == nil || strings.TrimSpace(*value) == "" {
		return fallback
	}
	return *value
}

func firstStringPtr(values ...*string) *string {
	for _, value := range values {
		if value != nil && strings.TrimSpace(*value) != "" {
			return value
		}
	}
	return nil
}

func payloadString(payload map[string]any, keys ...string) *string {
	value := stringFromAny(payloadValue(payload, keys...))
	if value == "" {
		return nil
	}
	return &value
}

func payloadValue(payload map[string]any, keys ...string) any {
	for _, key := range keys {
		if value, ok := payload[key]; ok {
			return value
		}
	}
	return nil
}

func firstAny(values ...any) any {
	for _, value := range values {
		if value != nil {
			return value
		}
	}
	return nil
}

func stringFromAny(value any) string {
	switch v := value.(type) {
	case string:
		return strings.TrimSpace(v)
	case json.Number:
		return v.String()
	case float64:
		return strconv.FormatFloat(v, 'f', -1, 64)
	case int:
		return strconv.Itoa(v)
	default:
		return ""
	}
}

func intFromAny(value any) int {
	switch v := value.(type) {
	case json.Number:
		i, _ := strconv.Atoi(v.String())
		return i
	case float64:
		return int(v)
	case int:
		return v
	case string:
		i, _ := strconv.Atoi(strings.TrimSpace(v))
		return i
	default:
		return 0
	}
}

func int32PtrFromAny(value any) *int32 {
	i := intFromAny(value)
	if i == 0 {
		return nil
	}
	v := int32(i)
	return &v
}

func int64PtrFromAny(value any) *int64 {
	switch v := value.(type) {
	case json.Number:
		i, err := strconv.ParseInt(v.String(), 10, 64)
		if err == nil && i != 0 {
			return &i
		}
	case float64:
		if v != 0 {
			i := int64(v)
			return &i
		}
	case int:
		if v != 0 {
			i := int64(v)
			return &i
		}
	case string:
		i, err := strconv.ParseInt(strings.TrimSpace(v), 10, 64)
		if err == nil && i != 0 {
			return &i
		}
	}
	return nil
}

func optionalString(value string) *string {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	return &value
}

func ptrStringValue(value string) *string {
	return &value
}

func ptrFloat32(value float32) *float32 {
	return &value
}
