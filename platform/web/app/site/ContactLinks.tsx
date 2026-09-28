import { t, type Locale } from "~/i18n";
import type { PublicSite } from "./types";

const kindMark = { github: "GH", linkedin: "IN", website: "WWW", other: "↗" } as const;

/** Configured e-mail and links only; the API accepts just https:// URLs. */
export function ContactLinks({ locale, site }: { locale: Locale; site: PublicSite }) {
  return (
    <ul className="flex flex-wrap gap-3">
      {site.contact_email && (
        <li>
          <a href={`mailto:${site.contact_email}`} className="inline-flex min-h-11 items-center gap-3 rounded-md border border-border-strong px-4 hover:border-accent focus-visible:outline-2 focus-visible:outline-accent">
            <span className="font-mono text-xs text-accent">@</span>
            <span>
              <span className="sr-only">{t(locale, "contact.email")}: </span>
              {site.contact_email}
            </span>
          </a>
        </li>
      )}
      {site.links.map((l) => (
        <li key={l.url}>
          <a href={l.url} rel="noopener noreferrer me" className="inline-flex min-h-11 items-center gap-3 rounded-md border border-border-strong px-4 hover:border-accent focus-visible:outline-2 focus-visible:outline-accent">
            <span aria-hidden="true" className="font-mono text-xs text-accent">{kindMark[l.kind]}</span>
            {l.label}
          </a>
        </li>
      ))}
    </ul>
  );
}
