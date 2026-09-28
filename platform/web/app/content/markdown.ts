// Markdown import/export for the canonical document (web-v1.md §7, §7.1).
//
// Dialect: CommonMark + GFM tables + leaf directives for embeds:
//   ::youtube{video=dQw4w9WgXcQ start=42}
//   ::image{asset=<uuid> alt="…" caption="…"}   ::video{asset=<uuid> poster=<uuid> caption="…"}
//   ::download{asset=<uuid> label="…"}
// Conversions are explicit over mdast (remark utilities) instead of Tiptap's beta Markdown
// extension, which corrupted fenced code containing ``` and escaped pipes in tables during the
// WEB-003 probe. Import never fetches URLs and never drops content silently: anything unsupported
// is kept as literal text and reported as a warning.

import type * as M from "mdast";
import { fromMarkdown } from "mdast-util-from-markdown";
import { toMarkdown } from "mdast-util-to-markdown";
import { gfmTable } from "micromark-extension-gfm-table";
import { gfmTableFromMarkdown, gfmTableToMarkdown } from "mdast-util-gfm-table";
import { directive } from "micromark-extension-directive";
import { directiveFromMarkdown, directiveToMarkdown, type LeafDirective } from "mdast-util-directive";
import {
  MARK_ORDER,
  isSafeHref,
  isVideoId,
  languagePattern,
  uuidPattern,
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

export type ImportWarning = { line?: number; message: string };
export type ImportResult = { doc: Doc; warnings: ImportWarning[] };

// --- Export ---------------------------------------------------------------------------------------

/** Serializes a canonical document. Empty paragraphs (visual spacing only) are not exported. */
export function docToMarkdown(doc: Doc): string {
  const root: M.Root = { type: "root", children: doc.content.flatMap(blockToMdast) as M.RootContent[] };
  return toMarkdown(root, {
    extensions: [gfmTableToMarkdown(), directiveToMarkdown()],
    bullet: "-",
    emphasis: "*",
    strong: "*",
    fence: "`",
    fences: true,
    rule: "-",
    listItemIndent: "one",
    incrementListMarker: true,
  });
}

function blockToMdast(b: Block): M.RootContent[] {
  switch (b.type) {
    case "paragraph":
      return b.content?.length ? [{ type: "paragraph", children: inlineToMdast(b.content) }] : [];
    case "heading":
      return [{ type: "heading", depth: b.attrs.level, children: inlineToMdast(b.content ?? []) }];
    case "bulletList":
    case "orderedList":
      return [
        {
          type: "list",
          ordered: b.type === "orderedList",
          start: b.type === "orderedList" ? (b.attrs?.start ?? 1) : null,
          spread: false,
          children: b.content.map(listItemToMdast),
        },
      ];
    case "blockquote":
      return [{ type: "blockquote", children: b.content.flatMap(blockToMdast) as M.BlockContent[] }];
    case "codeBlock":
      return [{ type: "code", lang: b.attrs?.language ?? null, value: (b.content ?? []).map((t) => t.text).join("") }];
    case "horizontalRule":
      return [{ type: "thematicBreak" }];
    case "table":
      return [
        {
          type: "table",
          align: b.content[0].content.map(() => null),
          children: b.content.map((row) => ({
            type: "tableRow",
            children: row.content.map((cell) => ({
              type: "tableCell",
              children: inlineToMdast(cell.content[0].content ?? []) as M.PhrasingContent[],
            })),
          })),
        },
      ];
    case "youtube":
      return [leaf("youtube", { video: b.attrs.videoId, start: b.attrs.start?.toString() })];
    case "image":
      return [leaf("image", { asset: b.attrs.assetId, alt: b.attrs.alt, caption: b.attrs.caption })];
    case "video":
      return [leaf("video", { asset: b.attrs.assetId, poster: b.attrs.posterAssetId, caption: b.attrs.caption })];
    case "download":
      return [leaf("download", { asset: b.attrs.assetId, label: b.attrs.label })];
  }
}

function leaf(name: string, attrs: Record<string, string | undefined>): M.RootContent {
  const attributes: Record<string, string> = {};
  for (const [k, v] of Object.entries(attrs)) if (v !== undefined) attributes[k] = v;
  return { type: "leafDirective", name, attributes, children: [] } as LeafDirective as M.RootContent;
}

function listItemToMdast(item: ListItem): M.ListItem {
  return { type: "listItem", spread: false, children: item.content.flatMap(blockToMdast) as M.BlockContent[] };
}

// Inline marks become nested mdast nodes, grouping neighbours that share the outer mark so
// "[**a** b](u)" stays one link: link → strong → emphasis → inlineCode.
function inlineToMdast(nodes: Inline[]): M.PhrasingContent[] {
  return group(nodes, 0);
}

function markKey(n: Inline, level: number): string | null {
  if (n.type !== "text") return null;
  const type = MARK_ORDER[level];
  const m = n.marks?.find((x) => x.type === type);
  if (!m) return null;
  return m.type === "link" ? `link:${m.attrs.href}` : type;
}

function group(nodes: Inline[], level: number): M.PhrasingContent[] {
  if (level >= 3) {
    return nodes.map((n): M.PhrasingContent => {
      if (n.type === "hardBreak") return { type: "break" };
      return n.marks?.some((m) => m.type === "code") ? { type: "inlineCode", value: n.text } : { type: "text", value: n.text };
    });
  }
  const out: M.PhrasingContent[] = [];
  let i = 0;
  while (i < nodes.length) {
    const key = markKey(nodes[i], level);
    let j = i + 1;
    while (j < nodes.length && markKey(nodes[j], level) === key) j++;
    const children = group(nodes.slice(i, j), level + 1);
    if (key === null) out.push(...children);
    else if (level === 0) out.push({ type: "link", url: key.slice(5), children: children as M.PhrasingContent[] } as M.Link);
    else if (level === 1) out.push({ type: "strong", children });
    else out.push({ type: "emphasis", children });
    i = j;
  }
  return out;
}

// --- Import ---------------------------------------------------------------------------------------

type Ctx = { source: string; warnings: ImportWarning[]; definitions: Map<string, string> };

/** Parses Markdown into a canonical document plus warnings. Never throws on odd input. */
export function markdownToDoc(markdown: string): ImportResult {
  const source = markdown.replace(/\r\n?/g, "\n");
  const tree = fromMarkdown(source, {
    extensions: [gfmTable(), directive()],
    mdastExtensions: [gfmTableFromMarkdown(), directiveFromMarkdown()],
  });
  const ctx: Ctx = { source, warnings: [], definitions: new Map() };
  for (const node of tree.children) {
    if (node.type === "definition") ctx.definitions.set(node.identifier, node.url);
  }
  const content = blocks(tree.children, ctx);
  return { doc: { type: "doc", content }, warnings: ctx.warnings };
}

function warn(ctx: Ctx, node: { position?: M.Node["position"] } | undefined, message: string) {
  ctx.warnings.push({ line: node?.position?.start.line, message });
}

function literal(ctx: Ctx, node: M.Node): string {
  const start = node.position?.start.offset;
  const end = node.position?.end.offset;
  return start !== undefined && end !== undefined ? ctx.source.slice(start, end) : "";
}

function literalParagraph(ctx: Ctx, node: M.Node): Paragraph[] {
  const text = literal(ctx, node);
  return text ? [{ type: "paragraph", content: [{ type: "text", text }] }] : [];
}

function blocks(nodes: M.Node[], ctx: Ctx): Block[] {
  return nodes.flatMap((n) => block(n, ctx));
}

function block(node: M.Node, ctx: Ctx): Block[] {
  switch (node.type) {
    case "paragraph": {
      const content = inline((node as M.Paragraph).children, [], ctx, true);
      return content.length ? [{ type: "paragraph", content }] : [];
    }
    case "heading": {
      const h = node as M.Heading;
      let level = h.depth;
      if (level === 1) {
        warn(ctx, h, "Título de nivel 1 convertido a nivel 2 (el título del contenido va aparte).");
        level = 2;
      } else if (level > 4) {
        warn(ctx, h, `Título de nivel ${level} convertido a nivel 4.`);
        level = 4;
      }
      const content = inline(h.children, [], ctx, true);
      return [{ type: "heading", attrs: { level: level as 2 | 3 | 4 }, ...(content.length ? { content } : {}) }];
    }
    case "thematicBreak":
      return [{ type: "horizontalRule" }];
    case "blockquote": {
      const content = blocks((node as M.Blockquote).children, ctx).filter(allowedInQuote(ctx, node));
      return content.length ? [{ type: "blockquote", content }] : [];
    }
    case "list": {
      const l = node as M.List;
      const items: ListItem[] = l.children.map((item) => {
        // Task-list syntax ("- [ ] x") is not parsed, so it stays as literal text.
        const content = blocks(item.children, ctx).filter(allowedInListItem(ctx, item));
        return { type: "listItem", content: content.length ? content : [{ type: "paragraph" }] };
      });
      if (l.ordered) {
        const start = l.start ?? 1;
        return [{ type: "orderedList", ...(start > 1 ? { attrs: { start } } : {}), content: items }];
      }
      return [{ type: "bulletList", content: items }];
    }
    case "code": {
      const c = node as M.Code;
      let language = c.lang ?? "";
      if (language && !languagePattern.test(language)) {
        const lower = language.toLowerCase();
        if (languagePattern.test(lower)) language = lower;
        else {
          warn(ctx, c, `Lenguaje de código «${language}» no válido; se omite.`);
          language = "";
        }
      }
      if (c.meta) warn(ctx, c, "Los metadatos del bloque de código se omiten.");
      return [
        {
          type: "codeBlock",
          ...(language ? { attrs: { language } } : {}),
          ...(c.value ? { content: [{ type: "text", text: c.value }] } : {}),
        },
      ];
    }
    case "table":
      return table(node as M.Table, ctx);
    case "leafDirective":
      return directiveBlock(node as LeafDirective, ctx);
    case "html":
      warn(ctx, node, "HTML no permitido; se conserva como texto.");
      return literalParagraph(ctx, node);
    case "definition":
      return [];
    default:
      warn(ctx, node, `Sintaxis no soportada (${node.type}); se conserva como texto.`);
      return literalParagraph(ctx, node);
  }
}

function allowedInListItem(ctx: Ctx, parent: M.Node) {
  const ok = new Set(["paragraph", "bulletList", "orderedList", "codeBlock", "blockquote"]);
  return (b: Block) => keepOrWarn(ctx, parent, b, ok);
}

function allowedInQuote(ctx: Ctx, parent: M.Node) {
  const ok = new Set(["paragraph", "heading", "bulletList", "orderedList", "codeBlock", "blockquote"]);
  return (b: Block) => keepOrWarn(ctx, parent, b, ok);
}

function keepOrWarn(ctx: Ctx, parent: M.Node, b: Block, ok: Set<string>): boolean {
  if (ok.has(b.type)) return true;
  warn(ctx, parent, `Un bloque «${b.type}» dentro de una lista o cita no está soportado y se omitió.`);
  return false;
}

function table(t: M.Table, ctx: Ctx): Block[] {
  if (t.align?.some((a) => a !== null)) warn(ctx, t, "La alineación de columnas de la tabla se omite.");
  if (t.children.length > 500 || t.children[0].children.length > 20) {
    warn(ctx, t, "Tabla demasiado grande (máximo 500 filas y 20 columnas); se conserva como texto.");
    return literalParagraph(ctx, t);
  }
  const width = t.children[0].children.length;
  const rows: TableRow[] = t.children.map((row, r) => {
    const cells: TableCell[] = [];
    for (let c = 0; c < width; c++) {
      const src = row.children[c];
      const content = src ? inline(src.children, [], ctx, false) : [];
      cells.push({ type: r === 0 ? "tableHeader" : "tableCell", content: [{ type: "paragraph", ...(content.length ? { content } : {}) }] });
    }
    if (row.children.length > width) warn(ctx, row, "Celdas sobrantes en una fila de tabla se omitieron.");
    return { type: "tableRow", content: cells };
  });
  return [{ type: "table", content: rows }];
}

function directiveBlock(d: LeafDirective, ctx: Ctx): Block[] {
  const a = d.attributes ?? {};
  const unsupported = (why: string) => {
    warn(ctx, d, why);
    return literalParagraph(ctx, d);
  };
  switch (d.name) {
    case "youtube": {
      const id = a.video ?? "";
      if (!isVideoId(id)) return unsupported("Vídeo de YouTube con id no válido; se conserva como texto.");
      const start = a.start ? Number(a.start) : 0;
      if (a.start && !(Number.isInteger(start) && start >= 0 && start <= 86400)) {
        return unsupported("Inicio de YouTube no válido; se conserva como texto.");
      }
      return [{ type: "youtube", attrs: start > 0 ? { videoId: id, start } : { videoId: id } }];
    }
    case "image":
    case "video":
    case "download": {
      const asset = a.asset ?? "";
      if (!uuidPattern.test(asset)) return unsupported(`«${d.name}» sin un asset válido; se conserva como texto.`);
      warn(ctx, d, `«${d.name}» requiere archivos subidos (WEB-004); el servidor lo rechazará hasta entonces.`);
      if (d.name === "image") {
        return [{ type: "image", attrs: { assetId: asset, alt: a.alt ?? "", ...(a.caption ? { caption: a.caption } : {}) } }];
      }
      if (d.name === "video") {
        const poster = a.poster && uuidPattern.test(a.poster) ? { posterAssetId: a.poster } : {};
        return [{ type: "video", attrs: { assetId: asset, ...poster, ...(a.caption ? { caption: a.caption } : {}) } }];
      }
      if (!a.label?.trim()) return unsupported("Descarga sin etiqueta; se conserva como texto.");
      return [{ type: "download", attrs: { assetId: asset, label: a.label } }];
    }
    default:
      return unsupported(`Directiva «${d.name}» desconocida; se conserva como texto.`);
  }
}

function inline(nodes: M.Node[], marks: Mark[], ctx: Ctx, allowBreak: boolean): Inline[] {
  const out: Inline[] = [];
  const text = (value: string, extra: Mark[] = []) => {
    if (value) out.push(textNode(value, [...marks, ...extra]));
  };
  for (const node of nodes) {
    switch (node.type) {
      case "text":
        text((node as M.Text).value);
        break;
      case "strong":
        out.push(...inline((node as M.Strong).children, withMark(marks, { type: "bold" }), ctx, allowBreak));
        break;
      case "emphasis":
        out.push(...inline((node as M.Emphasis).children, withMark(marks, { type: "italic" }), ctx, allowBreak));
        break;
      case "inlineCode":
        text((node as M.InlineCode).value, [{ type: "code" }]);
        break;
      case "break":
        if (allowBreak) out.push({ type: "hardBreak" });
        else {
          warn(ctx, node, "Saltos de línea dentro de celdas no soportados; se reemplazan por un espacio.");
          text(" ");
        }
        break;
      case "link":
      case "linkReference": {
        const url = node.type === "link" ? (node as M.Link).url : ctx.definitions.get((node as M.LinkReference).identifier);
        const children = (node as M.Link).children;
        if (url && isSafeHref(url)) {
          out.push(...inline(children, withMark(marks, { type: "link", attrs: { href: url } }), ctx, allowBreak));
        } else {
          warn(ctx, node, url ? `Enlace no permitido («${url.slice(0, 60)}»); se conserva sólo el texto.` : "Referencia de enlace sin definición; se conserva el texto.");
          out.push(...inline(children, marks, ctx, allowBreak));
        }
        break;
      }
      case "image":
      case "imageReference": {
        const img = node as M.Image;
        warn(ctx, node, "Las imágenes necesitan archivos subidos (WEB-004); se conserva el texto alternativo.");
        const url = node.type === "image" ? img.url : ctx.definitions.get((node as M.ImageReference).identifier);
        const alt = img.alt || "imagen";
        text(alt, url && isSafeHref(url) ? [{ type: "link", attrs: { href: url } }] : []);
        break;
      }
      case "html":
        warn(ctx, node, "HTML no permitido; se conserva como texto.");
        text((node as M.Html).value);
        break;
      default:
        // textDirective (e.g. "nota:importante") and anything else: keep the exact source text.
        text(literal(ctx, node));
    }
  }
  return mergeText(out);
}

function withMark(marks: Mark[], mark: Mark): Mark[] {
  return marks.some((m) => m.type === mark.type) ? marks : [...marks, mark];
}

function textNode(text: string, marks: Mark[]): TextNode {
  const sorted = [...marks].sort((a, b) => MARK_ORDER.indexOf(a.type) - MARK_ORDER.indexOf(b.type));
  return sorted.length ? { type: "text", text, marks: sorted } : { type: "text", text };
}

function sameMarks(a: TextNode, b: TextNode): boolean {
  return JSON.stringify(a.marks ?? []) === JSON.stringify(b.marks ?? []);
}

function mergeText(nodes: Inline[]): Inline[] {
  const out: Inline[] = [];
  for (const n of nodes) {
    const prev = out[out.length - 1];
    if (n.type === "text" && prev?.type === "text" && sameMarks(prev, n)) {
      out[out.length - 1] = { ...prev, text: prev.text + n.text };
    } else out.push(n);
  }
  return out;
}
