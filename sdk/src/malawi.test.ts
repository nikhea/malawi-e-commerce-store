import { describe, expect, it } from "vitest";
import {
  MWK_PER_USD,
  districts,
  districtsByRegion,
  findDistrict,
  formatMWK,
  mwk,
  mwkCentsToUsdCents,
  toKwacha,
} from "./malawi";

describe("MWK money helpers", () => {
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
