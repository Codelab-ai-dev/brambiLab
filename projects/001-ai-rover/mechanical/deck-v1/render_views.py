"""Vistas PNG (superior, inferior, lateral, isométrica) sin interfaz gráfica.

  fc.sh run render_views.py -- --dir out

Tesela las formas del modelo y las pinta con matplotlib en proyección
ortográfica con sombreado simple. Salida en <dir>/renders/.
"""
import sys, os
sys.path.insert(0, os.path.expanduser("~/.claude/skills/freecad/scripts"))
from fchelpers import *  # noqa: F401,F403
import numpy as np
import matplotlib
matplotlib.use("Agg")
import matplotlib.pyplot as plt
from mpl_toolkits.mplot3d.art3d import Poly3DCollection

COL = {"deck": (0.95, 0.55, 0.10), "support": (0.30, 0.30, 0.34),
       "chassis": (0.78, 0.84, 0.90), "unoq": (0.15, 0.45, 0.85)}


def tris(shape, tol=0.08):
    v, f = shape.tessellate(tol)
    v = np.array([(p.x, p.y, p.z) for p in v])
    return v[np.array(f)]


def draw(path, items, elev, azim, title, size=(12, 7.5)):
    fig = plt.figure(figsize=size, dpi=150)
    ax = fig.add_subplot(111, projection="3d", proj_type="ortho")
    ax.view_init(elev=elev, azim=azim)
    # dirección de la luz: desde la cámara, algo elevada
    e, a = np.radians(elev + 25), np.radians(azim - 30)
    light = np.array([np.cos(e) * np.cos(a), np.cos(e) * np.sin(a), np.sin(e)])
    allv = []
    for T, col, alpha in items:
        n = np.cross(T[:, 1] - T[:, 0], T[:, 2] - T[:, 0])
        n /= np.linalg.norm(n, axis=1)[:, None] + 1e-12
        k = 0.35 + 0.65 * np.abs(n @ light)
        fc = np.clip(np.array(col)[None, :] * k[:, None], 0, 1)
        fc = np.hstack([fc, np.full((len(fc), 1), alpha)])
        pc = Poly3DCollection(T, facecolors=fc, edgecolors="none", linewidths=0)
        ax.add_collection3d(pc)
        allv.append(T.reshape(-1, 3))
    V_ = np.vstack(allv)
    lo, hi = V_.min(0), V_.max(0)
    c, r = (lo + hi) / 2, (hi - lo).max() / 2
    ax.set_xlim(c[0] - r, c[0] + r); ax.set_ylim(c[1] - r, c[1] + r); ax.set_zlim(c[2] - r, c[2] + r)
    ax.set_box_aspect((1, 1, 1))
    ax.set_axis_off()
    ax.set_title(title, fontsize=11)
    fig.subplots_adjust(0, 0, 1, 0.95)
    fig.savefig(path, facecolor="white")
    plt.close(fig)
    print("png", os.path.basename(path), os.path.getsize(path) // 1024, "KB")


def main():
    args = argv_script()
    d = os.path.abspath(args[args.index("--dir") + 1])
    rd = os.path.join(d, "renders"); os.makedirs(rd, exist_ok=True)
    doc = App.openDocument(os.path.join(d, "rover_deck_v1.FCStd"))
    doc.recompute()
    deck = tris(doc.getObject("Deck").Shape)
    sup_shape = doc.getObject("Support").Shape
    sup = tris(sup_shape, 0.03)
    placed = []
    for o in doc.Objects:
        if o.TypeId == "App::Link":
            s = sup_shape.copy(); s.Placement = o.Placement
            placed.append(tris(s, 0.05))
    supp_all = np.vstack(placed)
    chassis = tris(doc.getObject("ChassisRef").Shape)
    unoq = tris(doc.getObject("UnoQ_Env").Shape)

    D = [(deck, COL["deck"], 1.0)]
    A = D + [(supp_all, COL["support"], 1.0), (chassis, COL["chassis"], 1.0), (unoq, COL["unoq"], 0.25)]
    draw(os.path.join(rd, "deck_superior.png"), D, 90, -90, "Piso superior — vista superior (frente a la izquierda)")
    draw(os.path.join(rd, "deck_inferior.png"), D, -90, -90, "Piso superior — vista inferior (nervaduras, alojamientos, tuercas M3)")
    draw(os.path.join(rd, "deck_isometrica.png"), D, 30, -60, "Piso superior — isométrica")
    draw(os.path.join(rd, "ensamblaje_lateral.png"), A, 0, -90, "Ensamblaje — vista lateral (chasis, soportes, piso, envolvente UNO Q)")
    draw(os.path.join(rd, "ensamblaje_isometrica.png"), A, 25, -55, "Ensamblaje — isométrica")
    draw(os.path.join(rd, "deck_isometrica_inferior.png"), D, -35, -60, "Piso superior — isométrica inferior")
    draw(os.path.join(rd, "soporte_isometrica.png"), [(sup, COL["support"], 1.0)], 20, -50,
         "Soporte (x4) — isométrica: guía cuadrada y piloto M2.5 abajo", size=(7, 7))


ejecutar(main)
