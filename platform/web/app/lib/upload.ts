// Browser upload of one file to Go (raw body, web-v1.md §12.1) with real progress and cancel.
// XMLHttpRequest is used because fetch still cannot report upload progress.

import type { Asset, AssetKind } from "~/content/api-types";

// Mirrors the server allowlist so obvious mistakes fail before sending; the server decides.
const kinds: Record<string, AssetKind> = {
  jpg: "image", jpeg: "image", png: "image", webp: "image",
  mp4: "video", m4v: "video",
  pdf: "resource", zip: "resource", stl: "resource",
};
export const limits: Record<AssetKind, number> = { image: 20 << 20, video: 250 << 20, resource: 100 << 20 };
export const acceptAttribute = ".jpg,.jpeg,.png,.webp,.mp4,.m4v,.pdf,.zip,.stl";
export const acceptFor: Record<AssetKind, string> = { image: ".jpg,.jpeg,.png,.webp", video: ".mp4,.m4v", resource: ".pdf,.zip,.stl" };

export function kindOf(name: string): AssetKind | null {
  return kinds[name.split(".").pop()?.toLowerCase() ?? ""] ?? null;
}

/** Early, friendly check. Returns a message, or null when the server should decide. */
export function precheck(file: File, only?: AssetKind): string | null {
  const kind = kindOf(file.name);
  if (!kind) {
    const ext = file.name.split(".").pop()?.toLowerCase();
    if (ext === "heic" || ext === "heif") return "HEIC/HEIF no está admitido: exporta la foto como JPEG.";
    if (ext === "svg") return "SVG no está admitido porque puede ejecutar código; exporta como PNG.";
    return "Formato no admitido. Imágenes: JPEG, PNG, WebP. Vídeo: MP4. Recursos: PDF, ZIP, STL.";
  }
  if (only && kind !== only) return `Aquí sólo se admiten archivos de tipo ${only === "image" ? "imagen" : only === "video" ? "vídeo" : "recurso"}.`;
  if (file.size === 0) return "El archivo está vacío.";
  if (file.size > limits[kind]) return `El archivo supera el límite de ${limits[kind] >> 20} MiB.`;
  return null;
}

export type UploadResult =
  | { ok: true; asset: Asset }
  | { ok: false; cancelled: boolean; message: string; retryable: boolean };

export type UploadHandle = { promise: Promise<UploadResult>; cancel: () => void };

export function uploadFile(file: File, csrf: string, onProgress: (sent: number, total: number) => void): UploadHandle {
  const xhr = new XMLHttpRequest();
  const promise = new Promise<UploadResult>((resolve) => {
    xhr.open("POST", `/api/v1/admin/assets?filename=${encodeURIComponent(file.name)}`);
    xhr.withCredentials = true;
    xhr.setRequestHeader("X-CSRF-Token", csrf);
    xhr.setRequestHeader("Content-Type", "application/octet-stream");
    xhr.upload.onprogress = (e) => onProgress(e.loaded, e.lengthComputable ? e.total : file.size);
    xhr.onload = () => {
      let body: { code?: string; message?: string } & Partial<Asset> = {};
      try {
        body = JSON.parse(xhr.responseText);
      } catch {
        /* non-JSON error page */
      }
      if (xhr.status === 201 && body.status === "ready") resolve({ ok: true, asset: body as Asset });
      else
        resolve({
          ok: false,
          cancelled: false,
          message: body.message ?? `El servidor respondió ${xhr.status}.`,
          // Busy, interrupted, disk and server errors can be retried; type/size problems cannot.
          retryable: xhr.status === 429 || xhr.status === 400 || xhr.status === 507 || xhr.status >= 500,
        });
    };
    xhr.onerror = () => resolve({ ok: false, cancelled: false, message: "Se perdió la conexión durante la subida.", retryable: true });
    xhr.onabort = () => resolve({ ok: false, cancelled: true, message: "Subida cancelada.", retryable: true });
    xhr.send(file);
  });
  return { promise, cancel: () => xhr.abort() };
}
