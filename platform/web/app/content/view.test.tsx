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

  it("shows an honest placeholder for media until uploads exist", () => {
    const html = renderToStaticMarkup(<DocumentView doc={fixture("media")} locale="en" />);
    expect(html.match(/File pending/g)).toHaveLength(3);
    expect(html).not.toMatch(/<img|<video/);
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
    // Media controls are disabled and explain why.
    expect(html.match(/disabled="" aria-describedby=|aria-describedby="[^"]+"[^>]*disabled=""/g)?.length ?? 0).toBeGreaterThanOrEqual(3);
    expect(html).toContain("WEB-004");
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
