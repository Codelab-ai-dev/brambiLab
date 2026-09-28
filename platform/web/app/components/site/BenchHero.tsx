// "Live test bench" hero for the public home. The server renders the complete, static scene;
// in the browser the oscilloscope trace morphs (sine → square: analog → digital), a pulse
// travels along the traces and the grid lights up under the pointer. Motion is skipped with
// prefers-reduced-motion. Decorative graphics are aria-hidden; all text is real content.

import { useEffect, useRef } from "react";
import { t, type Locale, type MessageKey } from "~/i18n";

type Area = { code: string; label: MessageKey };

// Part-number style labels (decoration only); the area names below them are the content.
const areas: Area[] = [
  { code: "SW-01", label: "site.area.software" },
  { code: "IA-02", label: "site.area.ai" },
  { code: "MCU-03", label: "site.area.mcu" },
  { code: "IOT-04", label: "site.area.iot" },
  { code: "BOT-05", label: "site.area.robotics" },
];

const W = 640;
const H = 200;
const POINTS = 160;

/** y = tanh(k·sin x) / tanh(k): k≈1 is a sine, k≈12 a square wave. */
function tracePath(k: number, phase: number): string {
  const norm = Math.tanh(k);
  let d = "";
  for (let i = 0; i <= POINTS; i++) {
    const x = (i / POINTS) * W;
    const v = Math.tanh(k * Math.sin((i / POINTS) * Math.PI * 6 + phase)) / norm;
    const y = H / 2 - v * (H * 0.3);
    d += `${i === 0 ? "M" : "L"}${x.toFixed(1)} ${y.toFixed(1)}`;
  }
  return d;
}

// Static trace for the server and reduced motion: halfway between analog and digital.
const STATIC_TRACE = tracePath(3.5, 0);

function Oscilloscope({ locale }: { locale: Locale }) {
  const trace = useRef<SVGPathElement>(null);
  const glow = useRef<SVGPathElement>(null);

  useEffect(() => {
    if (window.matchMedia("(prefers-reduced-motion: reduce)").matches) return;
    let frame = 0;
    let visible = true;
    const observer = new IntersectionObserver(([e]) => (visible = e.isIntersecting));
    if (trace.current) observer.observe(trace.current);
    const start = performance.now();
    const tick = (now: number) => {
      frame = requestAnimationFrame(tick);
      if (!visible || document.hidden) return;
      const s = (now - start) / 1000;
      // k eases between 1 (sine) and 12 (square) and back every ~8 s.
      const k = 1 + 11 * (0.5 - 0.5 * Math.cos((s / 8) * Math.PI * 2));
      const d = tracePath(k, s * 1.6);
      trace.current?.setAttribute("d", d);
      glow.current?.setAttribute("d", d);
    };
    frame = requestAnimationFrame(tick);
    return () => {
      cancelAnimationFrame(frame);
      observer.disconnect();
    };
  }, []);

  return (
    <figure aria-hidden="true" className="bl-scope overflow-hidden rounded-lg border border-[var(--bench-edge)] bg-[var(--bench-panel)]">
      <figcaption className="flex items-center justify-between border-b border-[var(--bench-trace)] px-3 py-2 font-mono text-[0.7rem] tracking-wider text-[var(--bench-label)] uppercase">
        <span>{t(locale, "bench.scope")}</span>
        <span className="flex gap-1.5">
          {["var(--bench-signal)", "var(--bench-trace)", "var(--bench-trace)"].map((c, i) => (
            <span key={i} className="size-1.5 rounded-full" style={{ background: c }} />
          ))}
        </span>
      </figcaption>
      <svg viewBox={`0 0 ${W} ${H}`} className="block h-auto w-full" preserveAspectRatio="none">
        <defs>
          <pattern id="bl-scope-grid" width={W / 10} height={H / 5} patternUnits="userSpaceOnUse">
            <path d={`M ${W / 10} 0 L 0 0 0 ${H / 5}`} fill="none" stroke="var(--bench-grid-strong)" strokeWidth="1" />
          </pattern>
          <filter id="bl-scope-blur" x="-10%" y="-50%" width="120%" height="200%">
            <feGaussianBlur stdDeviation="4" />
          </filter>
        </defs>
        <rect width={W} height={H} fill="url(#bl-scope-grid)" />
        <line x1="0" y1={H / 2} x2={W} y2={H / 2} stroke="var(--bench-grid-strong)" strokeDasharray="2 6" />
        <path ref={glow} d={STATIC_TRACE} fill="none" stroke="var(--bench-signal)" strokeWidth="6" opacity="0.35" filter="url(#bl-scope-blur)" />
        <path ref={trace} d={STATIC_TRACE} fill="none" stroke="var(--bench-signal)" strokeWidth="2" strokeLinejoin="round" />
      </svg>
    </figure>
  );
}

