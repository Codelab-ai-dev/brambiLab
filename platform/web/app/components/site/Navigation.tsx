// Public navigation (WEB-009): sticky, translucent deep navy. On phones the sections fold into a
// disclosure menu (button with aria-expanded, Escape closes and returns focus). Without
// JavaScript the same control is a link to the footer navigation, so every section stays reachable.
import { useEffect, useRef, useState } from "react";
import { BrambiLabLogo } from "~/components/brand/BrambiLabLogo";
import { t, type Locale, type MessageKey } from "~/i18n";
import { homePath, sectionPath, type Section } from "~/site/paths";
import type { Switcher } from "~/site/types";
import { Frame } from "./editorial";

export const navItems: { section: Section; label: MessageKey }[] = [
  { section: "projects", label: "nav.projects" },
  { section: "articles", label: "nav.articles" },
  { section: "search", label: "nav.search" },
  { section: "about", label: "nav.about" },
  { section: "contact", label: "nav.contact" },
];

export function isCurrent(pathname: string, href: string) {
  return pathname === href || pathname.startsWith(`${href}/`);
}

const focusRing = "focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-signal";

export function Navigation({ locale, pathname, switcher }: { locale: Locale; pathname: string; switcher: Switcher }) {
  const other: Locale = locale === "es" ? "en" : "es";
  // First render (server and hydration) is the no-JS version; the effect upgrades the control.
  const [enhanced, setEnhanced] = useState(false);
  const [open, setOpen] = useState(false);
  const button = useRef<HTMLButtonElement>(null);
  useEffect(() => setEnhanced(true), []);
  useEffect(() => {
    if (!open) return;
    const onKey = (e: KeyboardEvent) => {
      if (e.key !== "Escape") return;
      setOpen(false);
      button.current?.focus();
    };
    document.addEventListener("keydown", onKey);
    return () => document.removeEventListener("keydown", onKey);
  }, [open]);

  const toggle = "inline-flex min-h-11 items-center gap-2 rounded-sm border border-border-strong px-3 font-mono text-xs tracking-[0.12em] uppercase md:hidden";
  return (
    <header className="sticky top-0 z-40 border-b border-border bg-[rgb(6_17_31/0.9)] backdrop-blur-sm">
      <Frame className="flex min-h-16 items-center justify-between gap-6">
        <a href={homePath(locale)} aria-label="BrambiLab" className={`shrink-0 rounded-sm ${focusRing}`}>
          <BrambiLabLogo className="h-7 sm:h-8" />
        </a>
        <nav aria-label={t(locale, "nav.label")} className="flex items-center gap-3 md:gap-8">
          <ul
            id="site-nav"
            className={`${open ? "flex" : "hidden"} absolute inset-x-0 top-full flex-col border-b border-border bg-deep px-4 py-4 sm:px-6 md:static md:flex md:flex-row md:items-center md:gap-7 md:border-0 md:bg-transparent md:p-0`}
          >
            {navItems.map((n) => {
              const href = sectionPath(locale, n.section);
              const contact = n.section === "contact";
              return (
                <li key={n.section}>
                  <a
                    href={href}
                    aria-current={isCurrent(pathname, href) ? "page" : undefined}
                    className={
                      contact
                        ? `flex min-h-12 items-center text-2xl text-text md:min-h-10 md:rounded-sm md:border md:border-border-strong md:px-4 md:text-sm md:hover:border-signal ${focusRing}`
                        : `flex min-h-12 items-center text-2xl text-text-muted underline-offset-8 hover:text-text aria-[current=page]:text-text aria-[current=page]:underline aria-[current=page]:decoration-signal md:min-h-10 md:text-sm ${focusRing}`
                    }
                  >
                    {t(locale, n.label)}
                  </a>
                </li>
              );
            })}
          </ul>
          <a
            href={switcher.href}
            hrefLang={other}
            lang={other}
            aria-label={switcher.available ? t(locale, "switch.available") : t(locale, "switch.missing")}
            data-switch={switcher.available ? "same" : "index"}
            className={`inline-flex min-h-11 items-center rounded-sm px-2 font-mono text-xs tracking-[0.12em] text-text-muted uppercase hover:text-text md:min-h-10 ${focusRing}`}
          >
            {other}
          </a>
          {enhanced ? (
            <button ref={button} type="button" aria-expanded={open} aria-controls="site-nav" onClick={() => setOpen((o) => !o)} className={`${toggle} ${focusRing}`}>
              {open ? t(locale, "nav.close") : t(locale, "nav.menu")}
            </button>
          ) : (
            <a href="#footer-nav" className={`${toggle} ${focusRing}`}>
              {t(locale, "nav.menu")}
            </a>
          )}
        </nav>
      </Frame>
    </header>
  );
}
