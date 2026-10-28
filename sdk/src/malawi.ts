/**
 * Money helpers and Malawi administrative districts. The store prices the
 * catalog, cart and new orders in USD; MWK helpers remain for legacy order
 * amounts and value-preserving conversions.
 * Zero dependencies (mirrors the zero-dependency SDK contract).
 */

// ---------------------------------------------------------------------------
// Money — the API stores minor units ("price_cents" = USD cents)
// ---------------------------------------------------------------------------

/** USD ISO code. Catalog, cart and new orders are priced in USD. */
export const CURRENCY_USD = "USD";

/** Dollars (e.g. 85.29) → USD cents (8529). */
export function usd(dollars: number): number {
  if (!Number.isFinite(dollars)) throw new TypeError(`usd() expects a finite number, got ${dollars}`);
  return Math.round(dollars * 100);
}

/** USD cents (8529) → dollars (85.29). */
export function toUsd(cents: number): number {
  return cents / 100;
}

/** "$85.29" display string from USD cents. */
export function formatUSD(cents: number): string {
  if (!Number.isFinite(cents)) throw new TypeError(`formatUSD() expects a finite number, got ${cents}`);
  return `$${toUsd(cents).toFixed(2)}`;
}

// ---------------------------------------------------------------------------
// Legacy MWK — historical orders and value-preserving conversions
// ---------------------------------------------------------------------------

/** Server default (FX_MWK_PER_USD = 1700) used for legacy MWK→USD conversion. */
export const MWK_PER_USD = 1_700;

/** MWK ISO code. */
export const CURRENCY_MWK = "MWK";

/** Kwacha units (e.g. 125.5) → tambala minor units (12550). */
export function mwk(kwacha: number): number {
  if (!Number.isFinite(kwacha)) throw new TypeError(`mwk() expects a finite number, got ${kwacha}`);
  return Math.round(kwacha * 100);
}

/** Tambala minor units (12550) → kwacha units (125.5). */
export function toKwacha(cents: number): number {
  return cents / 100;
}

/** "K125.00" style display string from minor units. */
export function formatMWK(cents: number): string {
  if (!Number.isFinite(cents)) throw new TypeError(`formatMWK() expects a finite number, got ${cents}`);
  return `K${toKwacha(cents).toFixed(2)}`;
}

/** Approximate USD cents for a MWK minor-unit amount (server-side rate). */
export function mwkCentsToUsdCents(cents: number, rate: number = MWK_PER_USD): number {
  return Math.round(cents / rate);
}

// ---------------------------------------------------------------------------
// Districts — 28 districts grouped by region (delivery/zone UIs)
// ---------------------------------------------------------------------------

export type Region = "Northern" | "Central" | "Southern";

export type District = {
  name: string;
  region: Region;
};

const DISTRICTS: readonly District[] = [
  { name: "Chitipa", region: "Northern" },
  { name: "Karonga", region: "Northern" },
  { name: "Likoma", region: "Northern" },
  { name: "Mzimba", region: "Northern" },
  { name: "Nkhata Bay", region: "Northern" },
  { name: "Rumphi", region: "Northern" },

  { name: "Dedza", region: "Central" },
  { name: "Dowa", region: "Central" },
  { name: "Kasungu", region: "Central" },
  { name: "Lilongwe", region: "Central" },
  { name: "Mchinji", region: "Central" },
  { name: "Nkhotakota", region: "Central" },
  { name: "Ntcheu", region: "Central" },
  { name: "Ntchisi", region: "Central" },
  { name: "Salima", region: "Central" },

  { name: "Balaka", region: "Southern" },
  { name: "Blantyre", region: "Southern" },
  { name: "Chikwawa", region: "Southern" },
  { name: "Chiradzulu", region: "Southern" },
  { name: "Machinga", region: "Southern" },
  { name: "Mangochi", region: "Southern" },
  { name: "Mulanje", region: "Southern" },
  { name: "Mwanza", region: "Southern" },
  { name: "Neno", region: "Southern" },
  { name: "Nsanje", region: "Southern" },
  { name: "Phalombe", region: "Southern" },
  { name: "Thyolo", region: "Southern" },
  { name: "Zomba", region: "Southern" },
] as const;

export function districts(): readonly District[] {
  return DISTRICTS;
}

export function districtsByRegion(region: Region): readonly District[] {
  return DISTRICTS.filter((d) => d.region === region);
}

/** Case-insensitive exact district lookup ("blantyre" → Blantyre). */
export function findDistrict(name: string): District | undefined {
  const needle = name.trim().toLowerCase();
  return DISTRICTS.find((d) => d.name.toLowerCase() === needle);
}
