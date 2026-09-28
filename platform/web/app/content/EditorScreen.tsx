// Editing screen for one translation (WEB-003): metadata + visual body, autosave, conflicts,
// Markdown import/export and leave guards. Saving goes browser → Go with the CSRF token.

import { useEffect, useRef, useState, type ReactNode } from "react";
import { Link, useBlocker } from "react-router";
import { Button, Field, LinkButton, Notice, inputClass } from "~/components/admin/ui";
import { saveRevision, newIdempotencyKey } from "~/lib/admin-client";
import {
  formatDate,
  kindLabels,
  localeLabels,
  slugify,
  snapshotFrom,
  statusLabels,
  type Content,
  type Locale,
  type ProjectFields,
  type ProjectStatus,
  type Snapshot,
  type Term,
  type Translation,
} from "./api-types";
import { Autosaver, type AutosaveState } from "./autosave";
import { ContentEditor } from "./ContentEditor";
import { DocumentView } from "./DocumentView";
import { docToMarkdown, markdownToDoc, type ImportResult } from "./markdown";
import type { Doc } from "./schema";

type Props = {
  content: Content;
  translation: Translation;
  categories: Term[];
  tags: Term[];
  csrf: string;
};

const MAX_IMPORT_BYTES = 1 << 20;

export function EditorScreen({ content, translation, categories, tags, csrf }: Props) {
  const locale = translation.locale;
  const readOnly = content.archived_at !== null;
  const initial = snapshotFrom(translation.latest);

  const [snapshot, setSnapshot] = useState<Snapshot>(initial);
  const snapshotRef = useRef(snapshot);
  const [editorDoc, setEditorDoc] = useState<Doc>(initial.body);
  const [editorKey, setEditorKey] = useState(0);
  const [save, setSave] = useState<AutosaveState>({ status: "saved", version: translation.latest_version });
  const [editorProblem, setEditorProblem] = useState<string | null>(null);
  const [notes, setNotes] = useState<string[]>([]);
  const [slugTouched, setSlugTouched] = useState(initial.slug !== "");
  const [importOpen, setImportOpen] = useState(false);
  const saver = useRef<Autosaver<Snapshot> | null>(null);

  useEffect(() => {
    if (readOnly) return;
    const s = new Autosaver<Snapshot>(
      {
        save: (snap, expected, kind, key) => saveRevision(content.id, locale, csrf, snap, expected, kind, key),
        onState: setSave,
        newKey: newIdempotencyKey,
      },
      { snapshot: snapshotRef.current, version: translation.latest_version },
    );
    saver.current = s;
    return () => s.dispose();
    // One saver per mounted translation; the route remounts this screen on navigation.
  }, [content.id, locale, csrf, readOnly, translation.latest_version]);

  const change = (patch: Partial<Snapshot>) => {
    const next = { ...snapshotRef.current, ...patch };
    snapshotRef.current = next;
    setSnapshot(next);
    saver.current?.update(next);
  };

  const unsaved = () => saver.current?.hasUnsavedChanges ?? false;

  // Warn before leaving the page (reload, close tab) or navigating inside the panel.
  useEffect(() => {
    const onBeforeUnload = (e: BeforeUnloadEvent) => {
      if (unsaved()) e.preventDefault();
    };
    window.addEventListener("beforeunload", onBeforeUnload);
    return () => window.removeEventListener("beforeunload", onBeforeUnload);
  }, []);
  const blocker = useBlocker(({ currentLocation, nextLocation }) => unsaved() && currentLocation.pathname !== nextLocation.pathname);
  useEffect(() => {
    if (blocker.state !== "blocked") return;
    if (window.confirm("Hay cambios sin guardar. ¿Salir de todas formas? Se perderán.")) blocker.proceed();
    else blocker.reset();
  }, [blocker]);

  const fields = save.fields ?? {};
  const pf: ProjectFields = snapshot.project_fields ?? {};
  const setProject = (patch: Partial<ProjectFields>) => change({ project_fields: { ...pf, ...patch } });
  const other: Locale = locale === "es" ? "en" : "es";
  const hasOther = content.translations.some((t) => t.locale === other);
  const isCopy = translation.latest?.kind === "copy" && save.version === translation.latest_version;

  const download = (name: string, text: string, type: string) => {
    const url = URL.createObjectURL(new Blob([text], { type }));
    const a = Object.assign(document.createElement("a"), { href: url, download: name });
    a.click();
    URL.revokeObjectURL(url);
  };
  const baseName = `${snapshot.slug || "contenido"}.${locale}`;
  const exportMarkdown = () => download(`${baseName}.md`, docToMarkdown(snapshot.body), "text/markdown;charset=utf-8");
  const exportJSON = () => download(`${baseName}.json`, JSON.stringify(snapshot, null, 2), "application/json");

  const reloadFromServer = async () => {
    const response = await fetch(`/api/v1/admin/contents/${content.id}/translations/${locale}`, { credentials: "same-origin" });
    if (!response.ok) throw new Error(`HTTP ${response.status}`);
    const fresh = (await response.json()) as Translation;
    const next = snapshotFrom(fresh.latest);
    snapshotRef.current = next;
    setSnapshot(next);
    setEditorDoc(next.body);
    setEditorKey((k) => k + 1);
    saver.current?.reset(next, fresh.latest_version);
  };

  return (
    <div className="flex flex-col gap-6 py-6">
      <header className="flex flex-wrap items-start justify-between gap-4">
        <div className="min-w-0">
          <p className="text-sm text-text-muted">
            <Link to={`/admin/contenidos/${content.id}`} className="underline underline-offset-2">
              {kindLabels[content.kind].one}
            </Link>{" "}
            · {localeLabels[locale]}
          </p>
          <h1 className="mt-1 text-2xl font-semibold break-words">{snapshot.title || "Sin título"}</h1>
        </div>
        <div className="flex flex-wrap items-center gap-2">
          <SaveStatus state={save} readOnly={readOnly} />
          {!readOnly && (
            <Button tone="primary" onClick={() => void saver.current?.saveNow()} disabled={save.status === "conflict"}>
              Guardar revisión
            </Button>
          )}
        </div>
      </header>

      {readOnly && <Notice tone="warning">Contenido archivado: sólo lectura. Desarchívalo desde su ficha para editar.</Notice>}
      {isCopy && (
        <Notice tone="warning" title="Borrador copiado">
          Este texto es una copia de la versión {translation.latest?.copied_from_locale?.toUpperCase()} pendiente de traducir; no es una traducción terminada.
        </Notice>
      )}
      {save.status === "error" && save.message && <Notice tone="danger">{save.message}</Notice>}
      {editorProblem && <Notice tone="danger">{editorProblem}</Notice>}
      {notes.length > 0 && (
        <Notice title="Ajustes al guardar">
          <ul className="list-disc pl-5">
            {notes.map((n) => (
              <li key={n}>{n}</li>
            ))}
          </ul>
        </Notice>
      )}
      {save.status === "conflict" && (
        <ConflictDialog serverVersion={save.conflictVersion ?? 0} onExportMarkdown={exportMarkdown} onExportJSON={exportJSON} onReload={reloadFromServer} />
      )}

      <nav aria-label="Acciones del contenido" className="flex flex-wrap gap-2 text-sm">
        {save.version > 0 ? (
          <LinkButton to={`/admin/contenidos/${content.id}/${locale}/v/${save.version}`} title="Muestra la última versión guardada">
            Vista previa (v{save.version})
          </LinkButton>
        ) : (
          <Button disabled title="Guarda una revisión primero">
            Vista previa
          </Button>
        )}
        {save.version > 0 && <LinkButton to={`/admin/contenidos/${content.id}/${locale}/historial`}>Historial</LinkButton>}
        {!readOnly && <Button onClick={() => setImportOpen(true)}>Importar Markdown</Button>}
        <Button onClick={exportMarkdown}>Exportar Markdown</Button>
        {hasOther && <LinkButton to={`/admin/contenidos/${content.id}/${other}`}>Editar en {localeLabels[other]}</LinkButton>}
        <Button disabled aria-describedby="publish-note">
          Publicar
        </Button>
        <span id="publish-note" className="self-center text-xs text-text-muted">
          Publicar llegará con WEB-005.
        </span>
      </nav>

      <div className="grid gap-4 sm:grid-cols-2">
        <Field label="Título" error={fields.title}>
          {({ id, describedBy, invalid }) => (
            <input
              id={id}
              className={inputClass}
              value={snapshot.title}
              disabled={readOnly}
              aria-describedby={describedBy}
              aria-invalid={invalid}
              required
              maxLength={200}
              onChange={(e) => change({ title: e.target.value, ...(slugTouched ? {} : { slug: slugify(e.target.value) }) })}
            />
          )}
        </Field>
        <Field label="Slug (URL)" help="Minúsculas, números y guiones. Se propone a partir del título." error={fields.slug}>
          {({ id, describedBy, invalid }) => (
            <input
              id={id}
              className={`${inputClass} font-mono`}
              value={snapshot.slug}
              disabled={readOnly}
              aria-describedby={describedBy}
              aria-invalid={invalid}
              maxLength={120}
              onChange={(e) => {
                setSlugTouched(true);
                change({ slug: e.target.value });
              }}
            />
          )}
        </Field>
      </div>

      <div className="grid gap-8 lg:grid-cols-[minmax(0,1fr)_20rem]">
        <section aria-label="Cuerpo" className="flex min-w-0 flex-col gap-2">
          <ContentEditor
            key={editorKey}
            initialDoc={editorDoc}
            readOnly={readOnly}
            onChange={(doc, n) => {
              setEditorProblem(null);
              setNotes(n);
              change({ body: doc });
            }}
            onInvalid={setEditorProblem}
          />
          {fields.body && <p className="text-sm text-danger">{fields.body}</p>}
        </section>

        <aside aria-label="Datos del contenido" className="flex flex-col gap-5">
          <Field label="Resumen" help="Hasta 500 caracteres. Aparece en listados." error={fields.summary}>
            {({ id, describedBy, invalid }) => (
              <textarea
                id={id}
                className={inputClass}
                rows={3}
                value={snapshot.summary}
                disabled={readOnly}
                aria-describedby={describedBy}
                aria-invalid={invalid}
                maxLength={500}
                onChange={(e) => change({ summary: e.target.value })}
              />
            )}
          </Field>

          <Field label="Categoría" error={fields.category_id}>
            {({ id, describedBy, invalid }) => (
              <select
                id={id}
                className={inputClass}
                value={snapshot.category_id ?? ""}
                disabled={readOnly}
                aria-describedby={describedBy}
                aria-invalid={invalid}
                onChange={(e) => change({ category_id: e.target.value || null })}
              >
                <option value="">Sin categoría</option>
                {categories.map((c) => (
                  <option key={c.id} value={c.id}>
                    {c.labels[locale]}
                  </option>
                ))}
              </select>
            )}
          </Field>
          <fieldset className="flex flex-col gap-2">
            <legend className="text-sm font-medium">Etiquetas</legend>
            {tags.length === 0 ? (
              <p className="text-xs text-text-muted">
                No hay etiquetas. <Link to="/admin/taxonomia" className="underline">Créalas aquí</Link>.
              </p>
            ) : (
              <div className="flex flex-wrap gap-2">
                {tags.map((t) => {
                  const checked = snapshot.tag_ids.includes(t.id);
                  return (
                    <label key={t.id} className="flex items-center gap-1.5 rounded border border-border px-2 py-1 text-sm">
                      <input
                        type="checkbox"
                        checked={checked}
                        disabled={readOnly}
                        onChange={() => change({ tag_ids: checked ? snapshot.tag_ids.filter((x) => x !== t.id) : [...snapshot.tag_ids, t.id] })}
                      />
                      {t.labels[locale]}
                    </label>
                  );
                })}
              </div>
            )}
            {fields.tag_ids && <p className="text-xs text-danger">{fields.tag_ids}</p>}
          </fieldset>

          <details className="rounded-md border border-border p-3" open={Boolean(fields["seo.title"] || fields["seo.description"])}>
            <summary className="cursor-pointer text-sm font-medium">SEO</summary>
            <div className="mt-3 flex flex-col gap-4">
              <Field label="Título SEO" help="Hasta 70 caracteres. Si está vacío se usa el título." error={fields["seo.title"]}>
                {({ id, describedBy, invalid }) => (
                  <input id={id} className={inputClass} maxLength={70} value={snapshot.seo.title ?? ""} disabled={readOnly} aria-describedby={describedBy} aria-invalid={invalid} onChange={(e) => change({ seo: { ...snapshot.seo, title: e.target.value } })} />
                )}
              </Field>
              <Field label="Descripción SEO" help="Hasta 160 caracteres." error={fields["seo.description"]}>
                {({ id, describedBy, invalid }) => (
                  <textarea id={id} className={inputClass} rows={2} maxLength={160} value={snapshot.seo.description ?? ""} disabled={readOnly} aria-describedby={describedBy} aria-invalid={invalid} onChange={(e) => change({ seo: { ...snapshot.seo, description: e.target.value } })} />
                )}
              </Field>
            </div>
          </details>

          {content.kind === "project" && (
            <fieldset className="flex flex-col gap-4 rounded-md border border-border p-3" disabled={readOnly}>
              <legend className="px-1 text-sm font-medium">Ficha del proyecto</legend>
              <Field label="Estado técnico" help="Estado del proyecto, no de la publicación." error={fields["project_fields.status"]}>
                {({ id, describedBy, invalid }) => (
                  <select id={id} className={inputClass} value={pf.status ?? ""} aria-describedby={describedBy} aria-invalid={invalid} onChange={(e) => setProject({ status: e.target.value as ProjectStatus })}>
                    <option value="">Sin indicar</option>
                    {Object.entries(statusLabels).map(([v, l]) => (
                      <option key={v} value={v}>
                        {l}
                      </option>
                    ))}
                  </select>
                )}
              </Field>
              <Field label="Objetivo" error={fields["project_fields.objective"]}>
                {({ id, describedBy, invalid }) => (
                  <textarea id={id} className={inputClass} rows={2} maxLength={1000} value={pf.objective ?? ""} aria-describedby={describedBy} aria-invalid={invalid} onChange={(e) => setProject({ objective: e.target.value })} />
                )}
              </Field>
              <TechnologiesField value={pf.technologies ?? []} error={fields["project_fields.technologies"]} onChange={(technologies) => setProject({ technologies })} />
              <LinksField value={pf.links ?? []} errors={fields} onChange={(links) => setProject({ links })} />
              <Field label="Resultados" error={fields["project_fields.results"]}>
                {({ id, describedBy, invalid }) => (
                  <textarea id={id} className={inputClass} rows={3} maxLength={2000} value={pf.results ?? ""} aria-describedby={describedBy} aria-invalid={invalid} onChange={(e) => setProject({ results: e.target.value })} />
                )}
              </Field>
            </fieldset>
          )}
        </aside>
      </div>

      {importOpen && (
        <ImportDialog
          locale={locale}
          onCancel={() => setImportOpen(false)}
          onConfirm={(doc) => {
            setImportOpen(false);
            setEditorDoc(doc);
            setEditorKey((k) => k + 1);
            change({ body: doc });
          }}
        />
      )}
    </div>
  );
}

