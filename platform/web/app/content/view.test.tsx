import { readFileSync } from "node:fs";
import { join } from "node:path";
import { renderToStaticMarkup, renderToString } from "react-dom/server";
import { describe, expect, it, vi } from "vitest";
import { ContentEditor } from "./ContentEditor";
import { DocumentView } from "./DocumentView";
import { isSafeHref, parseYouTube, type Doc } from "./schema";

const fixture = (n: string): Doc =>
  JSON.parse(readFileSync(join(__dirname, `../../../contracts/fixtures/documents/${n}.json`), "utf8"));

describe("DocumentView", () => {
  it("renders semantic HTML without loading YouTube", () => {
    const html = renderToStaticMarkup(<DocumentView doc={fixture("full")} locale="es" />);
    for (const tag of ["<h2", "<h3", "<h4", "<strong>", "<em>", "<code", "<ul", "<ol", "<blockquote", "<pre", "<hr", "<table", '<th scope="col"', "<br/>"]) {
      expect(html).toContain(tag);
    }
    expect(html).toContain('rel="noopener noreferrer"');
    expect(html).not.toContain("<iframe");
    expect(html).toContain("Reproducir el vídeo de YouTube");
    expect(html).toContain('class="language-go"');
  });

  it("escapes text instead of interpreting it", () => {
    const html = renderToStaticMarkup(<DocumentView doc={fixture("escaping")} locale="es" />);
    expect(html).toContain("&lt;b&gt;no es html&lt;/b&gt;");
    expect(html).not.toContain("<b>");
  });

  it("never renders an unsafe link even if one slipped into the document", () => {
    const doc = { type: "doc", content: [{ type: "paragraph", content: [{ type: "text", text: "x", marks: [{ type: "link", attrs: { href: "javascript:alert(1)" } }] }] }] } as Doc;
    const html = renderToStaticMarkup(<DocumentView doc={doc} locale="en" />);
    expect(html).not.toContain("javascript:");
    expect(html).not.toContain("<a");
  });

  it("renders real media through /media with reserved dimensions", () => {
    const html = renderToStaticMarkup(
      <DocumentView doc={fixture("media")} locale="es" assets={{ "0b8f3a3e-5d2c-4c1a-9a57-3f0e2b6d9c11": { width: 1200, height: 800, bytes: 1 }, "0b8f3a3e-5d2c-4c1a-9a57-3f0e2b6d9c14": { width: null, height: null, bytes: 2_500_000 } }} />,
    );
    // Compact card: a real link to the file (works without JS), lazy thumbnail with reserved size.
    expect(html).toContain('href="/media/0b8f3a3e-5d2c-4c1a-9a57-3f0e2b6d9c11"');
    expect(html).toContain('<img src="/media/0b8f3a3e-5d2c-4c1a-9a57-3f0e2b6d9c11" alt="Placa de control, cara frontal" width="1200" height="800" loading="lazy"');
    expect(html).toContain("object-contain");
    expect(html).toContain("<figcaption");
    expect(html).toMatch(/<video src="\/media\/0b8f3a3e-5d2c-4c1a-9a57-3f0e2b6d9c12" poster="\/media\/0b8f3a3e-5d2c-4c1a-9a57-3f0e2b6d9c13"[^>]*controls="" preload="metadata"/);
    expect(html).toContain('href="/media/0b8f3a3e-5d2c-4c1a-9a57-3f0e2b6d9c14/download"');
    expect(html).toContain("2.4 MB");
    // Label text is escaped, never HTML.
    expect(html).toContain("Soporte STL v1 &quot;beta&quot;");
  });

  it("renders a stored document without content as empty instead of failing", () => {
    expect(renderToStaticMarkup(<DocumentView doc={{ type: "doc" } as unknown as Doc} locale="es" />)).toBe('<div class="space-y-4 leading-relaxed"></div>');
  });

  it("groups only consecutive images, keeps editorial order and numbers figures", () => {
    const img = (id: string, caption?: string) => ({ type: "image", attrs: { assetId: id, alt: `alt ${id}`, ...(caption ? { caption } : {}) } });
    const p = (text: string) => ({ type: "paragraph", content: [{ type: "text", text }] });
    const doc = { type: "doc", content: [p("antes"), img("a1", "Placa vertical"), img("a2"), img("a3"), p("entre"), img("b1"), { type: "blockquote", content: [img("c1"), img("c2")] }] } as Doc;
    const html = renderToStaticMarkup(<DocumentView doc={doc} locale="es" />);
    const galleries = [...html.matchAll(/data-gallery="(\d+)"/g)].map((m) => m[1]);
    expect(galleries).toEqual(["3", "1", "2"]);
    // Text stays where it was: before the first gallery, between the first and the second.
    expect(html.indexOf("antes")).toBeLessThan(html.indexOf("a1"));
    expect(html.indexOf("a3")).toBeLessThan(html.indexOf("entre"));
    expect(html.indexOf("entre")).toBeLessThan(html.indexOf("b1"));
    expect(html).toContain("FIG. 01");
    expect(html).toContain("FIG. 06");
    expect(html).toContain("Placa vertical");
    expect(html).toContain("Ampliar imagen");
  });

  it("never links an image that is not servable on public pages", () => {
    const doc = { type: "doc", content: [{ type: "image", attrs: { assetId: "ok", alt: "Visible" } }, { type: "image", attrs: { assetId: "gone", alt: "Revocada" } }] } as Doc;
    const html = renderToStaticMarkup(<DocumentView doc={doc} locale="en" publicOnly assets={{ ok: { width: 800, height: 600, bytes: 1 } }} />);
    expect(html).toContain('href="/media/ok"');
    expect(html).not.toContain("/media/gone");
    expect(html).toContain("This file is no longer publicly available.");
  });

  it("keeps wide tables in a scrollable, focusable region", () => {
    const html = renderToStaticMarkup(<DocumentView doc={fixture("tables")} locale="es" />);
    expect(html).toMatch(/<div class="overflow-x-auto" role="region" aria-label="Tabla" tabindex="0">/);
  });
});

