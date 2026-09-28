// Visual editor for the admin panel (WEB-003). Tiptap runs only in the browser
// (immediatelyRender: false): the server renders the frame and toolbar, never the editor itself.
// Every change is converted to the canonical document; the parent decides when to save.

import { NodeSelection } from "@tiptap/pm/state";
import { useEditor, useEditorState, EditorContent, type Editor } from "@tiptap/react";
import { useId, useRef, useState, type KeyboardEvent, type ReactNode } from "react";
import type { Asset, AssetKind, Locale } from "./api-types";
import { editorExtensions } from "./editor-extensions";
import { isSafeHref, languagePattern, parseYouTube, type Doc } from "./schema";
import { UnsupportedContentError, toCanonical, toTiptap } from "./tiptap-adapter";

// Admin UI is Spanish for now (web-v1.md §9); strings live here to move into a catalog later.
const m = {
  toolbar: "Formato del contenido",
  bold: "Negrita",
  italic: "Cursiva",
  code: "Código en línea",
  link: "Enlace",
  h2: "Título 2",
  h3: "Título 3",
  h4: "Título 4",
  bulletList: "Lista con viñetas",
  orderedList: "Lista numerada",
  quote: "Cita",
  codeBlock: "Bloque de código",
  rule: "Separador",
  table: "Insertar tabla",
  youtube: "YouTube",
  undo: "Deshacer",
  redo: "Rehacer",
  image: "Imagen",
  video: "Vídeo",
  download: "Descarga",
  mediaUnavailable: "Los medios se insertan desde el editor de un contenido.",
  mediaHelp: "Imagen, vídeo y descarga usan la biblioteca de medios; también puedes pegar o soltar imágenes.",
  mediaSelected: "Medio seleccionado",
  alt: "Texto alternativo",
  altHelp: "Describe la imagen para quien no la ve; vacío sólo si es decorativa.",
  caption: "Pie (opcional)",
  label: "Etiqueta del enlace de descarga",
  poster: "Póster del vídeo",
  choosePoster: "Elegir póster",
  removePoster: "Quitar póster",
  applyMedia: "Aplicar",
  tableTools: "Herramientas de tabla",
  addRow: "Añadir fila",
  addColumn: "Añadir columna",
  deleteRow: "Quitar fila",
  deleteColumn: "Quitar columna",
  deleteTable: "Quitar tabla",
  linkLabel: "Dirección del enlace",
  linkHelp: "https://…, mailto:…, /ruta interna o #ancla.",
  linkInvalid: "Usa una dirección http(s), mailto, una ruta que empiece por / o un ancla #.",
  linkApply: "Aplicar enlace",
  linkRemove: "Quitar enlace",
  youtubeLabel: "Enlace o id del vídeo de YouTube",
  youtubeHelp: "Por ejemplo https://youtu.be/dQw4w9WgXcQ. No se carga nada hasta que el visitante lo reproduce.",
  youtubeInvalid: "No es un enlace ni un id de YouTube válido.",
  youtubeInsert: "Insertar vídeo",
  languageLabel: "Lenguaje del bloque de código",
  languageHelp: "Opcional, en minúsculas: go, ts, python, c++…",
  languageInvalid: "Sólo minúsculas, números y + # . _ - (máximo 32).",
  languageApply: "Aplicar lenguaje",
  cancel: "Cancelar",
  editorLabel: "Contenido",
  loading: "Cargando el editor…",
};

type Panel = null | "link" | "youtube" | "language";

/** What the editor needs from the panel to work with library assets (WEB-004). */
export type MediaBridge = {
  locale: Locale;
  pick: (kind: AssetKind, title: string) => Promise<Asset | null>;
  /** Uploads a pasted/dropped image; resolves to null on failure (the caller reports it). */
  upload: (file: File) => Promise<Asset | null>;
};

const mediaTypes = new Set(["image", "video", "download"]);
const pastedImageTypes = new Set(["image/jpeg", "image/png", "image/webp"]);

type Props = {
  initialDoc: Doc;
  media?: MediaBridge;
  /** Called with the canonical document after every change, plus notes about normalizations. */
  onChange: (doc: Doc, notes: string[]) => void;
  /** Called when the content cannot be expressed canonically (e.g. merged cells). */
  onInvalid: (message: string) => void;
  readOnly?: boolean;
};

