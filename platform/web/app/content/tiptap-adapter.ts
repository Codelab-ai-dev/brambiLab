// Converts between Tiptap's JSON and the canonical document v1. The canonical form is what the API
// stores; Tiptap's JSON never leaves the browser. Defaults Tiptap adds (colspan: 1, language: null,
// link target/rel, …) are dropped; structures the canonical format cannot express raise an error
// or are normalized explicitly (never silently).

import type { JSONContent } from "@tiptap/core";
import {
  MARK_ORDER,
  isSafeHref,
  type Block,
  type Doc,
  type Inline,
  type ListItem,
  type Mark,
  type Paragraph,
  type TableCell,
  type TableRow,
  type TextNode,
} from "./schema";

export class UnsupportedContentError extends Error {}

export type AdapterNote = string;

/** Tiptap → canonical. Notes describe normalizations applied (e.g. a cell with two paragraphs). */
export function toCanonical(json: JSONContent): { doc: Doc; notes: AdapterNote[] } {
  const notes: AdapterNote[] = [];
  if (json.type !== "doc") throw new UnsupportedContentError("El documento no tiene raíz «doc».");
  return { doc: { type: "doc", content: blocks(json.content ?? [], notes) }, notes };
}

function blocks(nodes: JSONContent[], notes: AdapterNote[]): Block[] {
  return nodes.map((n) => block(n, notes));
}

function block(n: JSONContent, notes: AdapterNote[]): Block {
  const a = n.attrs ?? {};
  switch (n.type) {
    case "paragraph": {
      const content = inline(n.content ?? [], notes);
      return content.length ? { type: "paragraph", content } : { type: "paragraph" };
    }
    case "heading": {
      const level = Number(a.level);
      if (level !== 2 && level !== 3 && level !== 4) throw new UnsupportedContentError(`Nivel de título ${a.level} no soportado.`);
      const content = inline(n.content ?? [], notes);
      return { type: "heading", attrs: { level }, ...(content.length ? { content } : {}) };
    }
    case "bulletList":
      return { type: "bulletList", content: listItems(n, notes) };
    case "orderedList": {
      const start = Number(a.start ?? 1);
      return { type: "orderedList", ...(start > 1 ? { attrs: { start } } : {}), content: listItems(n, notes) };
    }
    case "blockquote":
      return { type: "blockquote", content: blocks(n.content ?? [], notes) };
    case "codeBlock": {
      const text = (n.content ?? []).map((t) => t.text ?? "").join("");
      const language = typeof a.language === "string" ? a.language : "";
      return {
        type: "codeBlock",
        ...(language ? { attrs: { language } } : {}),
        ...(text ? { content: [{ type: "text", text }] } : {}),
      };
    }
    case "horizontalRule":
      return { type: "horizontalRule" };
    case "table":
      return table(n, notes);
    case "youtube": {
      const start = Number(a.start);
      return { type: "youtube", attrs: { videoId: String(a.videoId), ...(start > 0 ? { start } : {}) } };
    }
    case "image":
      return { type: "image", attrs: { assetId: String(a.assetId), alt: String(a.alt ?? ""), ...(a.caption ? { caption: String(a.caption) } : {}) } };
    case "video":
      return {
        type: "video",
        attrs: { assetId: String(a.assetId), ...(a.posterAssetId ? { posterAssetId: String(a.posterAssetId) } : {}), ...(a.caption ? { caption: String(a.caption) } : {}) },
      };
    case "download":
      return { type: "download", attrs: { assetId: String(a.assetId), label: String(a.label ?? "") } };
    default:
      throw new UnsupportedContentError(`Bloque «${n.type}» no soportado.`);
  }
}

function listItems(n: JSONContent, notes: AdapterNote[]): ListItem[] {
  return (n.content ?? []).map((item) => ({ type: "listItem", content: blocks(item.content ?? [], notes) }));
}

// Canonical tables are simple: a header row, then body rows, one paragraph per cell.
function table(n: JSONContent, notes: AdapterNote[]): Block {
  const rows: TableRow[] = (n.content ?? []).map((row, r) => ({
    type: "tableRow",
    content: (row.content ?? []).map((cell): TableCell => {
      const ca = cell.attrs ?? {};
      if ((ca.colspan ?? 1) !== 1 || (ca.rowspan ?? 1) !== 1) {
        throw new UnsupportedContentError("Las celdas combinadas no están soportadas; sepáralas antes de guardar.");
      }
      const paragraphs = cell.content ?? [];
      if (paragraphs.some((p) => p.type !== "paragraph")) {
        throw new UnsupportedContentError("Las celdas de tabla sólo admiten texto.");
      }
      if (paragraphs.length > 1) notes.push("Varias líneas de una celda se unieron con espacios.");
      const content: Inline[] = [];
      paragraphs.forEach((p, i) => {
        if (i > 0 && content.length) content.push({ type: "text", text: " " });
        content.push(...inline(p.content ?? [], notes).map((x) => (x.type === "hardBreak" ? { type: "text" as const, text: " " } : x)));
      });
      const merged = mergeText(content);
      const want = r === 0 ? "tableHeader" : "tableCell";
      if (cell.type !== want) notes.push(r === 0 ? "La primera fila de la tabla se usa como encabezado." : "Sólo la primera fila de la tabla puede ser encabezado.");
      const paragraph: Paragraph = merged.length ? { type: "paragraph", content: merged } : { type: "paragraph" };
      return { type: want, content: [paragraph] };
    }),
  }));
  return { type: "table", content: rows };
}

function inline(nodes: JSONContent[], notes: AdapterNote[]): Inline[] {
  const out: Inline[] = [];
  for (const n of nodes) {
    if (n.type === "hardBreak") out.push({ type: "hardBreak" });
    else if (n.type === "text" && n.text) out.push(textNode(n, notes));
    else if (n.type !== "text") throw new UnsupportedContentError(`Elemento «${n.type}» no soportado dentro del texto.`);
  }
  return mergeText(out);
}

function textNode(n: JSONContent, notes: AdapterNote[]): TextNode {
  const marks: Mark[] = [];
  for (const m of n.marks ?? []) {
    switch (m.type) {
      case "bold":
      case "italic":
      case "code":
        marks.push({ type: m.type });
        break;
      case "link": {
        const href = String(m.attrs?.href ?? "");
        if (isSafeHref(href)) marks.push({ type: "link", attrs: { href } });
        else notes.push(`Se quitó un enlace no permitido («${href.slice(0, 60)}»).`);
        break;
      }
      default:
        throw new UnsupportedContentError(`Formato «${m.type}» no soportado.`);
    }
  }
  marks.sort((x, y) => MARK_ORDER.indexOf(x.type) - MARK_ORDER.indexOf(y.type));
  return marks.length ? { type: "text", text: n.text!, marks } : { type: "text", text: n.text! };
}

function mergeText(nodes: Inline[]): Inline[] {
  const out: Inline[] = [];
  for (const n of nodes) {
    const prev = out[out.length - 1];
    if (n.type === "text" && prev?.type === "text" && JSON.stringify(prev.marks ?? []) === JSON.stringify(n.marks ?? [])) {
      out[out.length - 1] = { ...prev, text: prev.text + n.text };
    } else out.push(n);
  }
  return out;
}

/** Canonical → Tiptap. The canonical node names already match the editor schema. */
export function toTiptap(doc: Doc): JSONContent {
  return JSON.parse(JSON.stringify(doc)) as JSONContent;
}
