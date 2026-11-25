"""Guarda el estado de visualización (GuiDocument.xml) del .FCStd: sólo piezas finales,
colores y vista isométrica. Requiere modo app:  fc.sh gui fix_display.py -- --file out/X.FCStd"""
import sys, os, traceback
sys.path.insert(0, os.path.expanduser("~/.claude/skills/freecad/scripts"))
from fchelpers import argv_script, ejecutar
import FreeCAD as App
import FreeCADGui as Gui

SHOW = {"Base": ((0.95, 0.55, 0.10), 0), "Lid": ((0.20, 0.55, 0.85), 40), "Battery_Ref": ((0.15, 0.60, 0.30), 60),
        "Zone_LiDAR": ((0.85, 0.20, 0.20), 50), "Zone_UNOQ": ((0.10, 0.45, 0.75), 70)}


def main():
    args = argv_script()
    f = os.path.abspath(args[args.index("--file") + 1])
    log = open(f + ".display.log", "w")
    try:
        doc = App.openDocument(f)
        doc.recompute()
        Gui.updateGui()
        gd = Gui.getDocument(doc.Name)
        for o in doc.Objects:
            vo = gd.getObject(o.Name)
            if vo is None or not hasattr(vo, "Visibility"):
                continue
            show = o.Name in SHOW
            if o.TypeId.startswith("PartDesign::") and o.TypeId != "PartDesign::Body":
                body = o.getParentGeoFeatureGroup()
                show = body is not None and body.Name in SHOW and body.Tip == o
            vo.Visibility = show
            if o.Name in SHOW and hasattr(vo, "ShapeColor"):
                vo.ShapeColor = SHOW[o.Name][0]
                vo.Transparency = SHOW[o.Name][1]
        Gui.updateGui()
        v = gd.activeView()
        v.viewIsometric(); v.fitAll()
        Gui.updateGui()
        doc.save()
        log.write("OK\n")
        App.closeDocument(doc.Name)
    except Exception:
        log.write(traceback.format_exc())
    finally:
        log.close()
        try:
            Gui.getMainWindow().close()
        except Exception:
            pass


ejecutar(main)
