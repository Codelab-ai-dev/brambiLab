// Three.js scene of the hero (#53), loaded on demand in the browser only (React.lazy from
// HeroVisual): never evaluated during SSR and never downloaded on other routes or where the static
// image is used. One canvas, one WebGL context, three draw calls (points, cyan lines), no
// post-processing, shadows or textures; device pixel ratio capped at 1.5.
// - The loop runs only while `playing`; otherwise the frame loop is "demand" (no continuous GPU
//   work, the last frame stays on screen).
// - The canvas ignores pointer events: scroll, touch and clicks go to the page. A window-level
//   pointer listener (passive, no React state) gives a slight, eased tilt towards the cursor.
// - Geometries and materials are declared in JSX, so React Three Fiber disposes them on unmount,
//   together with the renderer.
import { Canvas, useFrame } from "@react-three/fiber";
import { useEffect, useRef } from "react";
import type { Group } from "three";
import { BASE_ROTATION, CAMERA, CLOUD } from "./perception";

// Point colours: a cool grey-white, brighter near the sensor (vertex colours, one material).
const COLORS = (() => {
  const c = new Float32Array(CLOUD.light.length * 3);
  for (let i = 0; i < CLOUD.light.length; i++) {
    const l = CLOUD.light[i];
    c[i * 3] = 0.55 + 0.3 * l;
    c[i * 3 + 1] = 0.62 + 0.3 * l;
    c[i * 3 + 2] = 0.72 + 0.28 * l;
  }
  return c;
})();

function Cloud({ onFirstFrame }: { onFirstFrame: () => void }) {
  const group = useRef<Group>(null);
  const pointer = useRef({ x: 0, y: 0 });
  const first = useRef(true);
  useEffect(() => {
    const onMove = (e: PointerEvent) => {
      pointer.current.x = (e.clientX / window.innerWidth) * 2 - 1;
      pointer.current.y = (e.clientY / window.innerHeight) * 2 - 1;
    };
    window.addEventListener("pointermove", onMove, { passive: true });
    return () => window.removeEventListener("pointermove", onMove);
  }, []);
  useFrame((_, delta) => {
    const g = group.current;
    if (!g) return;
    const dt = Math.min(delta, 0.1); // no jump after a pause
    g.rotation.y += dt * 0.05; // slow turn: about two minutes per revolution
    // Limited, eased response to the cursor.
    g.rotation.x += (pointer.current.y * 0.06 - g.rotation.x) * Math.min(1, dt * 2);
    g.position.x += (pointer.current.x * 0.12 - g.position.x) * Math.min(1, dt * 2);
    if (first.current) {
      first.current = false;
      onFirstFrame();
    }
  });
  return (
    <group ref={group} rotation={[0, BASE_ROTATION, 0]}>
      <points>
        <bufferGeometry>
          <bufferAttribute attach="attributes-position" args={[CLOUD.points, 3]} />
          <bufferAttribute attach="attributes-color" args={[COLORS, 3]} />
        </bufferGeometry>
        <pointsMaterial size={0.022} sizeAttenuation vertexColors transparent opacity={0.85} depthWrite={false} />
      </points>
      <lineSegments>
        <bufferGeometry>
          <bufferAttribute attach="attributes-position" args={[CLOUD.lines, 3]} />
        </bufferGeometry>
        <lineBasicMaterial color="#1ec8ff" transparent opacity={0.6} depthWrite={false} />
      </lineSegments>
    </group>
  );
}

export default function HeroScene({ playing, onReady, onLost, className = "" }: { playing: boolean; onReady: () => void; onLost: () => void; className?: string }) {
  return (
    <div aria-hidden="true" className={className} style={{ pointerEvents: "none" }}>
      <Canvas
        dpr={[1, 1.5]}
        frameloop={playing ? "always" : "demand"}
        gl={{ antialias: true, alpha: true, powerPreference: "low-power" }}
        camera={{ position: CAMERA.position, fov: CAMERA.fov, near: 0.1, far: 30 }}
        onCreated={({ camera, gl }) => {
          camera.lookAt(...CAMERA.target);
          gl.domElement.addEventListener("webglcontextlost", (e) => {
            e.preventDefault();
            onLost();
          });
        }}
        style={{ pointerEvents: "none" }}
        tabIndex={-1}
      >
        <Cloud onFirstFrame={onReady} />
      </Canvas>
    </div>
  );
}
