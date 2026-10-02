// Visual column of the hero (#53). The server always renders the static image: the perception
// cloud projected with the same camera as the 3D scene, so both tell the same story. The Three.js
// scene (HeroScene, a separate chunk) is loaded only on wide screens with a fine pointer and hover,
// with WebGL, without reduced motion and without data saving. It replaces the relief pulse of #52
// as the one animated technical visual of the site.
// - The image stays until the scene has drawn its first frame, then a short cross-fade.
// - A load error, a render error or a lost WebGL context brings the image back for good.
// - The loop stops when the viewer pauses it, the column leaves the viewport or the tab is
//   hidden; turning on reduced motion during the visit unmounts the scene.
import { Component, lazy, Suspense, useEffect, useRef, useState, type CSSProperties, type ReactNode } from "react";
import { t, type Locale } from "~/i18n";
import { CLOUD, project } from "./perception";

const HeroScene = lazy(() => import("./HeroScene"));

// Static image: one dot path per brightness band (every second point keeps the HTML light) and the
// cyan line work, in a tall viewBox like the desktop column (cropped by "slice" elsewhere).
const W = 360;
const H = 640;
const STATIC = (() => {
  const { toScreen } = project(W, H);
  const bands = ["", "", ""];
  for (let i = 0; i < CLOUD.light.length; i += 2) {
    const p = toScreen(CLOUD.points[i * 3], CLOUD.points[i * 3 + 1], CLOUD.points[i * 3 + 2]);
    if (!p) continue;
    const band = CLOUD.light[i] > 0.66 ? 2 : CLOUD.light[i] > 0.4 ? 1 : 0;
    bands[band] += `M${Math.round(p[0])} ${Math.round(p[1])}h0`;
  }
  let lines = "";
  for (let i = 0; i < CLOUD.lines.length; i += 6) {
    const a = toScreen(CLOUD.lines[i], CLOUD.lines[i + 1], CLOUD.lines[i + 2]);
    const b = toScreen(CLOUD.lines[i + 3], CLOUD.lines[i + 4], CLOUD.lines[i + 5]);
    if (a && b) lines += `M${a[0].toFixed(1)} ${a[1].toFixed(1)}L${b[0].toFixed(1)} ${b[1].toFixed(1)}`;
  }
  return { bands, lines };
})();

export function PerceptionStatic({ className = "" }: { className?: string }) {
  return (
    <svg viewBox={`0 0 ${W} ${H}`} preserveAspectRatio="xMidYMid slice" aria-hidden="true" focusable="false" className={className} data-scene-static>
      {STATIC.bands.map((d, i) => (
        <path key={i} d={d} stroke="#c9d3de" strokeOpacity={[0.45, 0.65, 0.9][i]} strokeWidth="2" strokeLinecap="round" />
      ))}
      <path d={STATIC.lines} stroke="var(--color-signal)" strokeOpacity="0.6" strokeWidth="1" fill="none" />
    </svg>
  );
}

/** Whether this browser should get the 3D scene; re-evaluated when the preferences change. */
function useSceneEligibility() {
  const [eligible, setEligible] = useState(false);
  useEffect(() => {
    const reduce = window.matchMedia("(prefers-reduced-motion: reduce)");
    const wide = window.matchMedia("(min-width: 64rem) and (hover: hover) and (pointer: fine)");
    const saveData = (navigator as Navigator & { connection?: { saveData?: boolean } }).connection?.saveData === true;
    const webgl = hasWebGL();
    const update = () => setEligible(webgl && !saveData && !reduce.matches && wide.matches);
    update();
    reduce.addEventListener("change", update);
    wide.addEventListener("change", update);
    return () => {
      reduce.removeEventListener("change", update);
      wide.removeEventListener("change", update);
    };
  }, []);
  return eligible;
}

/** A capability probe, not a user-agent guess: can this browser create a WebGL context at all? */
function hasWebGL(): boolean {
  try {
    const canvas = document.createElement("canvas");
    const gl = (canvas.getContext("webgl2") ?? canvas.getContext("webgl")) as WebGLRenderingContext | null;
    if (!gl) return false;
    gl.getExtension("WEBGL_lose_context")?.loseContext(); // free the probe at once
    return true;
  } catch {
    return false;
  }
}

/** A failure inside the scene (or loading its chunk) never reaches the rest of the page. */
class SceneBoundary extends Component<{ onError: () => void; children: ReactNode }, { failed: boolean }> {
  state = { failed: false };
  static getDerivedStateFromError() {
    return { failed: true };
  }
  componentDidCatch() {
    this.props.onError();
  }
  render() {
    return this.state.failed ? null : this.props.children;
  }
}

