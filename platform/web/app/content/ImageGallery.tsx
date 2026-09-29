// Compact image cards and an accessible viewer for document images (#41). Shared by the public
// pages and the private preview through DocumentView.
//
// - A card is a real link to the file: without JavaScript it opens the image itself. With
//   JavaScript the click opens the viewer instead; the viewer offers the original explicitly.
// - The viewer is a native <dialog> (modal: the page behind is inert). Focus starts on the close
//   button, stays inside, and returns to the card that opened it. Escape, the close button and a
//   click on the dark area close it; a click on the image does not.
// - Only the thumbnails load (lazily, with reserved space); the viewer requests just the image
//   being shown, and reuses the browser cache when it is the same file.
import { useEffect, useId, useRef, useState, type MouseEvent } from "react";
import { t, type Locale } from "~/i18n";

export type GalleryImage = {
  key: string;
  assetId: string;
  alt: string;
  caption?: string;
  width: number | null;
  height: number | null;
  /** False when the file is not servable (public pages): shown as unavailable, never linked. */
  available: boolean;
  /** Figure number in document order. */
  figure: number;
};

const fig = (n: number) => `FIG. ${String(n).padStart(2, "0")}`;

export function ImageGallery({ images, locale }: { images: GalleryImage[]; locale: Locale }) {
  const [open, setOpen] = useState<number | null>(null);
  const triggers = useRef<(HTMLAnchorElement | null)[]>([]);
  // The card that invoked the viewer gets the focus back, even after moving to other images.
  const opener = useRef<number>(0);
  const viewable = images.filter((i) => i.available);

  const openAt = (e: MouseEvent<HTMLAnchorElement>, index: number) => {
    // Let modified clicks (new tab/window) do their native job.
    if (e.button !== 0 || e.metaKey || e.ctrlKey || e.shiftKey || e.altKey) return;
    e.preventDefault();
    opener.current = index;
    setOpen(index);
  };

  return (
    <div
      className={images.length > 1 ? "grid grid-cols-[repeat(auto-fill,minmax(min(100%,15rem),1fr))] gap-4" : "flex"}
      data-gallery={images.length}
    >
      {images.map((img) =>
        img.available ? (
          <figure key={img.key} className="group m-0 flex w-full max-w-[20rem] flex-col overflow-hidden rounded-md border border-border bg-surface" data-image-card>
            <a
              href={`/media/${img.assetId}`}
              ref={(el) => {
                triggers.current[viewable.indexOf(img)] = el;
              }}
              onClick={(e) => openAt(e, viewable.indexOf(img))}
              className="relative block h-[220px] bg-surface-muted bl-plate focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-accent"
            >
              <img
                src={`/media/${img.assetId}`}
                alt={img.alt}
                width={img.width ?? undefined}
                height={img.height ?? undefined}
                loading="lazy"
                decoding="async"
                className="h-full w-full object-contain p-2"
              />
              <span className="sr-only">{`. ${t(locale, "doc.image.enlarge")}`}</span>
              <span
                aria-hidden="true"
                className="absolute right-2 bottom-2 inline-flex items-center gap-1 rounded bg-primary/85 px-1.5 py-0.5 font-mono text-[0.68rem] tracking-wider text-primary-contrast uppercase opacity-80 transition-opacity group-hover:opacity-100 group-focus-within:opacity-100"
              >
                <svg viewBox="0 0 16 16" className="size-3" fill="none" stroke="currentColor" strokeWidth="1.6">
                  <path d="M9.5 2.5h4v4M6.5 13.5h-4v-4M13.5 2.5 9 7M2.5 13.5 7 9" />
                </svg>
                {t(locale, "doc.image.enlargeShort")}
              </span>
            </a>
            <figcaption className="flex items-baseline gap-2 border-t border-border px-3 py-2 text-sm text-text-muted">
              <span className="shrink-0 font-mono text-[0.7rem] tracking-wider text-accent">{fig(img.figure)}</span>
              {img.caption && <span className="min-w-0 [overflow-wrap:anywhere]">{img.caption}</span>}
            </figcaption>
          </figure>
        ) : (
          <p key={img.key} className="flex h-[220px] w-full max-w-[20rem] items-center justify-center rounded-md border border-dashed border-border px-4 text-center text-sm text-text-muted" data-media="unavailable">
            {t(locale, "media.unavailable")}
          </p>
        ),
      )}
      {open !== null && viewable[open] && (
        <Viewer
          images={viewable}
          index={open}
          locale={locale}
          onIndex={setOpen}
          onClose={() => {
            const trigger = triggers.current[opener.current];
            setOpen(null);
            requestAnimationFrame(() => trigger?.focus());
          }}
        />
      )}
    </div>
  );
}