export function ContentEditor({ initialDoc, onChange, onInvalid, readOnly = false, media }: Props) {
  // Tiptap keeps the options it was created with; read the latest bridge through a ref.
  const mediaRef = useRef(media);
  mediaRef.current = media;
  const editorRef = useRef<Editor | null>(null);

  // Pasted or dropped images become private assets, inserted once ready. Never data URLs.
  const insertFiles = (files: File[], pos?: number) => {
    const bridge = mediaRef.current;
    const images = files.filter((f) => pastedImageTypes.has(f.type));
    if (!bridge || images.length === 0) return false;
    for (const file of images) {
      void bridge.upload(file).then((asset) => {
        const e = editorRef.current;
        if (!asset || !e) return;
        const node = { type: "image", attrs: { assetId: asset.id, alt: asset.texts[bridge.locale]?.alt ?? "", caption: asset.texts[bridge.locale]?.caption || null } };
        if (pos !== undefined) e.chain().insertContentAt(pos, node).run();
        else e.chain().focus().insertContentAt(e.state.selection.to, node).run();
      });
    }
    return true;
  };

  const editor = useEditor({
    extensions: editorExtensions,
    content: toTiptap(initialDoc),
    editable: !readOnly,
    immediatelyRender: false,
    editorProps: {
      attributes: { "aria-label": m.editorLabel, "aria-multiline": "true", role: "textbox", class: "px-4 py-3" },
      handlePaste: (_view, event) => insertFiles(Array.from(event.clipboardData?.files ?? [])),
      handleDrop: (view, event) => {
        const files = Array.from(event.dataTransfer?.files ?? []);
        if (files.length === 0) return false;
        const pos = view.posAtCoords({ left: event.clientX, top: event.clientY })?.pos;
        return insertFiles(files, pos);
      },
    },
    onCreate: ({ editor }) => {
      editorRef.current = editor;
    },
    onUpdate: ({ editor }) => {
      try {
        const { doc, notes } = toCanonical(editor.getJSON());
        onChange(doc, notes);
      } catch (err) {
        onInvalid(err instanceof UnsupportedContentError ? err.message : "El contenido no se pudo convertir.");
      }
    },
  });

  return (
    <div className="bl-editor rounded-md border border-border bg-surface">
      <Toolbar editor={editor} disabled={readOnly || !editor} media={readOnly ? undefined : media} />
      {editor ? <EditorContent editor={editor} /> : <p className="px-4 py-3 text-text-muted">{m.loading}</p>}
    </div>
  );
}

