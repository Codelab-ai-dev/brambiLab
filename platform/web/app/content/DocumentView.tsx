// Renders a canonical document (web-v1.md §7.1) as semantic HTML. The private preview uses it now
// and the public site will use it in WEB-006, so both show exactly the same representation.
// Everything is built from React elements: no HTML strings, no dangerouslySetInnerHTML.

import { useState, type ReactNode } from "react";
import { t, type Locale } from "~/i18n";
import { isSafeHref, type Block, type Doc, type Inline, type TextNode } from "./schema";

/** Stored-file facts used to reserve space and label downloads; optional (e.g. import preview). */
export type AssetMeta = { width: number | null; height: number | null; bytes: number };

type Props = { doc: Doc; locale: Locale; assets?: Record<string, AssetMeta> };

export function DocumentView({ doc, locale, assets = {} }: Props) {
  return <div className="space-y-4 leading-relaxed">{doc.content.map((b, i) => renderBlock(b, i, locale, assets))}</div>;
}

function formatSize(n: number): string {
  return n < 1024 * 1024 ? `${Math.max(1, Math.round(n / 1024))} KB` : `${(n / 1024 / 1024).toFixed(1)} MB`;
}

function renderBlock(b: Block, key: number, locale: Locale, assets: Record<string, AssetMeta> = {}): ReactNode {
  switch (b.type) {
    case "paragraph":
      // Empty paragraphs are spacing in the editor; keep the line so layouts match the preview.
      return <p key={key}>{b.content?.length ? renderInline(b.content) : <br />}</p>;
    case "heading": {
      const Tag = (["h2", "h3", "h4"] as const)[b.attrs.level - 2];
      const size = { 2: "text-2xl", 3: "text-xl", 4: "text-lg" }[b.attrs.level];
      return (
        <Tag key={key} className={`${size} mt-8 font-semibold`}>
          {renderInline(b.content ?? [])}
        </Tag>
      );
    }
    case "bulletList":
      return (
        <ul key={key} className="list-disc space-y-1 pl-6">
          {b.content.map((item, i) => (
            <li key={i}>{item.content.map((c, j) => renderBlock(c, j, locale, assets))}</li>
          ))}
        </ul>
      );
    case "orderedList":
      return (
        <ol key={key} start={b.attrs?.start} className="list-decimal space-y-1 pl-6">
          {b.content.map((item, i) => (
            <li key={i}>{item.content.map((c, j) => renderBlock(c, j, locale, assets))}</li>
          ))}
        </ol>
      );
    case "blockquote":
      return (
        <blockquote key={key} className="space-y-2 border-l-4 border-border pl-4 text-text-muted">
          {b.content.map((c, i) => renderBlock(c, i, locale, assets))}
        </blockquote>
      );
    case "codeBlock": {
      const lang = b.attrs?.language;
      return (
        <pre key={key} className="overflow-x-auto rounded-md bg-surface-muted p-4 font-mono text-sm" data-language={lang}>
          <code className={lang ? `language-${lang}` : undefined}>{(b.content ?? []).map((x) => x.text).join("")}</code>
        </pre>
      );
    }
    case "horizontalRule":
      return <hr key={key} className="border-border" />;
    case "table":
      return (
        // Wide tables scroll inside their own box instead of widening the page on mobile.
        <div key={key} className="overflow-x-auto" role="region" aria-label="Tabla" tabIndex={0}>
          <table className="w-full border-collapse text-sm">
            <thead>
              <tr>
                {b.content[0].content.map((cell, i) => (
                  <th key={i} scope="col" className="border border-border bg-surface-muted px-3 py-2 text-left font-semibold">
                    {renderInline(cell.content[0].content ?? [])}
                  </th>
                ))}
              </tr>
            </thead>
            <tbody>
              {b.content.slice(1).map((row, r) => (
                <tr key={r}>
                  {row.content.map((cell, i) => (
                    <td key={i} className="border border-border px-3 py-2 align-top">
                      {renderInline(cell.content[0].content ?? [])}
                    </td>
                  ))}
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      );
    case "youtube":
      return <YouTubeEmbed key={key} videoId={b.attrs.videoId} start={b.attrs.start} locale={locale} />;
    case "image": {
      const meta = assets[b.attrs.assetId];
      return (
        <figure key={key} className="my-6">
          <img
            src={`/media/${b.attrs.assetId}`}
            alt={b.attrs.alt}
            width={meta?.width ?? undefined}
            height={meta?.height ?? undefined}
            loading="lazy"
            decoding="async"
            className="h-auto max-w-full rounded-md"
          />
          {b.attrs.caption && <figcaption className="mt-2 text-sm text-text-muted">{b.attrs.caption}</figcaption>}
        </figure>
      );
    }
    case "video": {
      const meta = assets[b.attrs.assetId];
      return (
        <figure key={key} className="my-6">
          <video
            src={`/media/${b.attrs.assetId}`}
            poster={b.attrs.posterAssetId ? `/media/${b.attrs.posterAssetId}` : undefined}
            width={meta?.width ?? undefined}
            height={meta?.height ?? undefined}
            controls
            preload="metadata"
            playsInline
            className="w-full rounded-md bg-surface-muted"
          />
          {b.attrs.caption && <figcaption className="mt-2 text-sm text-text-muted">{b.attrs.caption}</figcaption>}
        </figure>
      );
    }
    case "download": {
      const meta = assets[b.attrs.assetId];
      return (
        <p key={key}>
          <a
            href={`/media/${b.attrs.assetId}/download`}
            className="inline-flex items-center gap-3 rounded-md border border-border-strong px-4 py-2 font-medium hover:bg-surface-muted focus-visible:outline-2 focus-visible:outline-accent"
          >
            <span aria-hidden="true">↓</span>
            {b.attrs.label}
            {meta && <span className="font-mono text-xs font-normal text-text-muted">{formatSize(meta.bytes)}</span>}
          </a>
        </p>
      );
    }
  }
}

function renderInline(nodes: Inline[]): ReactNode[] {
  return nodes.map((n, i) => (n.type === "hardBreak" ? <br key={i} /> : <Text key={i} node={n} />));
}

function Text({ node }: { node: TextNode }) {
  let out: ReactNode = node.text;
  const has = (type: string) => node.marks?.some((m) => m.type === type);
  if (has("code")) out = <code className="rounded bg-surface-muted px-1 font-mono text-[0.9em]">{out}</code>;
  if (has("italic")) out = <em>{out}</em>;
  if (has("bold")) out = <strong>{out}</strong>;
  const link = node.marks?.find((m) => m.type === "link");
  // Defence in depth: the API already validated the href.
  if (link && "attrs" in link && isSafeHref(link.attrs.href)) {
    const external = /^https?:/.test(link.attrs.href);
    out = (
      <a href={link.attrs.href} className="text-accent underline underline-offset-2" rel={external ? "noopener noreferrer" : undefined}>
        {out}
      </a>
    );
  }
  return <>{out}</>;
}

// Click-to-load: nothing is requested from YouTube until the visitor asks for it (web-v1.md §12).
function YouTubeEmbed({ videoId, start, locale }: { videoId: string; start?: number; locale: Locale }) {
  const [active, setActive] = useState(false);
  if (active) {
    const src = `https://www.youtube-nocookie.com/embed/${videoId}?autoplay=1${start ? `&start=${start}` : ""}`;
    return (
      <div className="aspect-video w-full overflow-hidden rounded-md">
        <iframe
          className="h-full w-full"
          src={src}
          title={t(locale, "doc.youtube.title")}
          allow="accelerometer; autoplay; encrypted-media; gyroscope; picture-in-picture"
          referrerPolicy="strict-origin-when-cross-origin"
          sandbox="allow-scripts allow-same-origin allow-presentation"
          allowFullScreen
        />
      </div>
    );
  }
  return (
    <div className="flex aspect-video w-full flex-col items-center justify-center gap-2 rounded-md border border-border bg-surface-muted p-4 text-center">
      <button
        type="button"
        onClick={() => setActive(true)}
        className="rounded-md bg-primary px-4 py-2 font-medium text-primary-contrast focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-accent"
      >
        {t(locale, "doc.youtube.play")}
      </button>
      <p className="text-sm text-text-muted">{t(locale, "doc.youtube.notice")}</p>
    </div>
  );
}
