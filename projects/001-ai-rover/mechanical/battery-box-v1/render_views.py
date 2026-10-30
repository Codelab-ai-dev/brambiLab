"""Vistas PNG de la caja de batería (sin interfaz).

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

NAME = "ROVER_Battery_Sensor_Enclosure"
COL = {"base": (0.95, 0.55, 0.10), "lid": (0.20, 0.55, 0.85), "bat": (0.15, 0.6, 0.3),
       "lidar": (0.85, 0.2, 0.2), "unoq": (0.1, 0.45, 0.75)}


def tris(shape, tol=0.05):
    v, f = shape.tessellate(tol)
    v = np.array([(p.x, p.y, p.z) for p in v])
    return v[np.array(f)]


def draw(path, items, elev, azim, title, size=(10, 8)):
    fig = plt.figure(figsize=size, dpi=150)
    ax = fig.add_subplot(111, projection="3d", proj_type="ortho")
    ax.view_init(elev=elev, azim=azim)
    e, a = np.radians(elev + 25), np.radians(azim - 30)
    light = np.array([np.cos(e) * np.cos(a), np.cos(e) * np.sin(a), np.sin(e)])
    allv, allT, allC = [], [], []
    for T, col, alpha in items:
        n = np.cross(T[:, 1] - T[:, 0], T[:, 2] - T[:, 0])
        n /= np.linalg.norm(n, axis=1)[:, None] + 1e-12
        k = 0.35 + 0.65 * np.abs(n @ light)
        allC.append(np.hstack([np.clip(np.array(col)[None, :] * k[:, None], 0, 1), np.full((len(T), 1), alpha)]))
        allT.append(T); allv.append(T.reshape(-1, 3))
    # una sola colección: matplotlib ordena por cara y no por pieza
    ax.add_collection3d(Poly3DCollection(np.vstack(allT), facecolors=np.vstack(allC), edgecolors="none", linewidths=0))
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
    doc = App.openDocument(os.path.join(d, NAME + ".FCStd"))
    doc.recompute()
    B, Lh, BAT = doc.getObject("Base").Shape, doc.getObject("Lid").Shape, doc.getObject("Battery_Ref").Shape
    LZ, UQ = doc.getObject("Zone_LiDAR").Shape, doc.getObject("Zone_UNOQ").Shape
    b, l, bat = tris(B), tris(Lh), tris(BAT)
    zones = [(tris(LZ), COL["lidar"], 0.35), (tris(UQ), COL["unoq"], 0.25)]
    draw(os.path.join(rd, "iso_cerrada.png"), [(b, COL["base"], 1), (l, COL["lid"], 1)] + zones, 25, -55,
         "Isométrica cerrada — zonas conceptuales: LiDAR 50x50 (rojo), UNO Q (azul)")
    draw(os.path.join(rd, "iso_abierta.png"), [(b, COL["base"], 1), (bat, COL["bat"], 0.45)], 40, -55,
         "Isométrica abierta (sin tapa) con el pack 75x67x20")
    up = Lh.translated(V(0, 0, 35)); bup = BAT.translated(V(0, 0, 0))
    draw(os.path.join(rd, "explosionada.png"), [(b, COL["base"], 1), (tris(bup), COL["bat"], 0.45),
                                                (tris(up), COL["lid"], 0.9)], 18, -55, "Vista explosionada: base · pack · tapa")
    draw(os.path.join(rd, "base_superior.png"), [(b, COL["base"], 1)], 90, -90, "Base — vista superior")
    draw(os.path.join(rd, "tapa_inferior.png"), [(l, COL["lid"], 1)], -50, -60,
         "Tapa — vista inferior: labio y nervaduras sólo sobre la zona de cables")
    draw(os.path.join(rd, "frontal.png"), [(b, COL["base"], 1), (l, COL["lid"], 1)], 0, -90, "Vista frontal (desde -Y): ventilación")
    draw(os.path.join(rd, "lateral.png"), [(b, COL["base"], 1), (l, COL["lid"], 1)], 0, 0, "Vista lateral (desde +X): salida de cable")
    W = B.BoundBox.YLength
    half = Part.makeBox(200, W, 100, V(-10, W / 2, -10))
    sec = [(tris(B.cut(half)), COL["base"], 1), (tris(Lh.cut(half)), COL["lid"], 1), (tris(BAT.cut(half)), COL["bat"], 0.6)]
    draw(os.path.join(rd, "seccion.png"), sec, 15, 75, "Sección por Y = %.0f: pack, anclaje de brida y salida" % (W / 2))
    draw(os.path.join(rd, "seccion_torre.png"), [(tris(B.common(Part.makeBox(30, 6, 40, V(0, 0, -1)))), COL["base"], 1)],
         10, 70, "Sección por Y = 6: torre de esquina (inserto, piloto, ranura de tuerca, sólido para M3 al deck)", size=(8, 7))


ejecutar(main)