function Toolbar({ editor, disabled, media }: { editor: Editor | null; disabled: boolean; media?: MediaBridge }) {
  const [panel, setPanel] = useState<Panel>(null);
  const mediaNoteId = useId();
  const state = useEditorState({
    editor,
    selector: ({ editor: e }) =>
      e
        ? {
            bold: e.isActive("bold"),
            italic: e.isActive("italic"),
            code: e.isActive("code"),
            link: e.isActive("link"),
            h2: e.isActive("heading", { level: 2 }),
            h3: e.isActive("heading", { level: 3 }),
            h4: e.isActive("heading", { level: 4 }),
            bulletList: e.isActive("bulletList"),
            orderedList: e.isActive("orderedList"),
            quote: e.isActive("blockquote"),
            codeBlock: e.isActive("codeBlock"),
            table: e.isActive("table"),
            href: (e.getAttributes("link").href as string | undefined) ?? "",
            language: (e.getAttributes("codeBlock").language as string | undefined) ?? "",
            canUndo: e.can().undo(),
            canRedo: e.can().redo(),
            // A selected media node (click on it) can be edited in place.
            mediaNode:
              e.state.selection instanceof NodeSelection && mediaTypes.has(e.state.selection.node.type.name)
                ? { type: e.state.selection.node.type.name, attrs: e.state.selection.node.attrs as Record<string, string | null> }
                : null,
          }
        : null,
  });
  const chain = () => editor!.chain().focus();
  // Blocks (media, YouTube) go after the selection: inserting must never replace what is selected,
  // e.g. the media node inserted just before.
  const insertBlock = (node: Record<string, unknown>) => chain().insertContentAt(editor!.state.selection.to, node).run();
  const s = state ?? ({} as NonNullable<typeof state>);

  return (
    <div className="border-b border-border">
      <RovingToolbar label={m.toolbar}>
        <Group>
          <ToolButton label={m.bold} short="N" pressed={s.bold} disabled={disabled} onClick={() => chain().toggleBold().run()} className="font-bold" />
          <ToolButton label={m.italic} short="C" pressed={s.italic} disabled={disabled} onClick={() => chain().toggleItalic().run()} className="italic" />
          <ToolButton label={m.code} short="</>" pressed={s.code} disabled={disabled} onClick={() => chain().toggleCode().run()} className="font-mono" />
          <ToolButton label={m.link} short="Enlace" pressed={s.link} disabled={disabled} expanded={panel === "link"} onClick={() => setPanel(panel === "link" ? null : "link")} />
        </Group>
        <Group>
          <ToolButton label={m.h2} short="T2" pressed={s.h2} disabled={disabled} onClick={() => chain().toggleHeading({ level: 2 }).run()} />
          <ToolButton label={m.h3} short="T3" pressed={s.h3} disabled={disabled} onClick={() => chain().toggleHeading({ level: 3 }).run()} />
          <ToolButton label={m.h4} short="T4" pressed={s.h4} disabled={disabled} onClick={() => chain().toggleHeading({ level: 4 }).run()} />
        </Group>
        <Group>
          <ToolButton label={m.bulletList} short="• Lista" pressed={s.bulletList} disabled={disabled} onClick={() => chain().toggleBulletList().run()} />
          <ToolButton label={m.orderedList} short="1. Lista" pressed={s.orderedList} disabled={disabled} onClick={() => chain().toggleOrderedList().run()} />
          <ToolButton label={m.quote} short="Cita" pressed={s.quote} disabled={disabled} onClick={() => chain().toggleBlockquote().run()} />
          <ToolButton
            label={m.codeBlock}
            short="Código"
            pressed={s.codeBlock}
            disabled={disabled}
            onClick={() => {
              const wasActive = s.codeBlock;
              chain().toggleCodeBlock().run();
              setPanel(wasActive ? null : "language");
            }}
          />
          <ToolButton label={m.rule} short="—" disabled={disabled} onClick={() => chain().setHorizontalRule().run()} />
          <ToolButton label={m.table} short="Tabla" disabled={disabled || s.table} onClick={() => chain().insertTable({ rows: 3, cols: 2, withHeaderRow: true }).run()} />
          <ToolButton label={m.youtube} short="YouTube" disabled={disabled} expanded={panel === "youtube"} onClick={() => setPanel(panel === "youtube" ? null : "youtube")} />
        </Group>
        <Group>
          <ToolButton
            label={m.image}
            short="Imagen"
            disabled={disabled || !media}
            describedBy={mediaNoteId}
            onClick={async () => {
              const a = await media!.pick("image", "Insertar imagen");
              if (a) insertBlock({ type: "image", attrs: { assetId: a.id, alt: a.texts[media!.locale]?.alt ?? "", caption: a.texts[media!.locale]?.caption || null } });
            }}
          />
          <ToolButton
            label={m.video}
            short="Vídeo"
            disabled={disabled || !media}
            describedBy={mediaNoteId}
            onClick={async () => {
              const a = await media!.pick("video", "Insertar vídeo");
              if (a) insertBlock({ type: "video", attrs: { assetId: a.id, caption: a.texts[media!.locale]?.caption || null, posterAssetId: null } });
            }}
          />
          <ToolButton
            label={m.download}
            short="Descarga"
            disabled={disabled || !media}
            describedBy={mediaNoteId}
            onClick={async () => {
              const a = await media!.pick("resource", "Insertar descarga");
              if (a) insertBlock({ type: "download", attrs: { assetId: a.id, label: a.original_name } });
            }}
          />
        </Group>
        <Group>
          <ToolButton label={m.undo} short="↶" disabled={disabled || !s.canUndo} onClick={() => chain().undo().run()} />
          <ToolButton label={m.redo} short="↷" disabled={disabled || !s.canRedo} onClick={() => chain().redo().run()} />
        </Group>
      </RovingToolbar>

      {s.table && !disabled && (
        <RovingToolbar label={m.tableTools}>
          <Group>
            <ToolButton label={m.addRow} short={m.addRow} onClick={() => chain().addRowAfter().run()} />
            <ToolButton label={m.addColumn} short={m.addColumn} onClick={() => chain().addColumnAfter().run()} />
            <ToolButton label={m.deleteRow} short={m.deleteRow} onClick={() => chain().deleteRow().run()} />
            <ToolButton label={m.deleteColumn} short={m.deleteColumn} onClick={() => chain().deleteColumn().run()} />
            <ToolButton label={m.deleteTable} short={m.deleteTable} onClick={() => chain().deleteTable().run()} tone="danger" />
          </Group>
        </RovingToolbar>
      )}

      <p id={mediaNoteId} className="px-3 pb-2 text-xs text-text-muted">
        {media ? m.mediaHelp : m.mediaUnavailable}
      </p>

      {editor && media && s.mediaNode && (
        <MediaNodeForm
          key={JSON.stringify(s.mediaNode)}
          node={s.mediaNode}
          media={media}
          onApply={(attrs) => chain().updateAttributes(s.mediaNode!.type, attrs).run()}
        />
      )}

      {editor && panel === "link" && (
        <InlineForm
          key={`link-${s.href}`}
          label={m.linkLabel}
          help={m.linkHelp}
          initial={s.href}
          submit={m.linkApply}
          validate={(v) => (isSafeHref(normalizeHref(v)) ? null : m.linkInvalid)}
          onSubmit={(v) => {
            chain().extendMarkRange("link").setLink({ href: normalizeHref(v) }).run();
            setPanel(null);
          }}
          extra={s.link ? { label: m.linkRemove, onClick: () => (chain().extendMarkRange("link").unsetLink().run(), setPanel(null)) } : undefined}
          onCancel={() => setPanel(null)}
        />
      )}
      {editor && panel === "youtube" && (
        <InlineForm
          label={m.youtubeLabel}
          help={m.youtubeHelp}
          initial=""
          submit={m.youtubeInsert}
          validate={(v) => (parseYouTube(v) ? null : m.youtubeInvalid)}
          onSubmit={(v) => {
            insertBlock({ type: "youtube", attrs: parseYouTube(v)! });
            setPanel(null);
          }}
          onCancel={() => setPanel(null)}
        />
      )}
      {editor && panel === "language" && s.codeBlock && (
        <InlineForm
          key={`lang-${s.language}`}
          label={m.languageLabel}
          help={m.languageHelp}
          initial={s.language}
          submit={m.languageApply}
          validate={(v) => (v === "" || languagePattern.test(v) ? null : m.languageInvalid)}
          onSubmit={(v) => {
            chain().updateAttributes("codeBlock", { language: v || null }).run();
            setPanel(null);
          }}
          onCancel={() => setPanel(null)}
        />
      )}
    </div>
  );
}

