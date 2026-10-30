"""Exporta a PDF las hojas generadas por make_drawing.py. Requiere modo app:

  fc.sh gui export_pdf.py -- --dir out
"""
import sys, os, glob, traceback
sys.path.insert(0, os.path.expanduser("~/.claude/skills/freecad/scripts"))
from fchelpers import argv_script, ejecutar
import FreeCAD as App
import FreeCADGui as Gui


def main():
    args = argv_script()
    d = os.path.abspath(args[args.index("--dir") + 1])
    log = open(os.path.join(d, "export_pdf.log"), "w")
    try:
        import TechDrawGui
        for f in sorted(glob.glob(os.path.join(d, "rover_deck_v1_plano_*.FCStd"))):
            doc = App.openDocument(f)
            doc.recompute()
            Gui.updateGui()
            page = next(o for o in doc.Objects if o.TypeId == "TechDraw::DrawPage")
            pdf = os.path.splitext(f)[0] + ".pdf"
            TechDrawGui.exportPageAsPdf(page, pdf)
            log.write("pdf %s %d\n" % (os.path.basename(pdf), os.path.getsize(pdf)))
            App.closeDocument(doc.Name)
        log.write("OK\n")
    except Exception:
        log.write(traceback.format_exc())
    finally:
        log.close()
        try:
            Gui.getMainWindow().close()
        except Exception:
            pass


ejecutar(main)
