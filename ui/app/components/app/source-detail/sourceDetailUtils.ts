import type { DataSource, SourceRawInput } from "~/types/api";

export const RAW_INPUT_TABLES = new Set([
  "gleif_company_raw_inputs",
  "companies_house_company_raw_inputs",
  "brreg_company_raw_inputs",
  "ai_company_profile_raw_inputs",
  "domain_discovery_raw_inputs",
]);

export function hasRawInputs(source: DataSource): boolean {
  return RAW_INPUT_TABLES.has(source.input_table_name);
}

export function sourceDisplayName(source: DataSource): string {
  return source.display_name || source.name;
}

export function validateDuration(value: string): string | undefined {
  if (!/^\d+[hms]$/.test(value.trim())) {
    return "Use a Go duration such as 24h, 12h, or 30m.";
  }
  return undefined;
}

export function statusClass(status: string): string {
  if (status === "succeeded" || status === "processed") return "bg-green-100 text-green-800 border-green-200";
  if (status === "failed") return "bg-red-100 text-red-800 border-red-200";
  if (status === "running" || status === "processing") return "bg-blue-100 text-blue-800 border-blue-200";
  if (status === "ignored" || status === "cancelled" || status === "superseded") return "bg-gray-100 text-gray-700 border-gray-200";
  return "bg-amber-100 text-amber-800 border-amber-200";
}

export function liveStatusFilter(statusGroup: "live" | "archive"): string {
  return statusGroup === "live"
    ? "in.(pending,processing,failed)"
    : "in.(processed,ignored,superseded)";
}

export function canRetry(row: SourceRawInput): boolean {
  return row.processing_status === "failed" || row.processing_status === "ignored";
}

export function canIgnore(row: SourceRawInput): boolean {
  return row.processing_status === "pending" || row.processing_status === "failed";
}

export function sourceHasProcessor(source: DataSource): boolean {
  return source.input_table_name === "gleif_company_raw_inputs"
    || source.input_table_name === "companies_house_company_raw_inputs"
    || source.input_table_name === "brreg_company_raw_inputs";
}