// Edits the document's own alt/caption/label/poster for the selected media node. These values
// live in the revision snapshot; the library texts are only defaults.
function MediaNodeForm({ node, media, onApply }: { node: { type: string; attrs: Record<string, string | null> }; media: MediaBridge; onApply: (attrs: Record<string, string | null>) => void }) {
  const [alt, setAlt] = useState(node.attrs.alt ?? "");
  const [caption, setCaption] = useState(node.attrs.caption ?? "");
  const [label, setLabel] = useState(node.attrs.label ?? "");
  const [poster, setPoster] = useState(node.attrs.posterAssetId ?? null);
  const [error, setError] = useState<string | null>(null);
  const id = useId();
  return (
    <form
      aria-label={m.mediaSelected}
      className="flex flex-wrap items-end gap-3 border-t border-border bg-surface-muted px-3 py-2"
      onSubmit={(e) => {
        e.preventDefault();
        if (node.type === "download" && !label.trim()) {
          setError("La descarga necesita una etiqueta.");
          return;
        }
        setError(null);
        if (node.type === "image") onApply({ alt, caption: caption || null });
        else if (node.type === "video") onApply({ caption: caption || null, posterAssetId: poster });
        else onApply({ label: label.trim() });
      }}
    >
      <p className="w-full text-xs font-semibold">{m.mediaSelected}</p>
      {node.type === "image" && (
        <div className="flex min-w-56 flex-1 flex-col gap-1">
          <label htmlFor={`${id}-alt`} className="text-sm font-medium">{m.alt}</label>
          <input id={`${id}-alt`} value={alt} maxLength={300} onChange={(e) => setAlt(e.target.value)} aria-describedby={`${id}-alt-help`} className="min-h-9 rounded border border-border-strong bg-surface px-2" />
          <p id={`${id}-alt-help`} className="text-xs text-text-muted">{m.altHelp}</p>
        </div>
      )}
      {node.type !== "download" && (
        <div className="flex min-w-56 flex-1 flex-col gap-1">
          <label htmlFor={`${id}-caption`} className="text-sm font-medium">{m.caption}</label>
          <input id={`${id}-caption`} value={caption} maxLength={500} onChange={(e) => setCaption(e.target.value)} className="min-h-9 rounded border border-border-strong bg-surface px-2" />
        </div>
      )}
      {node.type === "download" && (
        <div className="flex min-w-56 flex-1 flex-col gap-1">
          <label htmlFor={`${id}-label`} className="text-sm font-medium">{m.label}</label>
          <input id={`${id}-label`} value={label} maxLength={200} onChange={(e) => setLabel(e.target.value)} aria-invalid={error ? true : undefined} className="min-h-9 rounded border border-border-strong bg-surface px-2 aria-invalid:border-danger" />
        </div>
      )}
      {node.type === "video" && (
        <div className="flex flex-col gap-1">
          <span className="text-sm font-medium">{m.poster}</span>
          <div className="flex gap-2">
            <button type="button" className="min-h-9 rounded border border-border-strong bg-surface px-3 text-sm" onClick={async () => {
              const a = await media.pick("image", "Elegir póster del vídeo");
              if (a) setPoster(a.id);
            }}>
              {poster ? "Cambiar póster" : m.choosePoster}
            </button>
            {poster && <button type="button" className="min-h-9 rounded px-3 text-sm text-danger" onClick={() => setPoster(null)}>{m.removePoster}</button>}
          </div>
        </div>
      )}
      <button type="submit" className="min-h-9 rounded bg-primary px-3 text-sm font-medium text-primary-contrast">{m.applyMedia}</button>
      {error && <p role="alert" className="w-full text-xs text-danger">{error}</p>}
    </form>
  );
}