function Modules({ locale }: { locale: Locale }) {
  return (
    <div className="relative">
      {/* Copper traces between the chips, with a pulse travelling along them (decorative). */}
      <svg aria-hidden="true" viewBox="0 0 1000 40" preserveAspectRatio="none" className="pointer-events-none absolute inset-x-[10%] top-1/2 hidden h-10 w-[80%] -translate-y-1/2 sm:block">
        <path d="M0 20 H1000" stroke="var(--bench-trace)" strokeWidth="3" fill="none" />
        <path d="M0 20 H1000" stroke="var(--bench-signal)" strokeWidth="3" fill="none" className="bl-pulse" pathLength="100" />
      </svg>
      <h2 className="sr-only">{t(locale, "bench.modules")}</h2>
      <ul className="relative grid grid-cols-2 gap-3 sm:grid-cols-5 sm:gap-4">
        {areas.map((a, i) => (
          <li
            key={a.code}
            className="bl-chip group flex flex-col gap-2 rounded-md last:col-span-2 sm:last:col-span-1 border border-[var(--bench-edge)] bg-[var(--bench-panel)] px-3 py-3 transition-colors hover:border-[var(--bench-signal)]"
            style={{ ["--i" as string]: i }}
          >
            <span className="flex items-center justify-between">
              <span className="font-mono text-sm font-medium tracking-wider text-[var(--bench-label)]">{a.code}</span>
              <span aria-hidden="true" className="bl-led size-2 rounded-full bg-[var(--bench-signal)]" />
            </span>
            <span className="text-base leading-tight font-semibold break-words hyphens-auto text-[var(--bench-text)] first-letter:uppercase sm:text-lg">{t(locale, a.label)}</span>
            {/* Chip pins, pure decoration. */}
            <span aria-hidden="true" className="flex justify-between px-1">
              {Array.from({ length: 6 }, (_, p) => (
                <span key={p} className="h-1.5 w-0.5 bg-[var(--bench-trace)] group-hover:bg-[var(--bench-signal)]" />
              ))}
            </span>
          </li>
        ))}
      </ul>
    </div>
  );
}

export function BenchHero({ locale }: { locale: Locale }) {
  const root = useRef<HTMLElement>(null);

  // The bench grid lights up under the pointer (mouse/pen only, never with reduced motion).
  useEffect(() => {
    const el = root.current;
    if (!el || window.matchMedia("(prefers-reduced-motion: reduce)").matches) return;
    let frame = 0;
    const move = (e: PointerEvent) => {
      if (e.pointerType === "touch") return;
      cancelAnimationFrame(frame);
      frame = requestAnimationFrame(() => {
        const r = el.getBoundingClientRect();
        el.style.setProperty("--mx", `${e.clientX - r.left}px`);
        el.style.setProperty("--my", `${e.clientY - r.top}px`);
        el.style.setProperty("--spot", "1");
      });
    };
    const leave = () => el.style.setProperty("--spot", "0");
    el.addEventListener("pointermove", move);
    el.addEventListener("pointerleave", leave);
    return () => {
      cancelAnimationFrame(frame);
      el.removeEventListener("pointermove", move);
      el.removeEventListener("pointerleave", leave);
    };
  }, []);

  return (
    <section ref={root} aria-labelledby="bench-title" className="bl-bench relative isolate overflow-hidden">
      <div className="relative mx-auto max-w-6xl px-4 pt-10 pb-16 sm:px-6 sm:pt-14 sm:pb-24">
        {/* Instrument status bar: only true facts (status, number of areas). */}
        <div className="flex flex-wrap items-center gap-x-6 gap-y-2 pb-4 font-mono text-[0.72rem] tracking-widest text-[var(--bench-label)] uppercase">
          <span className="flex items-center gap-2">
            <span aria-hidden="true" className="bl-led size-2 rounded-full bg-[var(--bench-signal)]" />
            {t(locale, "bench.status")}
          </span>
          <span>{t(locale, "bench.channels")}</span>
        </div>

        <div className="mt-10 grid items-center gap-10 lg:grid-cols-[minmax(0,1.1fr)_minmax(0,1fr)]">
          <div>
            <h1 id="bench-title" className="text-4xl leading-[1.05] font-semibold tracking-tight text-balance text-[var(--bench-text)] sm:text-6xl">
              {t(locale, "site.tagline")}
            </h1>
            <p className="mt-6 max-w-xl text-lg text-[var(--bench-muted)]">{t(locale, "site.lab")}</p>
            <a
              href="#publish"
              className="mt-8 inline-flex min-h-11 items-center gap-2 rounded-md border border-[var(--bench-signal)] px-5 font-medium text-[var(--bench-text)] transition-colors hover:bg-[var(--bench-signal)] hover:text-[var(--bench-bg)] focus-visible:outline-2 focus-visible:outline-offset-4 focus-visible:outline-[var(--bench-signal)]"
            >
              {t(locale, "bench.cta")} <span aria-hidden="true">↓</span>
            </a>
          </div>
          <Oscilloscope locale={locale} />
        </div>

        <div className="mt-14">
          <Modules locale={locale} />
        </div>
      </div>
    </section>
  );
}