function SaveStatus({ state, readOnly }: { state: AutosaveState; readOnly: boolean }) {
  let text: ReactNode;
  let color = "text-text-muted";
  if (readOnly) text = "Sólo lectura";
  else
    switch (state.status) {
      case "saved":
        text = state.version ? <>Guardado · v{state.version}{state.lastSavedAt ? ` · ${formatDate(new Date(state.lastSavedAt).toISOString())}` : ""}</> : "Sin revisiones todavía";
        break;
      case "dirty":
        text = "Cambios sin guardar";
        color = "text-warning";
        break;
      case "saving":
        text = state.retrying ? "Reintentando el guardado…" : "Guardando…";
        break;
      case "error":
        text = "Error al guardar";
        color = "text-danger";
        break;
      case "conflict":
        text = "Conflicto: hay una versión más nueva";
        color = "text-danger";
        break;
    }
  return (
    <p role="status" aria-live="polite" className={`text-sm ${color}`} data-save-status={readOnly ? "readonly" : state.status}>
      {text}
    </p>
  );
}

function TechnologiesField({ value, error, onChange }: { value: string[]; error?: string; onChange: (v: string[]) => void }) {
  const [text, setText] = useState(value.join(", "));
  return (
    <Field label="Tecnologías" help="Separadas por comas, por ejemplo: ESP32, Go, React." error={error}>
      {({ id, describedBy, invalid }) => (
        <input
          id={id}
          className={inputClass}
          value={text}
          aria-describedby={describedBy}
          aria-invalid={invalid}
          onChange={(e) => {
            setText(e.target.value);
            onChange(e.target.value.split(",").map((t) => t.trim()).filter(Boolean));
          }}
        />
      )}
    </Field>
  );
}

