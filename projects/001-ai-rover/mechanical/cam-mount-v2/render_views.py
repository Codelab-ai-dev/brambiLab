"""Vistas PNG del soporte de cámara (sin interfaz).

  fc.sh run render_views.py -- --dir out
"""
import sys, os
sys.path.insert(0, os.path.expanduser("~/.claude/skills/freecad/scripts"))
from fchelpers import *  # noqa: F401,F403
import numpy as np
import matplotlib
matplotlib.use("Agg")
import matplotlib.pyplot as plt
from mpl_toolkits.mplot3d.art3d import Poly3DCollection

COL = {"arm": (0.95, 0.55, 0.10), "carrier": (0.20, 0.55, 0.85), "module": (0.15, 0.6, 0.3),
       "deck": (0.85, 0.85, 0.85), "chassis": (0.78, 0.84, 0.90)}


def tris(shape, tol=0.05):
    v, f = shape.tessellate(tol)
    v = np.array([(p.x, p.y, p.z) for p in v])
    return v[np.array(f)]


def draw(path, items, elev, azim, title, size=(10, 8), clip=None):
    fig = plt.figure(figsize=size, dpi=150)
    ax = fig.add_subplot(111, projection="3d", proj_type="ortho")
    ax.view_init(elev=elev, azim=azim)
    e, a = np.radians(elev + 25), np.radians(azim - 30)
    light = np.array([np.cos(e) * np.cos(a), np.cos(e) * np.sin(a), np.sin(e)])
    allv = []
    for T, col, alpha, fit in items:
        n = np.cross(T[:, 1] - T[:, 0], T[:, 2] - T[:, 0])
        n /= np.linalg.norm(n, axis=1)[:, None] + 1e-12
        k = 0.35 + 0.65 * np.abs(n @ light)
        fc = np.hstack([np.clip(np.array(col)[None, :] * k[:, None], 0, 1), np.full((len(T), 1), alpha)])
        ax.add_collection3d(Poly3DCollection(T, facecolors=fc, edgecolors="none", linewidths=0))
        if fit:
            allv.append(T.reshape(-1, 3))
    V_ = np.vstack(allv)
    lo, hi = V_.min(0), V_.max(0)
    c, r = (lo + hi) / 2, (hi - lo).max() / 2 * 1.05
    ax.set_xlim(c[0] - r, c[0] + r); ax.set_ylim(c[1] - r, c[1] + r); ax.set_zlim(c[2] - r, c[2] + r)
    ax.set_box_aspect((1, 1, 1)); ax.set_axis_off(); ax.set_title(title, fontsize=11)
    fig.subplots_adjust(0, 0, 1, 0.95)
    fig.savefig(path, facecolor="white"); plt.close(fig)
    print("png", os.path.basename(path), os.path.getsize(path) // 1024, "KB")


def main():
    args = argv_script()
    d = os.path.abspath(args[args.index("--dir") + 1])
    rd = os.path.join(d, "renders"); os.makedirs(rd, exist_ok=True)
    doc = App.openDocument(os.path.join(d, "ESP32_CAM_Open_Frame.FCStd"))
    doc.recompute()
    tilt = doc.getObject("CameraTilt")
    def placed(obj, lk, dx=0.0):
        s = obj.Shape.copy(); s.Placement = tilt.Placement.multiply(lk.Placement)
        s.translate(V(-dx, 0, 0)); return s
    arm = doc.getObject("Arm").Shape
    car = placed(doc.getObject("OpenFrame"), doc.getObject("Frame_Placed"))
    mod = placed(doc.getObject("CamModule_Env"), doc.getObject("CamModule_Placed"))
    deck = doc.getObject("ComputeDeck").Shape
    ch = doc.getObject("ChassisRef").Shape
    # recorte del piso y chasis a la zona frontal para que el soporte se vea grande
    box = Part.makeBox(60, 90, 80, V(-5, 0, -20))
    deck_c, ch_c = deck.common(box), ch.common(box)
    A, C, M = tris(arm), tris(car), tris(mod)
    ctx = [(tris(deck_c), COL["deck"], 1.0, False), (tris(ch_c), COL["chassis"], 1.0, False)]
    full = [(A, COL["arm"], 1.0, True), (C, COL["carrier"], 1.0, True), (M, COL["module"], 0.25, True)] + ctx
    draw(os.path.join(rd, "isometrica.png"), full, 22, -140, "Open frame ESP32-CAM — isométrica (piso y chasis recortados)")
    draw(os.path.join(rd, "frontal.png"), full, 0, 180, "Vista frontal (desde delante del rover)")
    draw(os.path.join(rd, "lateral.png"), full, 0, -90, "Vista lateral — inclinación 10° hacia abajo")
    draw(os.path.join(rd, "superior.png"), full, 90, -90, "Vista superior")
    draw(os.path.join(rd, "trasera.png"), [(A, COL["arm"], 1.0, True), (C, COL["carrier"], 1.0, True), (M, COL["module"], 0.25, True)],
         0, 0, "Vista trasera (sin piso)")
    ex = [(A, COL["arm"], 1.0, True),
          (tris(placed(doc.getObject("OpenFrame"), doc.getObject("Frame_Placed"), 25)), COL["carrier"], 1.0, True),
          (tris(placed(doc.getObject("CamModule_Env"), doc.getObject("CamModule_Placed"), 55)), COL["module"], 0.35, True)]
    draw(os.path.join(rd, "explosionada.png"), ex, 18, -130, "Vista explosionada: brazo (A) · frame abierto (B) · módulo ESP32-CAM")
    test = doc.getObject("FrameTest").Shape
    draw(os.path.join(rd, "pieza_prueba.png"), [(tris(test), COL["carrier"], 1.0, True)], 35, -60,
         "Frame de prueba V2", size=(7, 6))
    draw(os.path.join(rd, "frame_frontal.png"), [(tris(doc.getObject("OpenFrame").Shape), COL["carrier"], 1.0, True)], 90, -90,
         "Frame (B) de frente: columnas M2 y ranuras inferiores", size=(7, 7))

    # zonas de exclusión sobre el frame (coordenadas locales)
    ko = doc.getObject("Keepouts").Shape
    draw(os.path.join(rd, "zonas_exclusion.png"), [(tris(doc.getObject("OpenFrame").Shape), COL["carrier"], 1.0, True),
         (tris(ko), (0.9, 0.2, 0.2), 0.18, True), (tris(doc.getObject("CamModule_Env").Shape), COL["module"], 0.12, True)],
         28, -55, "Zonas de exclusión (rojo): pines, USB-C y laterales; verde: módulo", size=(8, 8))


ejecutar(main)
