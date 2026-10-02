// Public navigation (WEB-009): sticky, translucent deep navy. On phones the sections fold into a
// disclosure menu (a button with aria-expanded; Escape closes and returns focus). Without
// JavaScript the same element is a link to the footer navigation, so every section stays reachable.
import { useEffect, useRef, useState, type KeyboardEvent as ReactKeyboardEvent, type MouseEvent as ReactMouseEvent } from "react";
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
  const button = useRef<HTMLAnchorElement>(null);
  // Background change once the page leaves the top (#52): one observed 1 px sentinel, no scroll
  // listener and no render per scrolled pixel.
  const sentinel = useRef<HTMLDivElement>(null);
  const [scrolled, setScrolled] = useState(false);
  useEffect(() => setEnhanced(true), []);
  useEffect(() => {
    const el = sentinel.current;
    if (!el || typeof IntersectionObserver === "undefined") return;
    const observer = new IntersectionObserver(([e]) => setScrolled(!e.isIntersecting));
    observer.observe(el);
    return () => observer.disconnect();
  }, []);
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
    <>
    <div ref={sentinel} aria-hidden="true" className="pointer-events-none absolute inset-x-0 top-0 h-px" />
    {/* At the top it continues the navy page header; scrolled, it becomes translucent deep navy. */}
    <header
      data-scrolled={scrolled || undefined}
      className="sticky top-0 z-40 border-b border-border bg-navy transition-[background-color] duration-(--motion-fast) ease-(--ease-out) data-scrolled:bg-[rgb(6_17_31/0.9)] data-scrolled:backdrop-blur-sm"
    >
      <Frame className="flex min-h-16 items-center justify-between gap-6">
        <a href={homePath(locale)} aria-label="BrambiLab" className={`shrink-0 rounded-sm ${focusRing}`}>
          <BrambiLabLogo className="h-7 sm:h-8" />
        </a>
        <nav aria-label={t(locale, "nav.label")} className="flex min-w-0 items-center justify-end gap-3 md:flex-wrap md:gap-x-8 md:gap-y-1 md:py-2">
          <ul
            id="site-nav"
            className={`${open ? "flex" : "hidden"} absolute inset-x-0 top-full flex-col border-b border-border bg-deep px-4 py-4 sm:px-6 md:static md:flex md:flex-row md:flex-wrap md:items-center md:justify-end md:gap-x-7 md:gap-y-1 md:border-0 md:bg-transparent md:p-0`}
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
                        : `flex min-h-12 items-center text-2xl text-text-muted underline decoration-transparent decoration-1 underline-offset-8 hover:text-text hover:decoration-border-strong aria-[current=page]:text-text aria-[current=page]:decoration-signal md:min-h-10 md:text-sm ${focusRing}`
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
          {/* One element before and after hydration, so focus is never lost when JavaScript
              arrives: a link to the footer navigation that, enhanced, acts as the menu button. */}
          <a
            ref={button}
            href="#footer-nav"
            {...(enhanced && {
              role: "button",
              "aria-expanded": open,
              "aria-controls": "site-nav",
              onClick: (e: ReactMouseEvent) => {
                e.preventDefault();
                setOpen((o) => !o);
              },
              onKeyDown: (e: ReactKeyboardEvent) => {
                if (e.key !== " ") return; // a button also activates with Space
                e.preventDefault();
                setOpen((o) => !o);
              },
            })}
            className={`${toggle} ${focusRing}`}
          >
            {open ? t(locale, "nav.close") : t(locale, "nav.menu")}
          </a>
        </nav>
      </Frame>
    </header>
    </>
  );
}