function LinksField({ value, errors, onChange }: { value: { label: string; url: string }[]; errors: Record<string, string>; onChange: (v: { label: string; url: string }[]) => void }) {
  const set = (i: number, patch: Partial<{ label: string; url: string }>) => onChange(value.map((l, j) => (j === i ? { ...l, ...patch } : l)));
  return (
    <fieldset className="flex flex-col gap-3">
      <legend className="text-sm font-medium">Enlaces</legend>
      {value.map((link, i) => (
        <div key={i} className="flex flex-col gap-2 rounded border border-border p-2">
          <Field label={`Etiqueta del enlace ${i + 1}`} error={errors[`project_fields.links[${i}].label`]}>
            {({ id, describedBy, invalid }) => <input id={id} className={inputClass} value={link.label} aria-describedby={describedBy} aria-invalid={invalid} onChange={(e) => set(i, { label: e.target.value })} />}
          </Field>
          <Field label={`URL del enlace ${i + 1}`} error={errors[`project_fields.links[${i}].url`]}>
            {({ id, describedBy, invalid }) => <input id={id} type="url" className={inputClass} value={link.url} placeholder="https://" aria-describedby={describedBy} aria-invalid={invalid} onChange={(e) => set(i, { url: e.target.value })} />}
          </Field>
          <Button tone="ghost" className="self-start text-danger" onClick={() => onChange(value.filter((_, j) => j !== i))}>
            Quitar enlace {i + 1}
          </Button>
        </div>
      ))}
      <Button className="self-start" onClick={() => onChange([...value, { label: "", url: "" }])} disabled={value.length >= 20}>
        Añadir enlace
      </Button>
    </fieldset>
  );
}