describe("ContentEditor on the server", () => {
  it("renders the frame and toolbar without instantiating Tiptap", () => {
    const html = renderToString(<ContentEditor initialDoc={fixture("full")} onChange={vi.fn()} onInvalid={vi.fn()} />);
    expect(html).toContain('role="toolbar"');
    expect(html).toContain("Cargando el editor");
    expect(html).not.toContain("ProseMirror");
    // Without a media bridge (e.g. on the server) media controls are disabled and explain why.
    expect(html.match(/disabled="" aria-describedby=|aria-describedby="[^"]+"[^>]*disabled=""/g)?.length ?? 0).toBeGreaterThanOrEqual(3);
    expect(html).toContain("Los medios se insertan desde el editor de un contenido.");
  });
});

describe("schema helpers mirror the Go validator", () => {
  it("accepts and rejects the same link targets", () => {
    for (const ok of ["https://example.com/a?b=1", "http://x.y", "mailto:a@b.c", "/es/proyectos", "#pinout"]) expect(isSafeHref(ok), ok).toBe(true);
    for (const bad of ["javascript:alert(1)", "JaVaScRiPt:x", "data:text/html,x", "vbscript:x", "//evil.example", "/\\evil", "https://u:p@x.y", "file:///etc/passwd", "java\tscript:x", ""]) {
      expect(isSafeHref(bad), bad).toBe(false);
    }
  });

  it("parses YouTube links and ids", () => {
    expect(parseYouTube("dQw4w9WgXcQ")).toEqual({ videoId: "dQw4w9WgXcQ" });
    expect(parseYouTube("https://www.youtube.com/watch?v=dQw4w9WgXcQ&t=42")).toEqual({ videoId: "dQw4w9WgXcQ", start: 42 });
    expect(parseYouTube("https://youtu.be/dQw4w9WgXcQ")).toEqual({ videoId: "dQw4w9WgXcQ" });
    expect(parseYouTube("https://www.youtube.com/shorts/dQw4w9WgXcQ")).toEqual({ videoId: "dQw4w9WgXcQ" });
    for (const bad of ["https://evil.example/watch?v=dQw4w9WgXcQ", "https://youtube.com/watch?v=short", "<script>", ""]) expect(parseYouTube(bad), bad).toBeNull();
  });
});

describe("displayTitle", () => {
  it("prefers the Spanish title in the Spanish admin, then English", async () => {
    const { displayTitle } = await import("./api-types");
    const c = (titles: Record<string, string | null>) =>
      ({ id: "x", kind: "project", project_id: null, created_at: "", archived_at: null,
        translations: Object.entries(titles).map(([locale, title]) => ({ locale, title, latest_version: 1, published: false, updated_at: null })) }) as never;
    expect(displayTitle(c({ en: "Test rover", es: "Rover de prueba" }), "—")).toBe("Rover de prueba");
    expect(displayTitle(c({ en: "Test rover", es: null }), "—")).toBe("Test rover");
    expect(displayTitle(c({ es: null }), "Sin título")).toBe("Sin título");
  });
});
