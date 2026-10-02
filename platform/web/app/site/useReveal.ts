// Section reveals (#52): progressive enhancement over complete server HTML. Only elements marked
// data-reveal that are still below the fold when the page becomes interactive are hidden, so no
// visible content ever flashes away; each appears once when it enters the viewport and stays.
// Without JavaScript, without IntersectionObserver or with reduced motion, nothing is hidden.
import { useEffect } from "react";

export function useReveal(key: string) {
  useEffect(() => {
    if (typeof IntersectionObserver === "undefined") return;
    const reduce = window.matchMedia("(prefers-reduced-motion: reduce)");
    if (reduce.matches) return;
    const pending = new Set<Element>();
    const show = (el: Element, instant = false) => {
      if (!pending.delete(el)) return;
      if (instant) el.setAttribute("data-reveal-instant", "");
      el.removeAttribute("data-reveal-pending");
      observer.unobserve(el);
    };
    const observer = new IntersectionObserver(
      (entries) => {
        for (const e of entries) if (e.isIntersecting) show(e.target);
      },
      { rootMargin: "0px 0px -8% 0px" },
    );
    for (const el of document.querySelectorAll("[data-reveal]")) {
      // Above or inside the viewport: already seen, never hidden.
      if (el.getBoundingClientRect().top < window.innerHeight) continue;
      el.setAttribute("data-reveal-pending", "");
      pending.add(el);
      observer.observe(el);
    }
    // Keyboard focus inside a pending section shows it at once, without the transition.
    const onFocus = (e: FocusEvent) => {
      const el = e.target instanceof Element ? e.target.closest("[data-reveal-pending]") : null;
      if (el) show(el, true);
    };
    // Turning reduced motion on during the visit shows everything that is still waiting.
    const onPreference = () => {
      if (reduce.matches) for (const el of [...pending]) show(el, true);
    };
    document.addEventListener("focusin", onFocus);
    reduce.addEventListener("change", onPreference);
    return () => {
      observer.disconnect();
      document.removeEventListener("focusin", onFocus);
      reduce.removeEventListener("change", onPreference);
      for (const el of pending) el.removeAttribute("data-reveal-pending");
    };
  }, [key]);
}