function useModal() {
  const ref = useRef<HTMLDialogElement>(null);
  useEffect(() => {
    const d = ref.current;
    if (d && !d.open) d.showModal();
    return () => d?.close();
  }, []);
  return ref;
}

function ConflictDialog({ serverVersion, onExportMarkdown, onExportJSON, onReload }: { serverVersion: number; onExportMarkdown: () => void; onExportJSON: () => void; onReload: () => Promise<void> }) {
  const ref = useModal();
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);
  return (
    <dialog ref={ref} aria-labelledby="conflict-title" className="m-auto max-h-[90vh] w-[calc(100%-2rem)] max-w-lg overflow-y-auto rounded-md border border-border bg-surface p-6 text-text backdrop:bg-black/50">
      <h2 id="conflict-title" className="text-lg font-semibold">
        Otra pestaña guardó una versión más nueva
      </h2>
      <p className="mt-3 text-sm">
        Mientras editabas se guardó la versión {serverVersion} desde otra pestaña o dispositivo. <strong>Tus cambios siguen aquí y no se han guardado.</strong> El autoguardado queda en pausa para no sobrescribir nada.
      </p>
      <ol className="mt-3 list-decimal pl-5 text-sm">
        <li>Descarga tus cambios para no perderlos.</li>
        <li>Carga la versión {serverVersion} y vuelve a aplicar lo que necesites.</li>
      </ol>
      {error && <p role="alert" className="mt-3 text-sm text-danger">{error}</p>}
      <div className="mt-5 flex flex-wrap gap-2">
        <Button onClick={onExportMarkdown}>Descargar mis cambios (.md)</Button>
        <Button onClick={onExportJSON}>Descargar todo (.json)</Button>
        <Button
          tone="danger"
          disabled={busy}
          onClick={async () => {
            if (!window.confirm(`¿Descartar tus cambios y cargar la versión ${serverVersion}? No se puede deshacer.`)) return;
            setBusy(true);
            try {
              await onReload();
            } catch {
              setError("No se pudo cargar la versión del servidor. Inténtalo de nuevo.");
              setBusy(false);
            }
          }}
        >
          Descartar mis cambios y cargar v{serverVersion}
        </Button>
      </div>
    </dialog>
  );
}

