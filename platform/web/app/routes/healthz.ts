// Liveness of the SSR process only; API and database readiness live in Go.
export function loader() {
  return new Response("ok", {
    headers: { "Content-Type": "text/plain", "Cache-Control": "no-store" },
  });
}