/** "example.com/x" → "https://example.com/x"; internal paths, anchors and schemes stay as typed. */
function normalizeHref(v: string): string {
  const value = v.trim();
  if (/^(https?:|mailto:|\/|#)/i.test(value) || value.includes(":")) return value;
  return `https://${value}`;
}

// WAI-ARIA toolbar: one Tab stop, arrow keys move between enabled buttons.
function RovingToolbar({ label, children }: { label: string; children: ReactNode }) {
  const ref = useRef<HTMLDivElement>(null);
  const onKeyDown = (e: KeyboardEvent<HTMLDivElement>) => {
    const keys = ["ArrowRight", "ArrowLeft", "Home", "End"];
    if (!keys.includes(e.key)) return;
    const buttons = Array.from(ref.current?.querySelectorAll<HTMLButtonElement>("button:not(:disabled)") ?? []);
    const i = buttons.indexOf(document.activeElement as HTMLButtonElement);
    const next =
      e.key === "Home" ? 0 : e.key === "End" ? buttons.length - 1 : (i + (e.key === "ArrowRight" ? 1 : -1) + buttons.length) % buttons.length;
    buttons.forEach((b, j) => (b.tabIndex = j === next ? 0 : -1));
    buttons[next]?.focus();
    e.preventDefault();
  };
  const onFocus = () => {
    // Keep exactly one tabbable button: the focused one, or the first enabled one.
    const buttons = Array.from(ref.current?.querySelectorAll<HTMLButtonElement>("button:not(:disabled)") ?? []);
    const current = buttons.indexOf(document.activeElement as HTMLButtonElement);
    buttons.forEach((b, j) => (b.tabIndex = j === (current >= 0 ? current : 0) ? 0 : -1));
  };
  return (
    <div ref={ref} role="toolbar" aria-label={label} onKeyDown={onKeyDown} onFocus={onFocus} className="flex flex-wrap items-center gap-2 p-2">
      {children}
    </div>
  );
}

function Group({ children }: { children: ReactNode }) {
  return <div role="group" className="flex flex-wrap items-center gap-0.5 rounded-md bg-surface-muted p-0.5">{children}</div>;
}

type ToolButtonProps = {
  label: string;
  short: string;
  onClick: () => void;
  pressed?: boolean;
  expanded?: boolean;
  disabled?: boolean;
  describedBy?: string;
  className?: string;
  tone?: "danger";
};

function ToolButton({ label, short, onClick, pressed, expanded, disabled, describedBy, className = "", tone }: ToolButtonProps) {
  const toneClass = tone === "danger" ? "text-danger" : "";
  return (
    <button
      type="button"
      // The visible short label is decorative when it differs; the accessible name is the full label.
      aria-label={short === label ? undefined : label}
      title={label}
      aria-pressed={pressed === undefined ? undefined : pressed}
      aria-expanded={expanded === undefined ? undefined : expanded}
      aria-describedby={describedBy}
      disabled={disabled}
      onMouseDown={(e) => e.preventDefault() /* keep the editor selection */}
      onClick={onClick}
      className={`min-h-9 min-w-9 rounded px-2 text-sm ${toneClass} ${className} hover:bg-surface focus-visible:outline-2 focus-visible:outline-accent aria-pressed:bg-primary aria-pressed:text-primary-contrast disabled:cursor-not-allowed disabled:opacity-40`}
    >
      {short}
    </button>
  );
}

type InlineFormProps = {
  label: string;
  help: string;
  initial: string;
  submit: string;
  validate: (value: string) => string | null;
  onSubmit: (value: string) => void;
  onCancel: () => void;
  extra?: { label: string; onClick: () => void };
};

function InlineForm({ label, help, initial, submit, validate, onSubmit, onCancel, extra }: InlineFormProps) {
  const [value, setValue] = useState(initial);
  const [error, setError] = useState<string | null>(null);
  const id = useId();
  return (
    <form
      className="flex flex-wrap items-end gap-2 border-t border-border bg-surface-muted px-3 py-2"
      onSubmit={(e) => {
        e.preventDefault();
        const problem = validate(value);
        setError(problem);
        if (!problem) onSubmit(value.trim());
      }}
      onKeyDown={(e) => e.key === "Escape" && onCancel()}
    >
      <div className="flex min-w-64 flex-1 flex-col gap-1">
        <label htmlFor={id} className="text-sm font-medium">
          {label}
        </label>
        <input
          id={id}
          autoFocus
          value={value}
          onChange={(e) => setValue(e.target.value)}
          aria-invalid={error ? true : undefined}
          aria-describedby={`${id}-help${error ? ` ${id}-error` : ""}`}
          className="min-h-9 rounded border border-border-strong bg-surface px-2 py-1 aria-invalid:border-2 aria-invalid:border-danger focus-visible:outline-2 focus-visible:outline-accent"
        />
        <p id={`${id}-help`} className="text-xs text-text-muted">
          {help}
        </p>
        {error && (
          <p id={`${id}-error`} role="alert" className="text-xs text-danger">
            {error}
          </p>
        )}
      </div>
      <button type="submit" className="min-h-9 rounded bg-primary px-3 py-1 text-sm font-medium text-primary-contrast">
        {submit}
      </button>
      {extra && (
        <button type="button" onClick={extra.onClick} className="rounded px-3 py-1 text-sm text-danger hover:bg-surface">
          {extra.label}
        </button>
      )}
      <button type="button" onClick={onCancel} className="rounded px-3 py-1 text-sm hover:bg-surface">
        {m.cancel}
      </button>
    </form>
  );
}
