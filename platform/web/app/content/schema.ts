// Canonical document v1 (web-v1.md §7.1). The Go API validates it strictly; these types and helpers
// mirror its rules so the editor and the Markdown importer can fail early with clear messages.

export type MarkType = "link" | "bold" | "italic" | "code";

export type Mark =
  | { type: "link"; attrs: { href: string } }
  | { type: "bold" | "italic" | "code" };

export type TextNode = { type: "text"; text: string; marks?: Mark[] };
export type HardBreak = { type: "hardBreak" };
export type Inline = TextNode | HardBreak;

export type Paragraph = { type: "paragraph"; content?: Inline[] };
export type Heading = { type: "heading"; attrs: { level: 2 | 3 | 4 }; content?: Inline[] };
export type ListItem = { type: "listItem"; content: Block[] };
export type BulletList = { type: "bulletList"; content: ListItem[] };
export type OrderedList = { type: "orderedList"; attrs?: { start: number }; content: ListItem[] };
export type Blockquote = { type: "blockquote"; content: Block[] };
export type CodeBlock = { type: "codeBlock"; attrs?: { language: string }; content?: TextNode[] };
export type HorizontalRule = { type: "horizontalRule" };
export type TableCell = { type: "tableHeader" | "tableCell"; content: [Paragraph] };
export type TableRow = { type: "tableRow"; content: TableCell[] };
export type Table = { type: "table"; content: TableRow[] };
export type YouTube = { type: "youtube"; attrs: { videoId: string; start?: number } };
// Media nodes are versioned now but only accepted once assets exist (WEB-004).
export type Image = { type: "image"; attrs: { assetId: string; alt: string; caption?: string } };
export type Video = { type: "video"; attrs: { assetId: string; posterAssetId?: string; caption?: string } };
export type Download = { type: "download"; attrs: { assetId: string; label: string } };
export type MediaNode = Image | Video | Download;

export type Block =
  | Paragraph
  | Heading
  | BulletList
  | OrderedList
  | Blockquote
  | CodeBlock
  | HorizontalRule
  | Table
  | YouTube
  | MediaNode;

export type Doc = { type: "doc"; content: Block[] };

export const SCHEMA_VERSION = 1;
export const MARK_ORDER: MarkType[] = ["link", "bold", "italic", "code"];

export const emptyDoc = (): Doc => ({ type: "doc", content: [] });

const videoIdPattern = /^[A-Za-z0-9_-]{11}$/;
export const languagePattern = /^[a-z0-9][a-z0-9+#._-]{0,31}$/;
export const uuidPattern = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/;

export function isVideoId(id: string): boolean {
  return videoIdPattern.test(id);
}

/** Extracts the video id from a YouTube URL (watch, youtu.be, embed, shorts) or a bare id. */
export function parseYouTube(input: string): { videoId: string; start?: number } | null {
  const value = input.trim();
  if (isVideoId(value)) return { videoId: value };
  let url: URL;
  try {
    url = new URL(value);
  } catch {
    return null;
  }
  const host = url.hostname.replace(/^(www\.|m\.)/, "");
  let id: string | null = null;
  if (host === "youtu.be") id = url.pathname.slice(1);
  else if (host === "youtube.com" || host === "youtube-nocookie.com") {
    if (url.pathname === "/watch") id = url.searchParams.get("v");
    else {
      const m = url.pathname.match(/^\/(embed|shorts|live)\/([^/]+)/);
      id = m ? m[2] : null;
    }
  }
  if (!id || !isVideoId(id)) return null;
  const t = url.searchParams.get("t") ?? url.searchParams.get("start");
  const start = t ? Number.parseInt(t, 10) : NaN;
  return Number.isInteger(start) && start > 0 && start <= 86400 ? { videoId: id, start } : { videoId: id };
}

/** Same rules as the Go validator: http(s), mailto, site-relative paths and anchors only. */
export function isSafeHref(href: string): boolean {
  if (!href || href.length > 2000) return false;
  for (const ch of href) {
    const c = ch.codePointAt(0)!;
    if (c < 0x20 || c === 0x7f || ch === "\\") return false;
  }
  if (href.startsWith("#") || (href.startsWith("/") && !href.startsWith("//"))) return true;
  let url: URL;
  try {
    url = new URL(href);
  } catch {
    return false;
  }
  switch (url.protocol) {
    case "http:":
    case "https:":
      return url.host !== "" && url.username === "" && url.password === "";
    case "mailto:":
      return url.pathname !== "";
    default:
      return false;
  }
}
