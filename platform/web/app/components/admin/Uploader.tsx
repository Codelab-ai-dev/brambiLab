// Upload queue: pick or drop files, real progress, cancel and retry. "Listo" only after the
// server answers ready (the file is stored, validated and sanitized).

import { useId, useRef, useState, type ReactNode } from "react";
import type { Asset, AssetKind } from "~/content/api-types";
import { formatBytes } from "~/content/api-types";
import { acceptAttribute, acceptFor, precheck, uploadFile, type UploadHandle } from "~/lib/upload";
import { Button } from "./ui";

type Item = {
  key: string;
  file: File;
  state: "uploading" | "processing" | "done" | "error" | "cancelled";
  asset?: Asset;
  sent: number;
  message?: string;
  retryable?: boolean;
  handle?: UploadHandle;
};

type Props = {
  csrf: string;
  only?: AssetKind;
  onUploaded: (asset: Asset) => void;
  label?: string;
  /** Optional extra content for a finished upload (e.g. a link to the new file). */
  renderDone?: (asset: Asset) => ReactNode;
};

export function Uploader({ csrf, only, onUploaded, label = "Subir archivos", renderDone }: Props) {
  const [items, setItems] = useState<Item[]>([]);
  const [dragging, setDragging] = useState(false);
  const inputId = useId();
  const counter = useRef(0);

  const patch = (key: string, p: Partial<Item>) => setItems((list) => list.map((i) => (i.key === key ? { ...i, ...p } : i)));

  // Uploads run one after another: the server accepts only one large upload at a time.
  const queue = useRef(Promise.resolve());
  const start = (file: File, key = `u${++counter.current}`) => {
    const problem = precheck(file, only);
    setItems((list) => [...list.filter((i) => i.key !== key), { key, file, state: problem ? "error" : "uploading", sent: 0, message: problem ?? undefined, retryable: false }]);
    if (problem) return;
    queue.current = queue.current.then(async () => {
      const handle = uploadFile(file, csrf, (sent, total) => patch(key, { sent, state: sent >= total ? "processing" : "uploading" }));
      patch(key, { handle });
      const result = await handle.promise;
      if (result.ok) {
        patch(key, { state: "done", handle: undefined, asset: result.asset });
        onUploaded(result.asset);
      } else {
        patch(key, { state: result.cancelled ? "cancelled" : "error", message: result.message, retryable: result.retryable, handle: undefined });
      }
    });
  };

  const addFiles = (files: FileList | null) => Array.from(files ?? []).forEach((f) => start(f));

  return (
    <div className="flex flex-col gap-3">
      <div
        onDragOver={(e) => {
          e.preventDefault();
          setDragging(true);
        }}
        onDragLeave={() => setDragging(false)}
        onDrop={(e) => {
          e.preventDefault();
          setDragging(false);
          addFiles(e.dataTransfer.files);
        }}
        className={`flex flex-col items-center gap-2 rounded-md border-2 border-dashed px-4 py-6 text-center ${dragging ? "border-accent bg-surface-muted" : "border-border-strong"}`}
      >
        <label htmlFor={inputId} className="cursor-pointer rounded-md bg-primary px-4 py-2 text-sm font-medium text-primary-contrast focus-within:outline-2 focus-within:outline-accent">
          {label}
          <input id={inputId} type="file" multiple={!only} accept={only ? acceptFor[only] : acceptAttribute} className="sr-only" onChange={(e) => {
            addFiles(e.target.files);
            e.target.value = "";
          }} />
        </label>
        <p className="text-xs text-text-muted">
          …o suéltalos aquí. {only === "image" ? "JPEG, PNG o WebP hasta 20 MiB." : only === "video" ? "MP4 hasta 250 MiB (recomendado H.264 + AAC, faststart)." : only === "resource" ? "PDF, ZIP o STL hasta 100 MiB." : "Imágenes JPEG/PNG/WebP (20 MiB), MP4 (250 MiB), PDF/ZIP/STL (100 MiB)."} Las fotos pierden sus metadatos (GPS, cámara) al subirse.
        </p>
      </div>
      {items.length > 0 && (
        <ul className="flex flex-col gap-2" aria-label="Subidas">
          {items.map((i) => {
            const pct = Math.round((i.sent / Math.max(1, i.file.size)) * 100);
            return (
              <li key={i.key} className="rounded-md border border-border p-3 text-sm" data-upload-state={i.state}>
                <div className="flex flex-wrap items-center justify-between gap-2">
                  <span className="min-w-0 break-all font-medium">{i.file.name}</span>
                  <span className="font-mono text-xs text-text-muted">{formatBytes(i.file.size)}</span>
                </div>
                {(i.state === "uploading" || i.state === "processing") && (
                  <>
                    <div role="progressbar" aria-label={`Progreso de ${i.file.name}`} aria-valuemin={0} aria-valuemax={100} aria-valuenow={pct} className="mt-2 h-2 overflow-hidden rounded bg-surface-muted">
                      <div className="h-full bg-accent transition-[width]" style={{ width: `${pct}%` }} />
                    </div>
                    <div className="mt-2 flex items-center justify-between gap-2">
                      <span role="status" className="font-mono text-xs text-text-muted">
                        {i.state === "processing" ? "Procesando en el servidor…" : `Subiendo… ${pct} %`}
                      </span>
                      {i.handle && (
                        <Button tone="ghost" className="min-h-8 px-2" onClick={() => i.handle?.cancel()}>
                          Cancelar<span className="sr-only"> la subida de {i.file.name}</span>
                        </Button>
                      )}
                    </div>
                  </>
                )}
                {i.state === "done" && (
                  <div className="mt-1 flex flex-wrap items-center justify-between gap-2">
                    <p role="status" className="text-xs text-success">Listo.</p>
                    {i.asset && renderDone?.(i.asset)}
                  </div>
                )}
                {(i.state === "error" || i.state === "cancelled") && (
                  <div className="mt-1 flex flex-wrap items-center justify-between gap-2">
                    <p role={i.state === "error" ? "alert" : "status"} className={`text-xs ${i.state === "error" ? "text-danger" : "text-text-muted"}`}>
                      {i.message}
                    </p>
                    {i.retryable && (
                      <Button className="min-h-8 px-2" onClick={() => start(i.file, i.key)}>
                        Reintentar<span className="sr-only"> {i.file.name}</span>
                      </Button>
                    )}
                  </div>
                )}
              </li>
            );
          })}
        </ul>
      )}
    </div>
  );
}
