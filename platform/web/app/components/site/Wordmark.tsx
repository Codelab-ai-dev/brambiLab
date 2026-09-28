// Typographic wordmark only: no logo is invented until Gustavo approves visual assets (web-v1.md §9).
export function Wordmark({ className = "" }: { className?: string }) {
  return (
    <span className={`font-mono text-sm font-medium tracking-[0.18em] ${className}`}>
      BRAMBILAB<span aria-hidden="true" className="text-accent">_</span>
    </span>
  );
}
