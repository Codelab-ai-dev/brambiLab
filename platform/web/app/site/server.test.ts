import { describe, expect, it } from "vitest";
import { listQuery } from "./loader.server";
import { parseOrigin } from "./seo.server";
import { xmlEscape, urlset } from "./sitemap.server";

const req = (qs: string) => new Request(`http://internal/es/proyectos${qs}`);

describe("PUBLIC_ORIGIN", () => {
  it("accepts bare https origins and localhost over http", () => {
    expect(parseOrigin("https://brambilab.example", true)).toBe("https://brambilab.example");
    expect(parseOrigin("https://brambilab.example/", true)).toBe("https://brambilab.example");
    expect(parseOrigin("http://localhost:8000", false)).toBe("http://localhost:8000");
  });
  it("rejects anything that is not a bare trusted origin", () => {
    for (const bad of ["http://brambilab.example", "https://brambilab.example/es", "https://u:p@x.example", "https://x.example?a=1", "ftp://x.example", "not a url"]) {
      expect(() => parseOrigin(bad, true), bad).toThrow();
    }
  });
  it("requires it in production and defaults to the local proxy otherwise", () => {
    expect(() => parseOrigin(undefined, true)).toThrow();
    expect(parseOrigin(undefined, false)).toBe("http://localhost:8000");
  });
});

describe("list query", () => {
  it("parses filters and pages", () => {
    expect(listQuery(req("?category=hardware&tag=esp32&page=2"), ["page", "category", "tag"])).toEqual({ page: 2, category: "hardware", tag: "esp32" });
    expect(listQuery(req("?category="), ["page", "category", "tag"])).toEqual({ page: 1 });
  });
  it("rejects malformed or unknown parameters with 400", () => {
    for (const qs of ["?page=0", "?page=abc", "?page=1&page=2", "?category=No%20Slug", "?sort=title", "?kind=page"]) {
      try {
        listQuery(req(qs), ["page", "category", "tag", "kind"]);
        throw new Error(`accepted ${qs}`);
      } catch (e) {
        expect((e as { init?: { status?: number } }).init?.status, qs).toBe(400);
      }
    }
  });
});

describe("sitemap XML", () => {
  it("escapes every URL and only links alternates when both exist", () => {
    expect(xmlEscape(`a&b<c>"'`)).toBe("a&amp;b&lt;c&gt;&quot;&apos;");
    const xml = urlset([
      { loc: "https://x.example/es/articulos/a&b", alternates: { es: "https://x.example/es/articulos/a&b" } },
      { loc: "https://x.example/es", alternates: { es: "https://x.example/es", en: "https://x.example/en" } },
    ]);
    expect(xml).toContain("<loc>https://x.example/es/articulos/a&amp;b</loc>");
    expect(xml.match(/xhtml:link/g)?.length).toBe(2);
  });
});

describe("SEO description fallback", () => {
  it("tolerates a stored document without content (was a 500)", async () => {
    const { firstText } = await import("./detail.server");
    expect(firstText({ body: { type: "doc" } as never })).toBe("");
    expect(firstText({ body: { type: "doc", content: [{ type: "paragraph", content: [{ type: "text", text: "Hola  mundo" }] }] } as never })).toBe("Hola mundo");
  });
});
