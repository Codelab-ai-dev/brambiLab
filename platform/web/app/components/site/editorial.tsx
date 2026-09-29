// Editorial building blocks of the public site (WEB-009): wide frame, section headers, technical
// metadata and image panels. Surfaces come from the scope classes in app.css (.bl-site, .bl-navy,
// .bl-light, .bl-signal); these components only use the semantic tokens.
import type { ReactNode } from "react";
import type { Cover } from "~/site/types";

/** Wide editorial frame: 1 column on phones, 6 on tablets, 12 on desktop (grid set by callers). */
export function Frame({ children, className = "" }: { children: ReactNode; className?: string }) {
  return <div className={`mx-auto w-full max-w-[100rem] px-4 sm:px-6 lg:px-10 ${className}`}>{children}</div>;
}

/** Arrow glyph as SVG (the ↗ character is not in the font's Latin subset). */
export function Arrow({ direction = "up-right", className = "" }: { direction?: "up-right" | "right"; className?: string }) {
  return (
    <svg viewBox="0 0 16 16" aria-hidden="true" focusable="false" data-dir={direction} className={`bl-arrow size-4 shrink-0 ${className}`} fill="none" stroke="currentColor" strokeWidth="1.8" strokeLinecap="square">
      {direction === "up-right" ? <path d="M4.5 11.5 11.5 4.5M5.5 4.5h6v6" /> : <path d="M2.5 8h11M9 3.5 13.5 8 9 12.5" />}
    </svg>
  );
}

/** "01 / Label ——" eyebrow. The number is editorial order on this page, never an identifier. */
export function Eyebrow({ index, children }: { index?: string; children: ReactNode }) {
  return (
    <p className="bl-meta flex items-center gap-3 text-text-muted">
      {index && <span className="text-accent">{index}</span>}
      {index && <span aria-hidden="true">/</span>}
      <span>{children}</span>
      <span aria-hidden="true" className="h-px w-10 bg-current opacity-60" />
    </p>
  );
}

export function SectionHeader({ id, index, label, title, children }: { id: string; index?: string; label: string; title: ReactNode; children?: ReactNode }) {
  return (
    <div>
      <Eyebrow index={index}>{label}</Eyebrow>
      <h2 id={id} className="bl-h-section mt-6 max-w-[14ch]">
        {title}
      </h2>
      {children}
    </div>
  );
}

export type MetaRow = { label: string; value: ReactNode };

/** "LABEL / value" rows. Only rows with real data are passed in; nothing is padded. */
export function TechnicalMeta({ rows, className = "" }: { rows: MetaRow[]; className?: string }) {
  if (rows.length === 0) return null;
  return (
    <dl className={`grid grid-cols-[minmax(0,7.5rem)_minmax(0,1fr)] gap-x-3 gap-y-2 font-mono text-xs ${className}`}>
      {rows.map((r) => (
        <div key={r.label} className="contents">
          <dt className="tracking-[0.12em] text-text-muted uppercase">{r.label}</dt>
          <dd className="min-w-0 tracking-wide [overflow-wrap:anywhere] uppercase">
            <span aria-hidden="true" className="mr-2 text-text-muted">/</span>
            {r.value}
          </dd>
        </div>
      ))}
    </dl>
  );
}

/**
 * A photograph with reserved space (aspect ratio from the file when known), desaturated on
 * editorial blocks. Without a photo the caller passes an illustration instead.
 */
export function Photo({ cover, className = "", priority = false, ratio = "4 / 3" }: { cover: Cover; className?: string; priority?: boolean; ratio?: string }) {
  return (
    <div className={`overflow-hidden bg-surface-muted ${className}`} style={{ aspectRatio: ratio }}>
      <img
        src={`/media/${cover.asset_id}`}
        alt={cover.alt}
        width={cover.width ?? undefined}
        height={cover.height ?? undefined}
        loading={priority ? "eager" : "lazy"}
        fetchPriority={priority ? "high" : undefined}
        decoding="async"
        className="bl-photo h-full w-full object-cover"
      />
    </div>
  );
}
