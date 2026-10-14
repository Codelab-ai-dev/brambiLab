#pragma once

// Interfaz de conduccion. Se sirve desde el puerto 80; el video viene del 81
// para que el stream no bloquee los comandos (esp_http_server atiende una
// peticion por servidor a la vez).
static const char INDEX_HTML[] PROGMEM = R"rawliteral(<!DOCTYPE html>
<html lang="es">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width,initial-scale=1,user-scalable=no">
<title>Rover</title>
<style>
*{box-sizing:border-box;-webkit-tap-highlight-color:transparent}
body{margin:0;background:#111418;color:#e8eaed;font:15px/1.4 system-ui,-apple-system,sans-serif;
     display:flex;flex-direction:column;align-items:center;gap:12px;padding:12px;min-height:100vh}
#cam{width:100%;max-width:640px;aspect-ratio:4/3;background:#000;border-radius:10px;object-fit:cover}
.row{display:flex;gap:10px;align-items:center;width:100%;max-width:640px;font-size:13px}
#dot{width:10px;height:10px;border-radius:50%;background:#d33;flex:none}
#dot.ok{background:#2d2}
#pad{display:grid;grid-template-columns:repeat(3,86px);grid-template-rows:repeat(3,86px);gap:10px}
button{background:#272c33;color:#e8eaed;border:1px solid #3a4049;border-radius:12px;
       font-size:26px;touch-action:none;user-select:none;cursor:pointer}
button:active,button.on{background:#2f6fd0;border-color:#4a86e8}
#f{grid-area:1/2}#l{grid-area:2/1}#s{grid-area:2/2;font-size:15px}#r{grid-area:2/3}#b{grid-area:3/2}
#spd{width:100%;max-width:640px;accent-color:#2f6fd0}
small{color:#9aa3ad}
</style>
</head>
<body>
<img id="cam" alt="camara">
<div class="row"><span id="dot"></span><span id="st">conectando</span>
  <span style="margin-left:auto" id="val">0 / 0</span></div>
<label class="row"><small>velocidad&nbsp;</small><input id="spd" type="range" min="80" max="255" value="190"></label>
<div id="pad">
  <button id="f" data-k="f">&#9650;</button>
  <button id="l" data-k="l">&#9664;</button>
  <button id="s">STOP</button>
  <button id="r" data-k="r">&#9654;</button>
  <button id="b" data-k="b">&#9660;</button>
</div>
<small>teclado: W A S D o flechas &middot; soltar = parar</small>

<script>
const held = {f:false, b:false, l:false, r:false};
const $ = id => document.getElementById(id);
let busy = false, lastSent = "";

$("cam").src = "http://" + location.hostname + ":81/stream";

function mix() {
  const sp = +$("spd").value;
  const vx = (held.f ? 1 : 0) - (held.b ? 1 : 0);
  const wz = (held.r ? 1 : 0) - (held.l ? 1 : 0);
  // Girando en parado se usa giro completo; en marcha solo se corrige.
  const turn = vx === 0 ? 1 : 0.55;
  const cl = v => Math.max(-255, Math.min(255, Math.round(v)));
  return [cl((vx + wz * turn) * sp), cl((vx - wz * turn) * sp)];
}

// Un envio en vuelo como mucho: si la red se atasca no se acumula cola.
async function tick() {
  if (busy) return;
  const [l, r] = mix();
  busy = true;
  try {
    const t0 = performance.now();
    const res = await fetch(`/drive?l=${l}&r=${r}`, {cache: "no-store"});
    const ms = Math.round(performance.now() - t0);
    $("dot").className = res.ok ? "ok" : "";
    $("st").textContent = res.ok ? ms + " ms" : "error " + res.status;
    $("val").textContent = l + " / " + r;
  } catch (e) {
    $("dot").className = "";
    $("st").textContent = "sin enlace";
  } finally {
    busy = false;
  }
}
setInterval(tick, 150);

function set(k, on) {
  if (!(k in held) || held[k] === on) return;
  held[k] = on;
  document.querySelector(`[data-k="${k}"]`).classList.toggle("on", on);
  tick();
}

function stop() {
  for (const k in held) set(k, false);
  fetch("/stop", {cache: "no-store"});
}

for (const el of document.querySelectorAll("[data-k]")) {
  const k = el.dataset.k;
  el.addEventListener("pointerdown", e => { el.setPointerCapture(e.pointerId); set(k, true); });
  el.addEventListener("pointerup", () => set(k, false));
  el.addEventListener("pointercancel", () => set(k, false));
  el.addEventListener("contextmenu", e => e.preventDefault());
}
$("s").addEventListener("click", stop);

const KEYS = {w:"f", s:"b", a:"l", d:"r",
              arrowup:"f", arrowdown:"b", arrowleft:"l", arrowright:"r"};
addEventListener("keydown", e => { const k = KEYS[e.key.toLowerCase()]; if (k) { e.preventDefault(); set(k, true); } });
addEventListener("keyup",   e => { const k = KEYS[e.key.toLowerCase()]; if (k) { e.preventDefault(); set(k, false); } });

// Si la pestana se va a segundo plano el rover no se queda rodando.
addEventListener("blur", stop);
document.addEventListener("visibilitychange", () => { if (document.hidden) stop(); });
</script>
</body>
</html>)rawliteral";