/** True once the page has loaded and the browser is idle: the 3D chunk never competes with it. */
function useAfterLoad() {
  const [later, setLater] = useState(false);
  useEffect(() => {
    let idle = 0;
    const go = () => {
      const w = window as Window & { requestIdleCallback?: (cb: () => void, o?: { timeout: number }) => number; cancelIdleCallback?: (id: number) => void };
      idle = w.requestIdleCallback ? w.requestIdleCallback(() => setLater(true), { timeout: 1500 }) : window.setTimeout(() => setLater(true), 200);
    };
    if (document.readyState === "complete") go();
    else window.addEventListener("load", go, { once: true });
    return () => {
      window.removeEventListener("load", go);
      const w = window as Window & { cancelIdleCallback?: (id: number) => void };
      if (w.cancelIdleCallback) w.cancelIdleCallback(idle);
      else window.clearTimeout(idle);
    };
  }, []);
  return later;
}

export function HeroVisual({ locale }: { locale: Locale }) {
  const ref = useRef<HTMLElement>(null);
  const afterLoad = useAfterLoad();
  const eligible = useSceneEligibility() && afterLoad;
  const [failed, setFailed] = useState(false);
  const [ready, setReady] = useState(false);
  const [userPaused, setUserPaused] = useState(false);
  const [inView, setInView] = useState(true);
  const [tabVisible, setTabVisible] = useState(true);
  const active = eligible && !failed;

  useEffect(() => {
    if (!active) setReady(false);
  }, [active]);
  useEffect(() => {
    const el = ref.current;
    const observer = typeof IntersectionObserver === "undefined" || !el ? null : new IntersectionObserver(([e]) => setInView(e.isIntersecting));
    if (el) observer?.observe(el);
    const onVisibility = () => setTabVisible(document.visibilityState === "visible");
    onVisibility();
    document.addEventListener("visibilitychange", onVisibility);
    return () => {
      observer?.disconnect();
      document.removeEventListener("visibilitychange", onVisibility);
    };
  }, []);

  const playing = active && !userPaused && inView && tabVisible;
  return (
    <figure
      ref={ref}
      data-scene={active ? (ready ? "ready" : "loading") : "static"}
      data-playing={playing && ready ? "true" : "false"}
      className="bl-enter relative -mx-4 h-64 overflow-hidden border-t border-border bg-deep text-text-muted sm:-mx-6 sm:h-80 lg:col-span-3 lg:mx-0 lg:h-auto lg:min-h-[36rem] lg:border-t-0 lg:border-l"
      style={{ "--i": 3 } as CSSProperties}
    >
      <PerceptionStatic className={`absolute inset-0 h-full w-full transition-opacity duration-(--motion-base) ease-(--ease-out) ${ready ? "opacity-0" : "opacity-100"}`} />
      {active && (
        <SceneBoundary onError={() => setFailed(true)}>
          <Suspense fallback={null}>
            <HeroScene
              playing={playing}
              onReady={() => setReady(true)}
              onLost={() => setFailed(true)}
              className={`absolute inset-0 transition-opacity duration-(--motion-base) ease-(--ease-out) ${ready ? "opacity-100" : "opacity-0"}`}
            />
          </Suspense>
        </SceneBoundary>
      )}
      {active && ready && (
        <button
          type="button"
          onClick={() => setUserPaused((p) => !p)}
          className="bl-meta absolute top-3 right-3 inline-flex min-h-9 items-center gap-2 rounded-sm border border-border-strong bg-deep px-2.5 text-[0.6875rem] text-text hover:border-signal"
        >
          <svg viewBox="0 0 12 12" aria-hidden="true" className="size-3" fill="currentColor">
            {userPaused ? <path d="M3 1.5v9l7-4.5z" /> : <path d="M2.5 1.5h2.5v9H2.5zM7 1.5h2.5v9H7z" />}
          </svg>
          {t(locale, userPaused ? "motion.resume" : "motion.pause")}
        </button>
      )}
      <svg viewBox="0 0 24 24" aria-hidden="true" className="absolute right-4 bottom-4 size-6 text-signal" fill="none" stroke="currentColor" strokeWidth="1">
        <path d="M0 12h24M12 0v24" />
        <circle cx="12" cy="12" r="6" />
      </svg>
      <figcaption className="bl-meta absolute bottom-3 left-3 bg-deep px-1.5 py-0.5 text-[0.6875rem] text-text-muted">{t(locale, "visual.conceptual")}</figcaption>
    </figure>
  );
}
