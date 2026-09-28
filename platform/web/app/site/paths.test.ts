import { describe, expect, it } from "vitest";
import { contentPath, localeOfPath, logPath, sectionPath, siteFromApiPath } from "./paths";

describe("site paths", () => {
  it("uses the localized segments of web-v1.md §9", () => {
    expect(sectionPath("es", "projects")).toBe("/es/proyectos");
    expect(sectionPath("en", "about")).toBe("/en/about");
    expect(logPath("es", "rover", "dia-1")).toBe("/es/proyectos/rover/bitacora/dia-1");
    expect(logPath("en", "rover", "day-1")).toBe("/en/projects/rover/log/day-1");
    expect(contentPath("en", "article", "notes")).toBe("/en/articles/notes");
    expect(contentPath("es", "log", "x", null)).toBeNull();
  });

  it("maps API reader paths to site paths", () => {
    expect(siteFromApiPath("/api/v1/public/es/projects/rover-marte")).toBe("/es/proyectos/rover-marte");
    expect(siteFromApiPath("/api/v1/public/en/projects/rover/logs/day-1")).toBe("/en/projects/rover/log/day-1");
    expect(siteFromApiPath("/api/v1/public/es/articles/nota")).toBe("/es/articulos/nota");
  });

  it("never maps anything else (no open redirects)", () => {
    for (const bad of [
      "https://evil.example/api/v1/public/es/articles/x",
      "//evil.example/api/v1/public/es/articles/x",
      "/api/v1/public/fr/articles/x",
      "/api/v1/public/es/articles/X",
      "/api/v1/public/es/articles/x/../../admin",
      "/api/v1/public/es/articles/x?next=//evil",
      "/api/v1/admin/contents",
      "/es/articulos/x",
      "",
      null,
    ]) {
      expect(siteFromApiPath(bad)).toBeNull();
    }
  });

  it("reads the locale from the path", () => {
    expect(localeOfPath("/en/projects")).toBe("en");
    expect(localeOfPath("/es")).toBe("es");
    expect(localeOfPath("/admin")).toBe("es");
  });
});
