// Procedural "perception" cloud for the home hero (#53): what a rotating range sensor would sweep
// in a small space — ground rings, a crate, a pillar and a wall — plus the sensor's own rings and
// field of view in cyan. Conceptual: no real scan, coordinates or telemetry. Pure, deterministic
// math (no browser or WebGL APIs), shared by the server-rendered static image and the 3D scene,
// so both show the same composition.

export type Vec3 = [number, number, number];

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

/** Sensor origin, slightly above the ground. */
export const SENSOR: Vec3 = [0, 0.35, 0];

export type Cloud = {
  /** xyz triplets. */
  points: Float32Array;
  /** 0–1 brightness per point (falls off with distance from the sensor). */
  light: Float32Array;
  /** Pairs of endpoints (xyz xyz) for the cyan line work. */
  lines: Float32Array;
};

function build(): Cloud {
  const rand = rng(53);
  const pts: number[] = [];
  const add = (x: number, y: number, z: number) => pts.push(x, y, z);

  // Ground: beam rings of a rotating sensor, denser near it, with a little jitter.
  for (let ring = 0; ring < 16; ring++) {
    const r = 0.45 + ring * 0.19;
    const n = Math.round(150 - ring * 5);
    for (let i = 0; i < n; i++) {
      const a = (i / n) * Math.PI * 2 + ring * 0.13;
      const jr = r + (rand() - 0.5) * 0.04;
      add(Math.cos(a) * jr, (rand() - 0.5) * 0.015, Math.sin(a) * jr);
    }
  }
  // Crate: points on the four faces the sensor sees, a regular raster like returns on a surface.
  const box = { x: 1.15, z: 0.55, w: 0.7, d: 0.55, h: 0.6 };
  for (let u = 0; u <= 1; u += 0.06) {
    for (let v = 0; v <= 1; v += 0.07) {
      add(box.x - box.w / 2 + u * box.w, v * box.h, box.z - box.d / 2); // front
      add(box.x - box.w / 2, v * box.h, box.z - box.d / 2 + u * box.d); // side facing the sensor
      add(box.x - box.w / 2 + u * box.w, box.h, box.z - box.d / 2 + v * box.d); // top
    }
  }
  // Pillar: a cylinder sampled by scan lines.
  for (let y = 0; y <= 1.5; y += 0.06) {
    for (let a = 0; a < Math.PI * 2; a += Math.PI / 14) add(-1.05 + Math.cos(a) * 0.22, y, -0.7 + Math.sin(a) * 0.22);
  }
  // Wall: a low panel at the back with an opening.
  for (let x = -2.2; x <= 2.2; x += 0.07) {
    for (let y = 0; y <= 0.9; y += 0.08) {
      if (x > -0.2 && x < 0.45 && y < 0.6) continue;
      add(x, y, -1.9 + (rand() - 0.5) * 0.02);
    }
  }

  const points = new Float32Array(pts);
  const light = new Float32Array(points.length / 3);
  for (let i = 0; i < light.length; i++) {
    const dx = points[i * 3] - SENSOR[0];
    const dz = points[i * 3 + 2] - SENSOR[2];
    light[i] = Math.max(0.25, 1 - Math.hypot(dx, dz) / 3.4);
  }

  // Cyan line work: three sweep rings on the ground and the sensor's field of view.
  const ln: number[] = [];
  for (const r of [0.8, 1.6, 2.4]) {
    const n = 72;
    for (let i = 0; i < n; i++) {
      const a0 = (i / n) * Math.PI * 2;
      const a1 = ((i + 1) / n) * Math.PI * 2;
      ln.push(Math.cos(a0) * r, 0.002, Math.sin(a0) * r, Math.cos(a1) * r, 0.002, Math.sin(a1) * r);
    }
  }
  const [sx, sy, sz] = SENSOR;
  const far: Vec3[] = [
    [1.9, 0.05, -1.2],
    [1.9, 1.1, -1.2],
    [0.6, 1.1, -1.9],
    [0.6, 0.05, -1.9],
  ];
  for (let i = 0; i < 4; i++) {
    const a = far[i];
    const b = far[(i + 1) % 4];
    ln.push(sx, sy, sz, ...a, ...a, ...b);
  }
  // Sensor marker: a small cross.
  ln.push(sx - 0.12, sy, sz, sx + 0.12, sy, sz, sx, sy - 0.12, sz, sx, sy + 0.12, sz, sx, sy, sz - 0.12, sx, sy, sz + 0.12);

  return { points, light, lines: new Float32Array(ln) };
}

export const CLOUD = build();

/** Camera shared by the scene and the static image. */
export const CAMERA = { position: [0.2, 2.2, 5.6] as Vec3, target: [0.1, 0.3, 0] as Vec3, fov: 34 };
/** Slight initial turn of the cloud so the crate and the field of view read well. */
export const BASE_ROTATION = -0.5;

/**
 * Perspective projection of the cloud for the static image: same camera and initial rotation as
 * the scene, into an SVG viewBox of the given size (vertical field of view, like Three.js).
 */
export function project(width: number, height: number) {
  const [cx, cy, cz] = CAMERA.position;
  const [tx, ty, tz] = CAMERA.target;
  // Camera basis: forward f, right r, up u.
  let fx = tx - cx, fy = ty - cy, fz = tz - cz;
  const fl = Math.hypot(fx, fy, fz);
  fx /= fl; fy /= fl; fz /= fl;
  // r = normalize(f × worldUp), with worldUp = (0, 1, 0).
  const rl = Math.hypot(fz, fx);
  const rx = -fz / rl, ry = 0, rz = fx / rl;
  const ux = ry * fz - rz * fy, uy = rz * fx - rx * fz, uz = rx * fy - ry * fx;
  const focal = height / 2 / Math.tan((CAMERA.fov * Math.PI) / 360);
  const cos = Math.cos(BASE_ROTATION), sin = Math.sin(BASE_ROTATION);
  const toScreen = (x: number, y: number, z: number): [number, number, number] | null => {
    // Rotate around Y (the scene's group rotation), then into camera space.
    const wx = x * cos + z * sin, wz = -x * sin + z * cos;
    const px = wx - cx, py = y - cy, pz = wz - cz;
    const depth = px * fx + py * fy + pz * fz;
    if (depth <= 0.1) return null;
    const sx = (px * rx + py * ry + pz * rz) / depth;
    const sy = (px * ux + py * uy + pz * uz) / depth;
    return [width / 2 + sx * focal, height / 2 - sy * focal, depth];
  };
  return { toScreen };
}
