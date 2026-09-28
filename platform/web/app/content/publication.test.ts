import { describe, expect, it } from "vitest";
import { formatLocal, nowLocal } from "./api-types";

describe("editorial wall clock", () => {
  it("shows the local time with its zone, never converted", () => {
    expect(formatLocal("2026-10-01T09:30")).toBe("1 oct 2026, 09:30 (America/Mexico_City)");
    expect(formatLocal("2026-12-31T23:59")).toMatch(/^31 dic 2026, 23:59 /);
  });

  it("reads now in America/Mexico_City regardless of the browser zone", () => {
    // 2026-09-28 16:00 UTC is 10:00 in Mexico City (UTC-6, no DST since 2022).
    expect(nowLocal(new Date("2026-09-28T16:00:00Z"))).toBe("2026-09-28T10:00");
    // Crossing midnight UTC stays on the local date.
    expect(nowLocal(new Date("2026-10-02T03:15:00Z"))).toBe("2026-10-01T21:15");
    // Former DST date: still UTC-6.
    expect(nowLocal(new Date("2026-07-01T12:00:00Z"))).toBe("2026-07-01T06:00");
  });
});
