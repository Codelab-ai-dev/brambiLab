// Small admin UI kit: consistent labels, help, errors, buttons and notices across panel screens.
import { useId, type ComponentProps, type ReactNode } from "react";
import { Link } from "react-router";

const tones = {
  primary: "bg-primary text-primary-contrast hover:opacity-90",
  secondary: "border border-border bg-surface text-text hover:bg-surface-muted",
  danger: "border border-danger bg-surface text-danger hover:bg-surface-muted",
  ghost: "hover:bg-surface-muted",
} as const;

type Tone = keyof typeof tones;
const base =
  "inline-flex min-h-10 items-center justify-center gap-2 rounded-md px-4 text-sm font-medium focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-accent disabled:cursor-not-allowed disabled:opacity-50";

export function Button({ tone = "secondary", className = "", ...props }: ComponentProps<"button"> & { tone?: Tone }) {
  return <button type="button" {...props} className={`${base} ${tones[tone]} ${className}`} />;
}

export function LinkButton({ tone = "secondary", className = "", ...props }: ComponentProps<typeof Link> & { tone?: Tone }) {
  return <Link {...props} className={`${base} ${tones[tone]} ${className}`} />;
}

type FieldProps = {
  label: string;
  help?: string;
  error?: string;
  children: (ids: { id: string; describedBy?: string; invalid?: true }) => ReactNode;
};

/** Visible label, help and error wired with aria-describedby / aria-invalid. */
export function Field({ label, help, error, children }: FieldProps) {
  const id = useId();
  const describedBy = [help && `${id}-help`, error && `${id}-error`].filter(Boolean).join(" ") || undefined;
  return (
    <div className="flex flex-col gap-1">
      <label htmlFor={id} className="text-sm font-medium">
        {label}
      </label>
      {children({ id, describedBy, invalid: error ? true : undefined })}
      {help && (
        <p id={`${id}-help`} className="text-xs text-text-muted">
          {help}
        </p>
      )}
      {error && (
        <p id={`${id}-error`} className="text-xs text-danger">
          {error}
        </p>
      )}
    </div>
  );
}

export const inputClass =
  "min-h-10 w-full rounded-md border border-border-strong bg-surface px-3 py-2 text-sm aria-invalid:border-danger aria-invalid:border-2 focus-visible:outline-2 focus-visible:outline-offset-1 focus-visible:outline-accent";

export function Notice({ tone = "info", title, children }: { tone?: "info" | "warning" | "danger" | "success"; title?: string; children: ReactNode }) {
  const color = { info: "border-border", warning: "border-warning", danger: "border-danger", success: "border-success" }[tone];
  return (
    <div role={tone === "danger" ? "alert" : "status"} className={`rounded-md border-l-4 ${color} bg-surface-muted px-4 py-3 text-sm`}>
      {title && <p className="font-semibold">{title}</p>}
      <div className={title ? "mt-1" : undefined}>{children}</div>
    </div>
  );
}

export function PageHeader({ title, description, actions }: { title: string; description?: ReactNode; actions?: ReactNode }) {
  return (
    <div className="flex flex-wrap items-start justify-between gap-4 py-6">
      <div className="min-w-0">
        <h1 className="text-2xl font-semibold break-words">{title}</h1>
        {description && <div className="mt-1 text-sm text-text-muted">{description}</div>}
      </div>
      {actions && <div className="flex flex-wrap gap-2">{actions}</div>}
    </div>
  );
}

export function EmptyState({ title, children }: { title: string; children?: ReactNode }) {
  return (
    <div className="rounded-md border border-dashed border-border px-6 py-10 text-center">
      <p className="font-medium">{title}</p>
      {children && <div className="mt-3 text-sm text-text-muted">{children}</div>}
    </div>
  );
}
