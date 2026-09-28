// Tiptap schema for the admin editor. It is restricted to the canonical document v1 (web-v1.md §7.1):
// anything the editor can produce must be convertible by tiptap-adapter.ts and accepted by Go.

import { Node, mergeAttributes, type Extensions } from "@tiptap/core";
import StarterKit from "@tiptap/starter-kit";
import { TableKit } from "@tiptap/extension-table";
import { isSafeHref } from "./schema";

function atomBlock(name: string, attrs: string[], label: (a: Record<string, unknown>) => string) {
  return Node.create({
    name,
    group: "block",
    atom: true,
    selectable: true,
    draggable: false,
    addAttributes() {
      return Object.fromEntries(attrs.map((a) => [a, { default: null }]));
    },
    // Only our own markup is parsed back; pasted iframes or foreign HTML never match.
    parseHTML() {
      return [
        {
          tag: `div[data-bl-node="${name}"]`,
          getAttrs: (el) => Object.fromEntries(attrs.map((a) => [a, (el as HTMLElement).getAttribute(`data-${a.toLowerCase()}`)])),
        },
      ];
    },
    renderHTML({ node, HTMLAttributes }) {
      const data = Object.fromEntries(attrs.map((a) => [`data-${a.toLowerCase()}`, node.attrs[a] ?? ""]));
      const frame = mergeAttributes(HTMLAttributes, data, {
        "data-bl-node": name,
        class: "my-4 flex items-center gap-3 rounded border border-dashed border-border-strong bg-surface-muted p-2 text-sm text-text-muted",
        contenteditable: "false",
      });
      // Owner-only preview through /media (the session cookie authorizes it).
      const thumbId = name === "image" ? node.attrs.assetId : name === "video" ? node.attrs.posterAssetId : null;
      if (thumbId) {
        return ["div", frame, ["img", { src: `/media/${thumbId}`, alt: "", class: "h-16 w-24 rounded object-cover", draggable: "false" }], ["span", {}, label(node.attrs)]];
      }
      return ["div", frame, ["span", {}, label(node.attrs)]];
    },
  });
}

export const YouTube = atomBlock("youtube", ["videoId", "start"], (a) => `YouTube · ${a.videoId}${a.start ? ` · desde ${a.start} s` : ""}`);
// Media nodes reference library assets by id (WEB-004); Go checks each asset on save.
export const Image = atomBlock("image", ["assetId", "alt", "caption"], (a) => `Imagen · ${a.alt || "sin texto alternativo"}${a.caption ? ` · ${a.caption}` : ""}`);
export const Video = atomBlock("video", ["assetId", "posterAssetId", "caption"], (a) => `Vídeo · ${a.caption || "sin pie"}${a.posterAssetId ? "" : " · sin póster"}`);
export const Download = atomBlock("download", ["assetId", "label"], (a) => `Descarga · ${a.label}`);

export const editorExtensions: Extensions = [
  StarterKit.configure({
    heading: { levels: [2, 3, 4] },
    strike: false,
    underline: false,
    link: {
      openOnClick: false,
      autolink: true,
      defaultProtocol: "https",
      isAllowedUri: (url) => isSafeHref(url),
      HTMLAttributes: { rel: null, target: null, class: null },
    },
  }),
  TableKit.configure({ table: { resizable: false } }),
  YouTube,
  Image,
  Video,
  Download,
];