function Viewer({ images, index, locale, onIndex, onClose }: { images: GalleryImage[]; index: number; locale: Locale; onIndex: (i: number) => void; onClose: () => void }) {
  const ref = useRef<HTMLDialogElement>(null);
  const captionId = useId();
  const img = images[index];
  const many = images.length > 1;
  const go = (d: number) => onIndex((index + d + images.length) % images.length);

  useEffect(() => {
    const dialog = ref.current;
    if (!dialog) return;
    const { overflow, paddingRight } = document.body.style;
    // Lock the page scroll without a layout jump where a scrollbar disappears.
    const gap = window.innerWidth - document.documentElement.clientWidth;
    document.body.style.overflow = "hidden";
    if (gap > 0) document.body.style.paddingRight = `${gap}px`;
    if (!dialog.open) dialog.showModal();
    // Initial focus on the close button (the first focusable element would be "open original").
    dialog.querySelector<HTMLButtonElement>("[data-viewer-close]")?.focus();
    return () => {
      document.body.style.overflow = overflow;
      document.body.style.paddingRight = paddingRight;
      if (dialog.open) dialog.close();
    };
  }, []);

  return (
    <dialog
      ref={ref}
      aria-label={t(locale, "doc.viewer.label")}
      aria-describedby={img.caption ? captionId : undefined}
      className="bl-viewer fixed inset-0 m-0 h-dvh max-h-none w-screen max-w-none p-0"
      onCancel={(e) => {
        e.preventDefault(); // Escape: close through React so focus returns to the card
        onClose();
      }}
      onKeyDown={(e) => {
        if (e.key === "Tab") {
          // Keep focus inside: the page behind is inert, but Tab could still leave the document.
          const items = [...e.currentTarget.querySelectorAll<HTMLElement>("a[href], button")];
          const first = items[0];
          const last = items[items.length - 1];
          if (e.shiftKey && document.activeElement === first) {
            e.preventDefault();
            last.focus();
          } else if (!e.shiftKey && document.activeElement === last) {
            e.preventDefault();
            first.focus();
          }
          return;
        }
        if (!many) return;
        if (e.key === "ArrowRight") go(1);
        if (e.key === "ArrowLeft") go(-1);
      }}
      onClick={(e) => {
        // Only the dark area closes; the image, caption and controls do not.
        if (e.target === e.currentTarget || (e.target as HTMLElement).dataset.backdrop !== undefined) onClose();
      }}
      data-viewer
    >
      <div data-backdrop className="flex h-full flex-col">
        <header className="flex items-center justify-between gap-3 px-4 py-3 sm:px-6">
          <p className="font-mono text-xs tracking-widest whitespace-nowrap text-[var(--bench-label)] uppercase" aria-live="polite">
            {fig(img.figure)}
            {many && (
              <>
                <span aria-hidden="true">{` · ${index + 1}/${images.length}`}</span>
                <span className="sr-only">{`, ${t(locale, "doc.viewer.position").replace("{n}", String(index + 1)).replace("{total}", String(images.length))}`}</span>
              </>
            )}
          </p>
          <div className="flex shrink-0 items-center gap-2">
            <a
              href={`/media/${img.assetId}`}
              target="_blank"
              rel="noopener"
              className="rounded border border-[var(--bench-edge)] px-2.5 py-1.5 text-sm whitespace-nowrap hover:border-[var(--bench-signal)] focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-[var(--bench-signal)]"
            >
              {t(locale, "doc.viewer.original")} <span aria-hidden="true">↗</span>
            </a>
            <button
              type="button"
              data-viewer-close
              onClick={onClose}
              className="rounded border border-[var(--bench-signal)] px-2.5 py-1.5 text-sm font-medium whitespace-nowrap hover:bg-[var(--bench-signal)] hover:text-[var(--bench-bg)] focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-[var(--bench-signal)]"
            >
              {t(locale, "doc.viewer.close")} <span aria-hidden="true">✕</span>
            </button>
          </div>
        </header>
        <div data-backdrop className="relative flex min-h-0 flex-1 items-center justify-center px-2 sm:px-16">
          {many && (
            <button type="button" onClick={() => go(-1)} aria-label={t(locale, "doc.viewer.prev")} className="bl-viewer-nav left-2 sm:left-4">
              <span aria-hidden="true">←</span>
            </button>
          )}
          <figure className="m-0 h-full w-full">
            {/* Fitted to the viewport (scaled up or down, never distorted); the natural size is
                "open original". key: a new element per image, so a slow load never shows the last one. */}
            <img key={img.assetId} src={`/media/${img.assetId}`} alt={img.alt} className="h-full w-full object-contain" data-viewer-image />
          </figure>
          {many && (
            <button type="button" onClick={() => go(1)} aria-label={t(locale, "doc.viewer.next")} className="bl-viewer-nav right-2 sm:right-4">
              <span aria-hidden="true">→</span>
            </button>
          )}
        </div>
        <footer data-backdrop className="min-h-12 px-4 py-3 text-center text-sm text-[var(--bench-muted)] sm:px-6">
          {img.caption && (
            <p id={captionId} className="mx-auto max-w-3xl [overflow-wrap:anywhere]">
              {img.caption}
            </p>
          )}
        </footer>
      </div>
    </dialog>
  );
}
