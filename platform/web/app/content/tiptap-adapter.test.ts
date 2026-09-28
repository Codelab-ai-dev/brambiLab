// @vitest-environment jsdom
import { readdirSync, readFileSync } from "node:fs";
import { join } from "node:path";
import { Editor } from "@tiptap/core";
import { afterEach, describe, expect, it } from "vitest";
import { editorExtensions } from "./editor-extensions";
import { docToMarkdown, markdownToDoc } from "./markdown";
import type { Doc } from "./schema";
import { UnsupportedContentError, toCanonical, toTiptap } from "./tiptap-adapter";

const dir = join(__dirname, "../../../contracts/fixtures/documents");
const fixtures: [string, Doc][] = readdirSync(dir)
  .filter((f) => f.endsWith(".json"))
  .map((f) => [f.replace(".json", ""), JSON.parse(readFileSync(join(dir, f), "utf8"))]);

let editors: Editor[] = [];
function editorWith(content: Doc | string): Editor {
  const e = new Editor({ extensions: editorExtensions, content: typeof content === "string" ? content : toTiptap(content) });
  editors.push(e);
  return e;
}
afterEach(() => {
  editors.forEach((e) => e.destroy());
  editors = [];
});

describe("editor ↔ canonical", () => {
  for (const [name, doc] of fixtures) {
    it(`${name}: loads into the real editor and comes back unchanged`, () => {
      const { doc: back, notes } = toCanonical(editorWith(doc).getJSON());
      expect(back).toEqual(doc);
      expect(notes).toEqual([]);
    });

    it(`${name}: editor → Markdown → editor keeps the content`, () => {
      const fromEditor = toCanonical(editorWith(doc).getJSON()).doc;
      const imported = markdownToDoc(docToMarkdown(fromEditor)).doc;
      const again = toCanonical(editorWith(imported).getJSON()).doc;
      expect(again).toEqual(imported);
      expect(docToMarkdown(again)).toBe(docToMarkdown(fromEditor));
    });
  }
});

describe("pasted HTML is reduced to the canonical subset", () => {
  it("drops scripts, iframes, event handlers and executable links", () => {
    const html = `<p>ok <a href="javascript:alert(1)">malo</a> <a href="https://example.com" onclick="x()">bueno</a></p>
      <script>alert(1)</script><iframe src="https://evil.example"></iframe>
      <p style="color:red"><span onmouseover="x()">texto</span> <s>tachado</s> <u>subrayado</u></p>
      <h1>Uno</h1><img src="https://evil.example/x.png" onerror="x()">`;
    const { doc } = toCanonical(editorWith(html).getJSON());
    const json = JSON.stringify(doc);
    for (const bad of ["javascript:", "script", "iframe", "onclick", "onmouseover", "onerror", "style", "evil.example"]) {
      expect(json).not.toContain(bad);
    }
    expect(json).toContain('"href":"https://example.com"');
    expect(json).toContain("tachado");
    expect(json).toContain("subrayado");
    // <h1> is not an editor level: it becomes a paragraph, never a level-1 heading.
    expect(json).not.toContain('"level":1');
  });
});

describe("normalizations are explicit", () => {
  it("rejects merged table cells", () => {
    const e = editorWith("<table><tr><th colspan='2'>a</th></tr><tr><td>1</td><td>2</td></tr></table>");
    expect(() => toCanonical(e.getJSON())).toThrow(UnsupportedContentError);
  });

  it("joins several lines in a cell and reports it", () => {
    const e = editorWith("<table><tr><th>a</th></tr><tr><td><p>uno</p><p>dos</p></td></tr></table>");
    const { doc, notes } = toCanonical(e.getJSON());
    expect(JSON.stringify(doc)).toContain('"text":"uno dos"');
    expect(notes).toContain("Varias líneas de una celda se unieron con espacios.");
  });

  it("uses the first row as header and reports it", () => {
    const e = editorWith("<table><tr><td>a</td></tr><tr><td>1</td></tr></table>");
    const { doc, notes } = toCanonical(e.getJSON());
    const table = doc.content[0];
    expect(table.type === "table" && table.content[0].content[0].type).toBe("tableHeader");
    expect(notes).toContain("La primera fila de la tabla se usa como encabezado.");
  });
});
