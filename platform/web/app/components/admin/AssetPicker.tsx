// Modal to choose (or upload) a library asset of one kind. Private files are previewed through
// /media with the owner session; nothing here makes a file public.

import { useEffect, useRef, useState } from "react";
import { assetKindLabels, formatBytes, type Asset, type AssetKind } from "~/content/api-types";
import { Button } from "./ui";
import { Uploader } from "./Uploader";

type Props = {
  kind: AssetKind;
  csrf: string;
  title: string;
  onPick: (asset: Asset) => void;
  onClose: () => void;
};

export function AssetThumb({ asset, className = "" }: { asset: Pick<Asset, "id" | "kind" | "original_name" | "width" | "height">; className?: string }) {
  if (asset.kind === "image") {
    return <img src={`/media/${asset.id}`} alt="" width={asset.width ?? undefined} height={asset.height ?? undefined} loading="lazy" className={`bg-surface-muted object-cover ${className}`} />;
  }
  return (
    <div className={`flex items-center justify-center bg-surface-muted font-mono text-xs text-text-muted ${className}`}>
      {asset.kind === "video" ? "MP4" : (asset.original_name.split(".").pop() ?? "").toUpperCase()}
    </div>
  );
}

export function AssetPicker({ kind, csrf, title, onPick, onClose }: Props) {
  const ref = useRef<HTMLDialogElement>(null);
  const [items, setItems] = useState<Asset[] | null>(null);
  const [error, setError] = useState<string | null>(null);
  // Uploads finished in this dialog stay listed even if the library request answers later.
  const uploaded = useRef<Asset[]>([]);
  const merge = (list: Asset[]) => {
    const seen = new Set<string>();
    return [...uploaded.current, ...list].filter((a) => !seen.has(a.id) && seen.add(a.id));
  };

  useEffect(() => {
    const d = ref.current;
    if (d && !d.open) d.showModal();
    return () => d?.close();
  }, []);

  useEffect(() => {
    let alive = true;
    fetch(`/api/v1/admin/assets?kind=${kind}&page_size=60`, { credentials: "same-origin" })
      .then((r) => (r.ok ? r.json() : Promise.reject(new Error(`HTTP ${r.status}`))))
      .then((page: { items: Asset[] }) => alive && setItems(merge(page.items)))
      .catch(() => alive && setError("No se pudo cargar la biblioteca. Cierra y vuelve a intentarlo."));
    return () => {
      alive = false;
    };
  }, [kind]);

  return (
    <dialog
      ref={ref}
      aria-labelledby="picker-title"
      onCancel={(e) => {
        e.preventDefault();
        onClose();
      }}
      className="m-auto max-h-[90vh] w-[calc(100%-2rem)] max-w-3xl overflow-y-auto rounded-md border border-border bg-surface p-6 text-text backdrop:bg-black/50"
    >
      <div className="flex items-start justify-between gap-4">
        <h2 id="picker-title" className="text-lg font-semibold">
          {title}
        </h2>
        <Button tone="ghost" onClick={onClose}>
          Cerrar
        </Button>
      </div>
      <div className="mt-4">
        <Uploader csrf={csrf} only={kind} label={`Subir ${assetKindLabels[kind].toLowerCase()}`} onUploaded={(a) => {
            uploaded.current = [a, ...uploaded.current];
            setItems((list) => merge(list ?? []));
          }} />
      </div>
      <h3 className="mt-6 text-sm font-semibold">Biblioteca</h3>
      {error && <p role="alert" className="mt-2 text-sm text-danger">{error}</p>}
      {items === null && !error && <p className="mt-2 text-sm text-text-muted">Cargando…</p>}
      {items?.length === 0 && <p className="mt-2 text-sm text-text-muted">Todavía no hay archivos de este tipo. Sube uno arriba.</p>}
      {items && items.length > 0 && (
        <ul className="mt-3 grid grid-cols-2 gap-3 sm:grid-cols-4">
          {items.map((a) => (
            <li key={a.id}>
              <button
                type="button"
                onClick={() => onPick(a)}
                aria-label={`Elegir ${a.original_name} (${formatBytes(a.bytes)})`}
                className="flex w-full flex-col overflow-hidden rounded-md border border-border text-left hover:border-accent focus-visible:outline-2 focus-visible:outline-accent"
              >
                <AssetThumb asset={a} className="aspect-video w-full" />
                <span className="truncate px-2 pt-1.5 text-xs font-medium">{a.original_name}</span>
                <span className="px-2 pb-1.5 font-mono text-[0.7rem] text-text-muted">{formatBytes(a.bytes)}</span>
              </button>
            </li>
          ))}
        </ul>
      )}
    </dialog>
  );
}
