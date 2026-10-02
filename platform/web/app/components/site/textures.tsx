// Original technical illustrations for the public site (WEB-009). They are conceptual — no
// telemetry, coordinates or project photos are implied — static (no animation loop) and
// decorative (aria-hidden). Generated from a fixed seed with integer coordinates, so the server
// and the browser always produce the same markup.

type Props = { className?: string };

/** mulberry32: small deterministic PRNG. */
function rng(seed: number) {
  return () => {
    seed |= 0;
    seed = (seed + 0x6d2b79f5) | 0;
    let t = Math.imul(seed ^ (seed >>> 15), 1 | seed);
    t = (t + Math.imul(t ^ (t >>> 7), 61 | t)) ^ t;
    return ((t ^ (t >>> 14)) >>> 0) / 4294967296;
  };
}

const r = Math.round;

// Point cloud: a warped surface seen in perspective, drawn as one path of round dots.
const CLOUD = (() => {
  const rand = rng(11);
  let d = "";
  let lit = "";
  for (let gy = 0; gy < 16; gy++) {
    for (let gx = 0; gx < 34; gx++) {
      const u = gx / 33 - 0.5;
      const v = gy / 15;
      const z = Math.exp(-((u * 2.4) ** 2)) * 70 * (0.4 + v * 0.6) + Math.sin(u * 12 + v * 5) * 6;
      const depth = 0.55 + v * 0.45;
      const x = 240 + u * 440 * depth + (rand() - 0.5) * 4;
      const y = 90 + v * 170 - z * depth + (rand() - 0.5) * 4;
      const dot = `M${r(x)} ${r(y)}h0`;
      if (Math.abs(u) < 0.08 && v > 0.5) lit += dot;
      else d += dot;
    }
  }
  return { d, lit };
})();

export function PointCloud({ className = "" }: Props) {
  return (
    <svg viewBox="0 0 480 300" aria-hidden="true" focusable="false" className={className}>
      <g stroke="rgb(139 152 168 / 0.35)" strokeWidth="1">
        <path d="M240 20v260M40 262h400" strokeDasharray="2 6" />
      </g>
      <path d={CLOUD.d} stroke="rgb(255 255 255 / 0.55)" strokeWidth="2.4" strokeLinecap="round" />
      <path d={CLOUD.lit} stroke="var(--color-signal)" strokeWidth="2.8" strokeLinecap="round" />
    </svg>
  );
}

// Connected nodes: each links to its two nearest neighbours; one gateway is lit.
const NET = (() => {
  const rand = rng(5);
  const nodes = Array.from({ length: 15 }, () => ({ x: r(30 + rand() * 420), y: r(30 + rand() * 240) }));
  const edges = new Set<string>();
  nodes.forEach((a, i) => {
    nodes
      .map((b, j) => ({ j, dist: (a.x - b.x) ** 2 + (a.y - b.y) ** 2 }))
      .filter((n) => n.j !== i)
      .sort((p, q) => p.dist - q.dist)
      .slice(0, 2)
      .forEach(({ j }) => edges.add(i < j ? `${i}-${j}` : `${j}-${i}`));
  });
  const d = [...edges].map((e) => {
    const [i, j] = e.split("-").map(Number);
    return `M${nodes[i].x} ${nodes[i].y}L${nodes[j].x} ${nodes[j].y}`;
  });
  return { nodes, d: d.join("") };
})();

export function NodeNetwork({ className = "" }: Props) {
  const hub = NET.nodes[0];
  return (
    <svg viewBox="0 0 480 300" aria-hidden="true" focusable="false" className={className}>
      <path d={NET.d} stroke="rgb(139 152 168 / 0.5)" strokeWidth="1" />
      {NET.nodes.map((n, i) => (
        <rect key={i} x={n.x - 3} y={n.y - 3} width="6" height="6" fill="var(--color-surface)" stroke="currentColor" strokeWidth="1.2" />
      ))}
      <g fill="none" stroke="var(--color-signal)">
        <rect x={hub.x - 5} y={hub.y - 5} width="10" height="10" fill="var(--color-signal)" />
        <circle cx={hub.x} cy={hub.y} r="18" strokeWidth="1" />
        <circle cx={hub.x} cy={hub.y} r="34" strokeWidth="1" opacity="0.5" />
      </g>
    </svg>
  );
}

// Trajectory: a planned route (dashed) and the driven one (solid) over a survey grid.
export function Trajectory({ className = "" }: Props) {
  const grid: string[] = [];
  for (let x = 40; x <= 440; x += 40) grid.push(`M${x} 20v260`);
  for (let y = 20; y <= 280; y += 40) grid.push(`M40 ${y}h400`);
  const waypoints = [
    [60, 250],
    [150, 190],
    [230, 210],
    [320, 110],
    [420, 60],
  ];
  return (
    <svg viewBox="0 0 480 300" aria-hidden="true" focusable="false" className={className}>
      <path d={grid.join("")} stroke="rgb(139 152 168 / 0.22)" strokeWidth="1" />
      <path d="M60 250C100 220 120 190 150 190S200 215 230 210 290 130 320 110 390 70 420 60" fill="none" stroke="rgb(255 255 255 / 0.5)" strokeWidth="1.5" strokeDasharray="4 6" />
      <path d="M60 250C96 224 118 196 152 194S204 222 232 214 284 142 316 118" fill="none" stroke="var(--color-signal)" strokeWidth="2" />
      {waypoints.map(([x, y], i) => (
        <rect key={i} x={x - 4} y={y - 4} width="8" height="8" fill="var(--color-surface)" stroke={i < 4 ? "var(--color-signal)" : "currentColor"} strokeWidth="1.5" />
      ))}
      <path d="M316 118l-12 2 6 10z" fill="var(--color-signal)" />
    </svg>
  );
}
