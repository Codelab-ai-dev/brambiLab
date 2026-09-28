import { readdirSync, readFileSync } from "node:fs";
import { join } from "node:path";
import { describe, expect, it } from "vitest";
import { docToMarkdown, markdownToDoc } from "./markdown";
import type { Block, Doc } from "./schema";

const dir = join(__dirname, "../../../contracts/fixtures/documents");
export const fixtures: Record<string, Doc> = Object.fromEntries(
  readdirSync(dir)
    .filter((f) => f.endsWith(".json"))
    .map((f) => [f.replace(".json", ""), JSON.parse(readFileSync(join(dir, f), "utf8")) as Doc]),
);

// Empty paragraphs are visual spacing only; Markdown cannot represent them (documented).
function withoutEmptyParagraphs(doc: Doc): Doc {
  const strip = (blocks: Block[]): Block[] =>
    blocks
      .filter((b) => !(b.type === "paragraph" && !b.content?.length))
      .map((b) => ("content" in b && Array.isArray(b.content) && b.type !== "table" && b.type !== "paragraph" && b.type !== "heading" && b.type !== "codeBlock"
        ? ({ ...b, content: (b.content as any[]).map((c) => (c.type === "listItem" ? { ...c, content: strip(c.content) } : c)) } as Block)
        : b));
  return { type: "doc", content: strip(doc.content) };
}

describe("canonical → Markdown → canonical", () => {
  for (const [name, doc] of Object.entries(fixtures)) {
    it(`${name} keeps its semantics`, () => {
      const md = docToMarkdown(doc);
      const back = markdownToDoc(md);
      expect(back.doc, md).toEqual(withoutEmptyParagraphs(doc));
      // Exporting again is stable.
      expect(docToMarkdown(back.doc)).toBe(md);
      const unexpected = back.warnings.filter((w) => !w.message.includes("WEB-004"));
      expect(unexpected, md).toEqual([]);
    });
  }
});

describe("Markdown import", () => {
  const messages = (md: string) => markdownToDoc(md).warnings.map((w) => w.message);

  it("reports and keeps unsupported syntax instead of dropping it", () => {
    const { doc, warnings } = markdownToDoc("<div>raw</div>\n\n::rara{a=1}\n\n[mal](javascript:alert(1))\n");
    expect(warnings).toHaveLength(3);
    expect(JSON.stringify(doc)).toContain("<div>raw</div>");
    expect(JSON.stringify(doc)).toContain("::rara{a=1}");
    expect(JSON.stringify(doc)).toContain("mal");
    expect(JSON.stringify(doc)).not.toContain("javascript:");
  });

  it("maps headings into levels 2-4 with warnings", () => {
    const { doc, warnings } = markdownToDoc("# uno\n\n###### seis\n\n## dos\n");
    expect(doc.content.map((b) => (b.type === "heading" ? b.attrs.level : 0))).toEqual([2, 4, 2]);
    expect(warnings).toHaveLength(2);
  });

  it("never loads images or fetches URLs: keeps the alt text", () => {
    const { doc, warnings } = markdownToDoc("![Placa ESP32](https://example.com/p.png)\n");
    expect(doc.content[0]).toEqual({
      type: "paragraph",
      content: [{ type: "text", text: "Placa ESP32", marks: [{ type: "link", attrs: { href: "https://example.com/p.png" } }] }],
    });
    expect(warnings[0].message).toContain("WEB-004");
  });

  it("drops unsafe link targets from reference definitions too", () => {
    const { doc } = markdownToDoc("[x][r]\n\n[r]: data:text/html,hi\n");
    expect(JSON.stringify(doc)).not.toContain("data:");
  });

  it("keeps text that happens to look like a directive", () => {
    const { doc } = markdownToDoc("Nota:importante a las 10:30 y ratio 3:2\n");
    expect(doc.content[0]).toEqual({ type: "paragraph", content: [{ type: "text", text: "Nota:importante a las 10:30 y ratio 3:2" }] });
  });

  it("warns about table alignment and code metadata", () => {
    expect(messages("| a | b |\n|:--|--:|\n| 1 | 2 |\n")).toEqual(["La alineación de columnas de la tabla se omite."]);
    expect(messages("```go title=main.go\nx\n```\n")).toEqual(["Los metadatos del bloque de código se omiten."]);
  });

  it("rejects invalid embeds as text", () => {
    const { doc, warnings } = markdownToDoc('::youtube{video="nope"}\n\n::image{asset="fake"}\n');
    expect(doc.content.every((b) => b.type === "paragraph")).toBe(true);
    expect(warnings).toHaveLength(2);
  });

  it("never throws on odd input", () => {
    for (const input of ["", "\u0000\u0001", "> ".repeat(500) + "x", "[".repeat(10000), "|\n|-\n|", "\r\n\r\n# t\r\n"]) {
      expect(() => markdownToDoc(input)).not.toThrow();
    }
    expect(markdownToDoc("").doc).toEqual({ type: "doc", content: [] });
  });
});