function ImportDialog({ locale, onCancel, onConfirm }: { locale: Locale; onCancel: () => void; onConfirm: (doc: Doc) => void }) {
  const ref = useModal();
  const [text, setText] = useState("");
  const [result, setResult] = useState<ImportResult | null>(null);
  const [error, setError] = useState<string | null>(null);
  const analyze = (markdown: string) => {
    if (new Blob([markdown]).size > MAX_IMPORT_BYTES) {
      setError("El archivo supera 1 MiB.");
      setResult(null);
      return;
    }
    setError(null);
    setResult(markdownToDoc(markdown));
  };
  return (
    <dialog
      ref={ref}
      aria-labelledby="import-title"
      onCancel={(e) => {
        e.preventDefault();
        onCancel();
      }}
      className="m-auto max-h-[90vh] w-[calc(100%-2rem)] max-w-3xl overflow-y-auto rounded-md border border-border bg-surface p-6 text-text backdrop:bg-black/50"
    >
      <h2 id="import-title" className="text-lg font-semibold">
        Importar Markdown
      </h2>
      <p className="mt-2 text-sm text-text-muted">
        Revisa el resultado antes de aplicarlo. Importar reemplaza el cuerpo actual en el editor y no publica nada; puedes volver atrás desde el historial.
      </p>
      <div className="mt-4 flex flex-col gap-3">
        <Field label="Archivo .md" help="Máximo 1 MiB. No se descarga nada de las URL que contenga.">
          {({ id, describedBy }) => (
            <input
              id={id}
              type="file"
              accept=".md,.markdown,text/markdown,text/plain"
              aria-describedby={describedBy}
              onChange={async (e) => {
                const file = e.target.files?.[0];
                if (!file) return;
                if (file.size > MAX_IMPORT_BYTES) {
                  setError("El archivo supera 1 MiB.");
                  return;
                }
                const content = await file.text();
                setText(content);
                analyze(content);
              }}
            />
          )}
        </Field>
        <Field label="…o pega el texto">
          {({ id }) => <textarea id={id} rows={6} className={`${inputClass} font-mono`} value={text} onChange={(e) => setText(e.target.value)} />}
        </Field>
        <Button className="self-start" onClick={() => analyze(text)}>
          Analizar
        </Button>
        {error && <p role="alert" className="text-sm text-danger">{error}</p>}
        {result && (
          <div className="flex flex-col gap-3">
            {result.warnings.length > 0 ? (
              <Notice tone="warning" title={`${result.warnings.length} aviso(s): nada se descarta en silencio`}>
                <ul className="max-h-40 list-disc overflow-y-auto pl-5">
                  {result.warnings.map((w, i) => (
                    <li key={i}>
                      {w.line ? `Línea ${w.line}: ` : ""}
                      {w.message}
                    </li>
                  ))}
                </ul>
              </Notice>
            ) : (
              <Notice tone="success">Sin avisos: todo el contenido es compatible.</Notice>
            )}
            <div className="max-h-72 overflow-y-auto rounded border border-border p-4" aria-label="Vista previa de la importación">
              {result.doc.content.length ? <DocumentView doc={result.doc} locale={locale} /> : <p className="text-sm text-text-muted">El documento está vacío.</p>}
            </div>
          </div>
        )}
      </div>
      <div className="sticky -bottom-6 -mx-6 mt-5 flex flex-wrap justify-end gap-2 border-t border-border bg-surface px-6 py-4">
        <Button onClick={onCancel}>Cancelar</Button>
        <Button tone="primary" disabled={!result || result.doc.content.length === 0} onClick={() => result && onConfirm(result.doc)}>
          Reemplazar el cuerpo actual
        </Button>
      </div>
    </dialog>
  );
}
