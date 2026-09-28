import { describe, expect, it } from "vitest";
import { formatDay, isoDay } from "~/i18n";

describe("editorial dates", () => {
  it("formats in America/Mexico_City in both languages", () => {
    expect(formatDay("es", "2026-09-28T16:00:00Z")).toBe("28 de septiembre de 2026");
    expect(formatDay("en", "2026-09-28T16:00:00Z")).toBe("September 28, 2026");
    // 03:00 UTC on Oct 2 is still Oct 1 in Mexico City.
    expect(formatDay("es", "2026-10-02T03:00:00Z")).toBe("1 de octubre de 2026");
    expect(isoDay("2026-10-02T03:00:00Z")).toBe("2026-10-01");
  });
});
