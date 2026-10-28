import { describe, expect, it } from "vitest";
import {
  CURRENCY_USD,
  MWK_PER_USD,
  districts,
  districtsByRegion,
  findDistrict,
  formatMWK,
  formatUSD,
  mwk,
  mwkCentsToUsdCents,
  toKwacha,
  toUsd,
  usd,
} from "./malawi";

describe("USD money helpers", () => {
  it("converts dollars to cents", () => {
    expect(usd(125)).toBe(12500);
    expect(usd(85.29)).toBe(8529);
    expect(usd(10.005)).toBe(1001); // rounds
  });

  it("goes back to dollars", () => {
    expect(toUsd(8529)).toBe(85.29);
  });

  it("formats for display", () => {
    expect(formatUSD(12500)).toBe("$125.00");
    expect(formatUSD(99)).toBe("$0.99");
    expect(formatUSD(8529)).toBe("$85.29");
  });

  it("exposes the store currency code", () => {
    expect(CURRENCY_USD).toBe("USD");
  });
});

describe("legacy MWK money helpers", () => {
  it("converts kwacha to tambala minor units", () => {
    expect(mwk(125)).toBe(12500);
    expect(mwk(0.99)).toBe(99);
    expect(mwk(10.005)).toBe(1001); // rounds
  });

  it("goes back to kwacha", () => {
    expect(toKwacha(12500)).toBe(125);
  });

  it("formats for display", () => {
    expect(formatMWK(12500)).toBe("K125.00");
    expect(formatMWK(99)).toBe("K0.99");
  });

  it("matches the server FX default", () => {
    expect(MWK_PER_USD).toBe(1700);
    // The payments module: 125000-3 pricing would be a fraction of a US dollar.
    expect(mwkCentsToUsdCents(1_700_000)).toBe(1000); // K17,000.00 → $10.00
  });
});

describe("districts", () => {
  it("lists all 28 districts", () => {
    expect(districts().length).toBe(28);
  });

  it("partitions by region", () => {
    expect(districtsByRegion("Northern").length).toBe(6);
    expect(districtsByRegion("Central").length).toBe(9);
    expect(districtsByRegion("Southern").length).toBe(13);
  });

  it("looks up case-insensitively", () => {
    expect(findDistrict("BLANTYRE")?.region).toBe("Southern");
    expect(findDistrict("lilongwe")?.name).toBe("Lilongwe");
    expect(findDistrict("Nairobi")).toBeUndefined();
  });
});
