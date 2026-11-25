"""Soporte abierto ESP32-CAM + placa de expansión (cam-mount v2, "open frame").

  ~/.claude/skills/freecad/scripts/fc.sh run build_open_frame.py -- --out <carpeta>

La placa queda SUSPENDIDA sobre 4 columnas M2 (2 agujeros arriba, 2 ranuras abajo).
Ninguna otra parte del frame entra en la huella de la placa a menos de standoff_height
de su cara trasera, ni en las zonas de exclusión (filas de pines, pines superiores e
inferiores, USB-C).

Piezas:
  Arm        A: base sobre el piso v1 (2 ranuras M3), muro, 2 brazos con 2 tornillos M3
             por lado (pivote + bloqueo: unión rígida en V1, base para un pivote en V2)
  OpenFrame  B: dos montantes laterales, esquinas, columnas M2, puente inferior por
             debajo de la placa y orejas laterales M3
  FrameTest  el mismo frame sin orejas, para validar el módulo
Referencias: ComputeDeck (STEP del piso v1), ChassisRef, CamModule_Env, Keepouts.

Coordenadas del rover = las del piso v1 (X=0 frente supuesto, la cámara mira a -X).
Coordenadas locales del frame: origen en la esquina inferior izquierda de la placa vista
de frente; x a la derecha, y arriba, z hacia la escena; z=0 es la cara trasera (cama).
"""
import sys, os, math, json
sys.path.insert(0, os.path.expanduser("~/.claude/skills/freecad/scripts"))
HERE = os.path.dirname(os.path.abspath(__file__))
sys.path.insert(0, os.path.join(HERE, "..", "common"))
from fchelpers import *  # noqa: F401,F403
from fcparam import E, build_params, sk, feat, try_fillet, SKETCHES
import Import

SHEET = "ROVER_ESP32_CAM_MOUNT"

PARAMS = [
    ("# Conjunto (medido con vernier)",),
    ("assembly_width", 36.05, "mm", None, "medido"),
    ("assembly_height", 41.89, "mm", None, "medido"),
    ("assembly_depth", 22.69, "mm", None, "medido; sándwich completo"),
    ("# Patrón M2 medido centro a centro (A sup-izq, B sup-der, C inf-izq, D inf-der)",),
    ("top_hole_spacing", 30.57, "mm", None, "A-B"),
    ("left_upper_to_lower", 34.53, "mm", None, "A-C"),
    ("right_upper_to_lower", 34.53, "mm", None, "B-D"),
    ("bottom_hole_spacing", 22.21, "mm", None, "C-D"),
    ("diagonal_A_D", 42.89, "mm", None, "A-D"),
    ("upper_hole_x_left", 2.74, "mm", None, "A, validado con test carrier"),
    ("upper_hole_x_right", None, "mm", "=upper_hole_x_left + top_hole_spacing", "B = 33.31"),
    ("upper_hole_y", 40.04, "mm", None, "A y B, validado"),
    ("ad_dx", None, "mm", "=(diagonal_A_D^2 - right_upper_to_lower^2 + top_hole_spacing^2) / (2 * top_hole_spacing)",
     "D desde A-D y B-D"),
    ("lower_hole_x_right", None, "mm", "=upper_hole_x_left + ad_dx", "D.x"),
    ("lower_hole_y_right", None, "mm", "=upper_hole_y - sqrt(diagonal_A_D^2 - ad_dx^2)", "D.y"),
    ("ad_ux", None, "", "=ad_dx / diagonal_A_D", "dirección A->D"),
    ("ad_uy", None, "", "=(lower_hole_y_right - upper_hole_y) / diagonal_A_D", ""),
    ("ac_a", None, "mm", "=(left_upper_to_lower^2 - bottom_hole_spacing^2 + diagonal_A_D^2) / (2 * diagonal_A_D)",
     "C desde A-C y C-D"),
    ("ac_h", None, "mm", "=sqrt(left_upper_to_lower^2 - ac_a^2)", ""),
    ("lower_hole_x_left", None, "mm", "=upper_hole_x_left + ac_a * ad_ux + ac_h * ad_uy", "C.x"),
    ("lower_hole_y_left", None, "mm", "=upper_hole_y + ac_a * ad_uy - ac_h * ad_ux", "C.y"),
    ("m2_clearance", 2.20, "mm", None, "paso M2 (agujeros A, B)"),
    ("lower_slot_width", 2.30, "mm", None, "ranuras horizontales C, D"),
    ("lower_slot_length", 5.00, "mm", None, "ajuste +-1.35 mm"),
    ("m2_nut_af", 4.2, "mm", None, "tuerca M2 4.0 + 0.2"),
    ("m2_nut_depth", 1.8, "mm", None, ""),
    ("lower_nut_channel", None, "mm", "=lower_slot_length + 2.6 mm", "canal para que la tuerca acompañe la ranura"),
    ("# Separadores (la placa queda suspendida)",),
    ("standoff_outer_diameter", 5.50, "mm", None, ""),
    ("standoff_height", 2.00, "mm", None, "cara del frame -> cara trasera de la placa (V5; antes 6.5); subir si algún pin toca"),
    ("standoff_fillet", 0.5, "mm", None, ""),
    ("# Zonas de exclusión (posiciones de pines SUPUESTAS: medir)",),
    ("pin_section_width", 25.44, "mm", None, "ancho exterior de las dos filas de headers"),
    ("pin_keepout", 3.00, "mm", None, ""),
    ("pin_row_center_x", None, "mm", "=assembly_width / 2", "SUPUESTO: filas centradas en la placa"),
    ("pin_row_y0", 10.0, "mm", None, "SUPUESTO: inicio vertical de las filas"),
    ("pin_row_y1", 34.0, "mm", None, "SUPUESTO: fin vertical de las filas"),
    ("pin_tail_length", 3.0, "mm", None, "SUPUESTO: cuánto sobresalen los pines por detrás"),
    ("top_pin_zone_height", 8.0, "mm", None, "SUPUESTO: pines superiores"),
    ("bottom_pin_zone_height", 12.0, "mm", None, "SUPUESTO: header inferior"),
    ("usb_keepout_width", 16.00, "mm", None, ""),
    ("usb_keepout_height", 12.00, "mm", None, "sobre el borde superior"),
    ("lateral_clearance", 3.00, "mm", None, ""),
    ("# Frame abierto (pieza B)",),
    ("frame_outer_width", 44.00, "mm", None, ""),
    ("frame_outer_height", 50.00, "mm", None, ""),
    ("frame_wall", 3.00, "mm", None, "puente inferior"),
    ("frame_depth", 6.00, "mm", None, "profundidad (z) de montantes y esquinas"),
    ("upright_inner_x", 0.0, "mm", None, "cara interior del montante izq.: 0 = fuera de la huella de la placa"),
    ("bridge_gap", 1.0, "mm", None, "puente inferior por debajo del borde de la placa"),
    ("ear_extension", 3.0, "mm", None, "orejas más allá del frame"),
    ("ear_depth", 11.0, "mm", None, ""),
    ("ear_margin", 5.0, "mm", None, ""),
    ("lock_distance", 9.0, "mm", None, "segundo M3 bajo el pivote: unión rígida V1"),
    ("m3_clearance", 3.40, "mm", None, ""),
    ("m3_nut_af", 5.7, "mm", None, ""),
    ("m3_nut_depth", 2.6, "mm", None, ""),
    ("corner_r", 2.0, "mm", None, ""),
    ("# Base y brazo (pieza A) sobre el piso v1",),
    ("deck_front_x", 7.5, "mm", None, "borde frontal del piso v1"),
    ("deck_top_z", 43.0, "mm", None, ""),
    ("deck_center_y", 45.0, "mm", None, ""),
    ("front_exp_x", 14.0, "mm", None, "puntos M3 frontales del piso v1"),
    ("deck_pass_x", 13.85, "mm", None, "paso de cable frontal del piso v1"),
    ("deck_pass_y", 51.5, "mm", None, ""),
    ("deck_mount_length", 55.00, "mm", None, "Y"),
    ("deck_mount_width", 24.00, "mm", None, "X"),
    ("deck_mount_thickness", 4.00, "mm", None, ""),
    ("mount_x_center", None, "mm", "=front_exp_x - 4 mm", "base X -2..22, delante de los M3 del LiDAR"),
    ("slot_x_center", None, "mm", "=front_exp_x - 2 mm", "ranuras descentradas para alcanzar X=14"),
    ("slot_width", 3.40, "mm", None, ""),
    ("slot_length", 10.00, "mm", None, ""),
    ("slot_spacing", 40.00, "mm", None, "= puntos frontales del piso"),
    ("base_pass_l", 7.0, "mm", None, ""),
    ("base_pass_w", 11.0, "mm", None, ""),
    ("arm_height", 35.00, "mm", None, "cara superior del piso -> pivote"),
    ("arm_offset", 16.50, "mm", None, "borde del piso -> pivote; encargo 15: con 15 el borde inferior del frame toca el chasis a 10°"),
    ("pivot_x", None, "mm", "=deck_front_x - arm_offset", ""),
    ("pivot_z", None, "mm", "=deck_top_z - arm_height", ""),
    ("arm_thickness", 5.00, "mm", None, ""),
    ("general_clearance", 0.30, "mm", None, ""),
    ("arm_center_offset", None, "mm", "=frame_outer_width / 2 + ear_extension + general_clearance + arm_thickness / 2", ""),
    ("arm_bottom_width", 10.0, "mm", None, ""),
    ("arm_top_front_x", -2.0, "mm", None, ""),
    ("arm_top_rear_x", None, "mm", "=deck_front_x - 1 mm", ""),
    ("pivot_boss_r", 6.0, "mm", None, ""),
    ("lock_boss_r", 5.0, "mm", None, ""),
    ("wall_drop", 7.0, "mm", None, ""),
    ("rib_thickness", 3.00, "mm", None, ""),
    ("rib_length", 10.0, "mm", None, ""),
    ("rib_height", 17.0, "mm", None, ""),
    ("minimum_wall", 2.40, "mm", None, ""),
    ("fillet_radius", 2.00, "mm", None, ""),
    ("tie_slot_width", 3.00, "mm", None, ""),
    ("tie_slot_length", 10.00, "mm", None, ""),
    ("tie_slot_x", -3.7, "mm", None, "dentro del brazo, delante de la nervadura"),
    ("tie_slot_z", 23.0, "mm", None, ""),
    ("tilt_angle", 10, "deg", None, "positivo = cámara hacia abajo"),
    ("lock_x", None, "mm", "=pivot_x + lock_distance * sin(tilt_angle)", "segundo M3 en el brazo"),
    ("lock_z", None, "mm", "=pivot_z - lock_distance * cos(tilt_angle)", ""),
    ("# Referencias",),
    ("chassis_length", 160, "mm", None, ""),
    ("chassis_width", 90, "mm", None, ""),
    ("chassis_thickness", 3, "mm", None, "SUPUESTO"),
]

M0_ROWS = [[0, 0, -1], [-1, 0, 0], [0, 1, 0]]   # local -> rover: X=-z, Y=-x, Z=y


def rot_from_rows(rows):
    m = App.Matrix()
    (m.A11, m.A12, m.A13), (m.A21, m.A22, m.A23), (m.A31, m.A32, m.A33) = rows
    return App.Placement(m).Rotation


def pts(P):
    return {"A": (P["upper_hole_x_left"], P["upper_hole_y"]), "B": (P["upper_hole_x_right"], P["upper_hole_y"]),
            "C": (P["lower_hole_x_left"], P["lower_hole_y_left"]), "D": (P["lower_hole_x_right"], P["lower_hole_y_right"])}


def build_frame(doc, P, name, ears=True):
    body = doc.addObject("PartDesign::Body", name)
    aw, ah = P["assembly_width"], P["assembly_height"]
    xo0, xo1 = aw / 2 - P["frame_outer_width"] / 2, aw / 2 + P["frame_outer_width"] / 2
    yo0, yo1 = ah / 2 - P["frame_outer_height"] / 2, ah / 2 + P["frame_outer_height"] / 2
    ui, uo = P["upright_inner_x"], aw - P["upright_inner_x"]
    r = P["standoff_outer_diameter"] / 2
    H = P["pts"] = pts(P)
    R = P["corner_r"]
    fd = P["frame_depth"]
    top = fd + P["standoff_height"]
    sl = P["lower_slot_length"] + P["standoff_outer_diameter"] - P["lower_slot_width"]   # columna inferior

    # montantes + esquinas (perfil izquierdo y derecho, cada uno en forma de C)
    s = sk(body, "Sk_Uprights")
    (ax, ay), (bx, by), (cx, cy), (dx, dy) = H["A"], H["B"], H["C"], H["D"]
    left = [(xo0, yo0), (cx, yo0), (cx, cy), (ui, cy), (ui, ay - r), (ax, ay - r), (ax, yo1), (xo0, yo1)]
    s.poly(left, [R, None, None, None, None, None, R, R])
    right = [(dx, yo0), (xo1, yo0), (xo1, yo1), (bx, yo1), (bx, by - r), (uo, by - r), (uo, dy), (dx, dy)]
    s.poly(right, [None, R, R, R, None, None, None, None])
    feat(body, "Pad", "Uprights", s, Length=fd)

    # puente inferior: por DEBAJO del borde de la placa, fuera de su huella
    s = sk(body, "Sk_Bridge")
    s.rect(xo0, yo0, xo1, -P["bridge_gap"], R)
    feat(body, "Pad", "BottomBridge", s, Length=P["frame_wall"])

    # columnas M2 desde la cama hasta la cara trasera de la placa
    s = sk(body, "Sk_UpperColumns")
    for k in ("A", "B"):
        s.circle(H[k][0], H[k][1], r)
    feat(body, "Pad", "UpperColumns", s, Length=top)
    s = sk(body, "Sk_LowerColumns")
    for k in ("C", "D"):
        s.rect_c(H[k][0], H[k][1], sl, P["standoff_outer_diameter"], r - 0.01)
    feat(body, "Pad", "LowerColumns", s, Length=top)

    if ears:
        py, pz = ah / 2, P["ear_depth"] / 2
        ly = py - P["lock_distance"]
        ey0, ey1 = ly - P["ear_margin"], py + P["ear_margin"]
        s = sk(body, "Sk_Ears")
        s.rect(xo0 - P["ear_extension"], ey0, ui, ey1, R)
        s.rect(uo, ey0, xo1 + P["ear_extension"], ey1, R)
        feat(body, "Pad", "Ears", s, Length=P["ear_depth"])
        s = sk(body, "Sk_EarHoles", plane="YZ_Plane", offset=aw / 2)
        s.circle(py, pz, P["m3_clearance"] / 2)
        s.circle(ly, pz, P["m3_clearance"] / 2)
        feat(body, "Pocket", "EarHoles", s, Type="ThroughAll", SideType="Symmetric")
        s = sk(body, "Sk_EarNutsL", plane="YZ_Plane", offset=ui)
        s.hexagon(py, pz, P["m3_nut_af"]); s.hexagon(ly, pz, P["m3_nut_af"])
        feat(body, "Pocket", "EarNutsL", s, Length=P["m3_nut_depth"])
        s = sk(body, "Sk_EarNutsR", plane="YZ_Plane", offset=uo)
        s.hexagon(py, pz, P["m3_nut_af"]); s.hexagon(ly, pz, P["m3_nut_af"])
        feat(body, "Pocket", "EarNutsR", s, Length=P["m3_nut_depth"], Reversed=True)

    # agujeros M2 arriba, ranuras horizontales abajo, tuercas cautivas detrás
    s = sk(body, "Sk_M2", offset=top)
    for k in ("A", "B"):
        s.circle(H[k][0], H[k][1], P["m2_clearance"] / 2)
    for k in ("C", "D"):
        s.rect_c(H[k][0], H[k][1], P["lower_slot_length"], P["lower_slot_width"], P["lower_slot_width"] / 2 - 0.05)
    feat(body, "Pocket", "M2Holes", s, Type="ThroughAll")
    s = sk(body, "Sk_M2Nuts")
    for k in ("A", "B"):
        s.hexagon(H[k][0], H[k][1], P["m2_nut_af"])
    for k in ("C", "D"):
        s.rect_c(H[k][0], H[k][1], P["lower_nut_channel"], P["m2_nut_af"])
    feat(body, "Pocket", "M2Nuts", s, Length=P["m2_nut_depth"], Reversed=True)
    # sin fillet en la base: las columnas nacen en la cama; el fillet PartDesign dejaba el sólido inválido
    return body, (False, 0)


def build_arm(doc, P):
    body = doc.addObject("PartDesign::Body", "Arm")
    zt, cy = P["deck_top_z"], P["deck_center_y"]
    mx = P["mount_x_center"]
    bx0, bx1 = mx - P["deck_mount_width"] / 2, mx + P["deck_mount_width"] / 2
    by0, by1 = cy - P["deck_mount_length"] / 2, cy + P["deck_mount_length"] / 2
    s = sk(body, "Sk_Base", offset=zt)
    s.rect(bx0, by0, bx1, by1, P["fillet_radius"])
    feat(body, "Pad", "Base", s, Length=P["deck_mount_thickness"])
    s = sk(body, "Sk_Slots", offset=zt + P["deck_mount_thickness"])
    for sgn in (-1, 1):
        s.rect_c(P["slot_x_center"], cy + sgn * P["slot_spacing"] / 2, P["slot_length"], P["slot_width"],
                 P["slot_width"] / 2 - 0.05)
    s.rect_c(P["deck_pass_x"], P["deck_pass_y"], P["base_pass_l"], P["base_pass_w"])
    feat(body, "Pocket", "Slots", s, Type="ThroughAll")
    yo = P["arm_center_offset"] + P["arm_thickness"] / 2
    s = sk(body, "Sk_Wall", offset=zt - P["wall_drop"])
    s.rect(bx0, cy - yo, P["arm_top_rear_x"], cy + yo)
    feat(body, "Pad", "FrontWall", s, Length=P["wall_drop"])
    px, pz, lx, lz = P["pivot_x"], P["pivot_z"], P["lock_x"], P["lock_z"]
    hb = P["arm_bottom_width"] / 2
    top = zt + P["deck_mount_thickness"]
    for i, sgn in enumerate((-1, 1)):
        yc = cy + sgn * P["arm_center_offset"]
        s = sk(body, "Sk_Arm%d" % i, plane="XZ_Plane", offset=-yc)
        s.poly([(px - hb, pz), (px + hb, pz), (P["arm_top_rear_x"], zt), (P["arm_top_rear_x"], top), (P["arm_top_front_x"], top)])
        feat(body, "Pad", "Arm%d" % i, s, Length=P["arm_thickness"], SideType="Symmetric")
        s = sk(body, "Sk_Lobe%d" % i, plane="XZ_Plane", offset=-yc)
        lb = P["lock_boss_r"]
        s.poly([(px - lb, pz), (px + lb, pz), (lx + lb, lz), (lx - lb, lz)])
        feat(body, "Pad", "Lobe%d" % i, s, Length=P["arm_thickness"], SideType="Symmetric")
        s = sk(body, "Sk_Bosses%d" % i, plane="XZ_Plane", offset=-yc)
        s.circle(px, pz, P["pivot_boss_r"])
        s.circle(lx, lz, P["lock_boss_r"])
        feat(body, "Pad", "Bosses%d" % i, s, Length=P["arm_thickness"], SideType="Symmetric")
    for i, sgn in enumerate((-1, 1)):
        yin = cy + sgn * (P["arm_center_offset"] - P["arm_thickness"] / 2 + 1.0)
        s = sk(body, "Sk_Rib%d" % i, plane="YZ_Plane", offset=bx0)
        if sgn < 0:
            p3 = [(yin, zt - P["rib_height"]), (yin + P["rib_length"], zt), (yin, zt)]
        else:
            p3 = [(yin, zt - P["rib_height"]), (yin, zt), (yin - P["rib_length"], zt)]
        s.poly(p3)
        feat(body, "Pad", "Rib%d" % i, s, Length=P["rib_thickness"])
    s = sk(body, "Sk_ArmHoles", plane="XZ_Plane", offset=-cy)
    s.circle(px, pz, P["m3_clearance"] / 2)
    s.circle(lx, lz, P["m3_clearance"] / 2)
    feat(body, "Pocket", "ArmHoles", s, Type="ThroughAll", SideType="Symmetric")
    s = sk(body, "Sk_TieSlots", plane="XZ_Plane", offset=-cy)
    s.rect_c(P["tie_slot_x"], P["tie_slot_z"], P["tie_slot_width"], P["tie_slot_length"])
    feat(body, "Pocket", "TieSlots", s, Type="ThroughAll", SideType="Symmetric")
    return body


def keepouts(v):
    """Zonas de exclusión en coordenadas locales (cajas Part, para validar y mostrar)."""
    aw, ah, ad = v["assembly_width"], v["assembly_height"], v["assembly_depth"]
    zb = v["frame_depth"] + v["standoff_height"]                     # cara trasera de la placa
    k = {}
    half = v["pin_section_width"] / 2
    xl0 = v["pin_row_center_x"] - half - v["pin_keepout"]
    xl1 = v["pin_row_center_x"] - half + 2.54 + v["pin_keepout"]
    xr0 = v["pin_row_center_x"] + half - 2.54 - v["pin_keepout"]
    xr1 = v["pin_row_center_x"] + half + v["pin_keepout"]
    y0, y1 = v["pin_row_y0"] - v["pin_keepout"], v["pin_row_y1"] + v["pin_keepout"]
    for nm, a, b in (("pines_izq", xl0, xl1), ("pines_der", xr0, xr1)):
        k[nm] = Part.makeBox(b - a, y1 - y0, zb, V(a, y0, 0))       # detrás de la placa, toda la profundidad
    A_r = v["upper_hole_x_left"] + v["standoff_outer_diameter"] / 2 + 0.5
    B_l = v["upper_hole_x_right"] - v["standoff_outer_diameter"] / 2 - 0.5
    k["pines_sup"] = Part.makeBox(B_l - A_r, v["top_pin_zone_height"], zb, V(A_r, ah - v["top_pin_zone_height"], 0))
    sl = v["lower_slot_length"] + v["standoff_outer_diameter"] - v["lower_slot_width"]
    C_r = v["lower_hole_x_left"] + sl / 2 + 0.5
    D_l = v["lower_hole_x_right"] - sl / 2 - 0.5
    k["pines_inf"] = Part.makeBox(D_l - C_r, v["bottom_pin_zone_height"], zb, V(C_r, 0, 0))
    k["usb"] = Part.makeBox(v["usb_keepout_width"], v["usb_keepout_height"], zb + ad,
                            V(aw / 2 - v["usb_keepout_width"] / 2, ah, 0))
    k["lateral"] = Part.makeCompound([Part.makeBox(v["lateral_clearance"], ah, ad, V(-v["lateral_clearance"], 0, zb)),
                                      Part.makeBox(v["lateral_clearance"], ah, ad, V(aw, 0, zb))])
    k["modulo"] = Part.makeBox(aw, ah, ad, V(0, 0, zb))
    k["delante_lente"] = Part.makeBox(aw + 30, ah + 30, 60, V(-15, -15, zb + ad))
    return k


def main():
    args = argv_script()
    out = os.path.abspath(args[args.index("--out") + 1])
    os.makedirs(out, exist_ok=True)
    doc = App.newDocument("ESP32_CAM_Open_Frame")
    P, v = build_params(doc, SHEET, PARAMS)
    frame, fil = build_frame(doc, P, "OpenFrame", ears=True)
    test, _ = build_frame(doc, P, "FrameTest", ears=False)
    arm = build_arm(doc, P)
    ch = doc.addObject("PartDesign::Body", "ChassisRef")
    s = sk(ch, "Sk_Chassis"); s.rect(E.k(0), E.k(0), P["chassis_length"], P["chassis_width"])
    feat(ch, "Pad", "ChassisPlate", s, Length=P["chassis_thickness"], Reversed=True)
    env = doc.addObject("PartDesign::Body", "CamModule_Env")
    s = sk(env, "Sk_Env", offset=P["frame_depth"] + P["standoff_height"])
    s.rect(E.k(0), E.k(0), P["assembly_width"], P["assembly_height"])
    feat(env, "Pad", "EnvBox", s, Length=P["assembly_depth"])
    deck_step = os.path.join(HERE, "..", "deck-v1", "out", "rover_deck_v1_deck.step")
    deck = None
    if os.path.exists(deck_step):
        sh = Part.Shape(); sh.read(deck_step)
        deck = doc.addObject("Part::Feature", "ComputeDeck"); deck.Shape = sh
    doc.recompute()

    tilt = doc.addObject("App::Part", "CameraTilt")
    tilt.Placement = App.Placement(V(v["pivot_x"], v["deck_center_y"], v["pivot_z"]), App.Rotation(V(0, 1, 0), -v["tilt_angle"]))
    for prop, ex in ((".Placement.Base.x", "pivot_x"), (".Placement.Base.y", "deck_center_y"), (".Placement.Base.z", "pivot_z")):
        tilt.setExpression(prop, "%s.%s" % (SHEET, ex))
    tilt.setExpression(".Placement.Rotation.Angle", "-%s.tilt_angle" % SHEET)
    rot0 = rot_from_rows(M0_ROWS)
    base_local = V(v["ear_depth"] / 2, v["assembly_width"] / 2, -v["assembly_height"] / 2)
    for nm, tgt in (("Frame_Placed", frame), ("CamModule_Placed", env)):
        lk = doc.addObject("App::Link", nm)
        lk.LinkedObject = tgt
        lk.Placement = App.Placement(base_local, rot0)
        lk.setExpression(".Placement.Base.x", "%s.ear_depth / 2" % SHEET)
        lk.setExpression(".Placement.Base.y", "%s.assembly_width / 2" % SHEET)
        lk.setExpression(".Placement.Base.z", "-%s.assembly_height / 2" % SHEET)
        tilt.addObject(lk)
    KO = keepouts(v)
    ko = doc.addObject("Part::Feature", "Keepouts")
    ko.Shape = Part.makeCompound([s_ for n_, s_ in KO.items() if n_ not in ("delante_lente", "modulo")])
    verificar(doc)

    def to_rover(tdeg=None):
        t = v["tilt_angle"] if tdeg is None else tdeg
        return App.Placement(V(v["pivot_x"], v["deck_center_y"], v["pivot_z"]), App.Rotation(V(0, 1, 0), -t)).multiply(
            App.Placement(base_local, rot0))
    PL = to_rover()
    def rover(shape, pl=PL):
        s_ = shape.copy(); s_.Placement = pl.multiply(s_.Placement); return s_
    def local(shape):
        s_ = shape.copy(); s_.Placement = PL.inverse().multiply(s_.Placement); return s_
    A, F, M = arm.Shape, rover(frame.Shape), rover(env.Shape)
    CH, DK = ch.Shape, (deck.Shape if deck else None)
    Fl = frame.Shape
    R = []
    def chk(n, ok, d):
        R.append((n, "OK" if ok is True else ("FALLA" if ok is False else ok), d))

    # 1 agujeros superiores y 2 ranuras inferiores, medidos en el sólido
    top = v["frame_depth"] + v["standoff_height"]
    circ = sorted({(round(e.Curve.Center.x, 3), round(e.Curve.Center.y, 3)) for e in Fl.Edges
                   if e.Curve.TypeId == "Part::GeomCircle" and abs(e.Curve.Radius - v["m2_clearance"] / 2) < 1e-4
                   and abs(e.Curve.Center.z - top) < 1e-4 and e.Curve.Center.y > v["assembly_height"] / 2})
    exp_ab = sorted({(round(v["upper_hole_x_left"], 3), round(v["upper_hole_y"], 3)), (round(v["upper_hole_x_right"], 3), round(v["upper_hole_y"], 3))})
    chk("1. Agujeros superiores A/B", circ == exp_ab, "medidos %s; A-B %.2f" % (circ, v["top_hole_spacing"]))
    C = (v["lower_hole_x_left"], v["lower_hole_y_left"]); D = (v["lower_hole_x_right"], v["lower_hole_y_right"])
    Ax, Ay, Bx = v["upper_hole_x_left"], v["upper_hole_y"], v["upper_hole_x_right"]
    dist = lambda p, q: math.hypot(p[0] - q[0], p[1] - q[1])
    travel = (v["lower_slot_length"] - v["lower_slot_width"]) / 2
    off = (v["top_hole_spacing"] - v["bottom_hole_spacing"]) / 2
    hh = math.sqrt(v["left_upper_to_lower"] ** 2 - off ** 2)
    Cs, Ds = (Ax + off, Ay - hh), (Bx - off, Ay - hh)
    chk("2. Ranuras inferiores C/D", abs(Cs[0] - C[0]) <= travel and abs(Ds[0] - D[0]) <= travel,
        "centros C=(%.3f, %.3f) D=(%.3f, %.3f) resueltos con A-C, B-D, C-D y A-D (residuos %.3f/%.3f/%.3f/%.3f mm). "
        "Recorrido +-%.2f mm cubre también la solución simétrica C=(%.2f, %.2f) D=(%.2f, %.2f), que da A-D=%.2f en vez de %.2f"
        % (C[0], C[1], D[0], D[1], dist((Ax, Ay), C) - v["left_upper_to_lower"], dist((Bx, Ay), D) - v["right_upper_to_lower"],
           dist(C, D) - v["bottom_hole_spacing"], dist((Ax, Ay), D) - v["diagonal_A_D"], travel,
           Cs[0], Cs[1], Ds[0], Ds[1], dist((Ax, Ay), Ds), v["diagonal_A_D"]))
    # 3-8 zonas de exclusión: el frame no puede entrar (salvo las 4 columnas en sus agujeros)
    cols = Part.makeCompound([Part.makeCylinder(v["standoff_outer_diameter"] / 2 + 0.6, 50, V(Ax, Ay, -1)),
                              Part.makeCylinder(v["standoff_outer_diameter"] / 2 + 0.6, 50, V(Bx, Ay, -1)),
                              Part.makeBox(v["lower_slot_length"] + v["standoff_outer_diameter"] - v["lower_slot_width"] + 1.2,
                                           v["standoff_outer_diameter"] + 1.2, 50,
                                           V(C[0] - (v["lower_slot_length"] + v["standoff_outer_diameter"] - v["lower_slot_width"]) / 2 - 0.6, C[1] - v["standoff_outer_diameter"] / 2 - 0.6, -1)),
                              Part.makeBox(v["lower_slot_length"] + v["standoff_outer_diameter"] - v["lower_slot_width"] + 1.2,
                                           v["standoff_outer_diameter"] + 1.2, 50,
                                           V(D[0] - (v["lower_slot_length"] + v["standoff_outer_diameter"] - v["lower_slot_width"]) / 2 - 0.6, D[1] - v["standoff_outer_diameter"] / 2 - 0.6, -1))])
    for n_, key in (("3. Headers laterales (filas de pines + 3 mm)", ("pines_izq", "pines_der")),
                    ("4. Pines superiores", ("pines_sup",)), ("5. Pines inferiores", ("pines_inf",)),
                    ("6. USB-C 16x12", ("usb",)), ("6b. Laterales 3 mm", ("lateral",))):
        vol = sum(Fl.cut(cols).common(KO[k]).Volume for k in key)
        chk(n_, vol < 1e-6, "interferencia frame-zona %.4f mm3" % vol)
    tails = []
    zb = top
    for k in ("pines_izq", "pines_der"):
        bb = KO[k].BoundBox
        tails.append(Part.makeBox(bb.XLength - 2 * v["pin_keepout"], bb.YLength - 2 * v["pin_keepout"], v["pin_tail_length"],
                                  V(bb.XMin + v["pin_keepout"], bb.YMin + v["pin_keepout"], zb - v["pin_tail_length"])))
    vt = sum(Fl.common(t).Volume for t in tails) + Fl.common(KO["modulo"]).Volume
    gap = min(Fl.distToShape(t)[0] for t in tails)
    chk("7. Ningún componente toca el PLA", vt < 1e-6,
        "colas de pines (%.1f mm, SUPUESTAS) y volumen del módulo sin interferencia; distancia mínima frame-colas %.2f mm; "
        "la placa sólo apoya en 4 coronas de Ø%.1f" % (v["pin_tail_length"], gap, v["standoff_outer_diameter"]))
    footprint = Part.makeBox(v["assembly_width"], v["assembly_height"], v["standoff_height"] - 0.01, V(0, 0, v["frame_depth"] + 0.01))
    vin = Fl.cut(cols).common(footprint).Volume
    open_area = v["assembly_width"] * v["assembly_height"]
    proj = Fl.cut(cols).common(Part.makeBox(v["assembly_width"], v["assembly_height"], 50, V(0, 0, -1)))
    chk("8. Centro abierto / placa suspendida", vin < 1e-6,
        "fuera de las columnas no hay material a menos de %.1f mm detrás de la placa (%.4f mm3); "
        "dentro de la huella sólo quedan las franjas de los montantes (x<%.1f y x>%.2f) y las esquinas"
        % (v["standoff_height"], vin, v["upright_inner_x"], v["assembly_width"] - v["upright_inner_x"]))
    vlens = Fl.common(KO["delante_lente"]).Volume + local(A).common(KO["delante_lente"]).Volume
    chk("9. Retirar la placa", vlens < 1e-6, "4 tornillos M2 por delante; nada delante del módulo (%.4f mm3); tuercas cautivas detrás" % vlens)

    def overhangs(shape):
        z0 = shape.optimalBoundingBox().ZMin; bad = []
        for fc in shape.Faces:
            pr = fc.ParameterRange
            try:
                n = fc.normalAt((pr[0] + pr[1]) / 2, (pr[2] + pr[3]) / 2)
            except Exception:
                continue
            bb_ = fc.optimalBoundingBox()
            if n.z < -0.72 and fc.Area > 1 and abs(bb_.ZMax - z0) > 1e-3:
                bad.append(round(min(bb_.XLength, bb_.YLength), 1))
        return bad
    arm_p = A.copy(); arm_p.rotate(V(0, 0, 0), V(1, 0, 0), 180); arm_p.translate(V(0, 0, -arm_p.optimalBoundingBox().ZMin))
    fr_p, te_p = Fl.copy(), test.Shape.copy()
    oa, of, ot = overhangs(arm_p), overhangs(fr_p), overhangs(te_p)
    chk("10. Imprimible sin soportes", max(oa + of + ot + [0]) <= 12,
        "voladizos (ancho mm): brazo %s, frame %s, prueba %s (techos de tuercas y agujeros horizontales)"
        % (sorted(set(oa)) or "ninguno", sorted(set(of)) or "ninguno", sorted(set(ot)) or "ninguno"))
    secs = {"montante": (v["frame_outer_width"] / 2 - v["assembly_width"] / 2 + v["upright_inner_x"], v["frame_depth"]),
            "brazo": (v["arm_bottom_width"], v["arm_thickness"]), "base": (v["deck_mount_width"], v["deck_mount_thickness"])}
    walls = {"columna M2 (5.5-2.2)/2": (v["standoff_outer_diameter"] - v["m2_clearance"]) / 2,
             "oreja junto a tuerca M3": v["frame_outer_width"] / 2 - v["assembly_width"] / 2 + v["ear_extension"] + v["upright_inner_x"] - v["m3_nut_depth"],
             "buje pivote": v["pivot_boss_r"] - v["m3_clearance"] / 2, "buje bloqueo": v["lock_boss_r"] - v["m3_clearance"] / 2,
             "puente inferior": v["frame_wall"]}
    low = [k for k, w in walls.items() if w < v["minimum_wall"]]
    chk("11. Rigidez / paredes", "AVISO" if low else True,
        "secciones %s; paredes %s%s. Unión rígida: 2 M3 por lado (pivote + bloqueo a %.0f mm). Sin análisis de elementos finitos"
        % ({k: "%gx%g" % s_ for k, s_ in secs.items()}, {k: round(w, 2) for k, w in walls.items()},
           (" — bajo %.1f: %s" % (v["minimum_wall"], low)) if low else "", v["lock_distance"]))
    d = PL.Rotation.multVec(V(0, 0, 1))
    lens = PL.multVec(V(v["assembly_width"] / 2, v["assembly_height"] / 2, top + v["assembly_depth"]))
    pts_ = list(A.Vertexes) + list(CH.Vertexes) + (list(DK.Vertexes) if DK else []) + list(F.Vertexes)
    worst = max((p.Point - lens).dot(d) for p in pts_)
    chk("12. No bloquea la cámara", worst < 0,
        "dirección (%.3f, %.3f, %.3f); todo el soporte, el piso y el chasis quedan >= %.1f mm detrás del plano frontal del módulo"
        % (d.x, d.y, d.z, -worst))
    inter = {"brazo-piso": A.common(DK).Volume if DK else -1, "frame-piso": F.common(DK).Volume if DK else -1,
             "brazo-chasis": A.common(CH).Volume, "frame-chasis": F.common(CH).Volume, "frame-brazo": F.common(A).Volume,
             "módulo-brazo": M.common(A).Volume, "módulo-chasis": M.common(CH).Volume}
    gch = F.distToShape(CH)[0]
    chk("13. Interferencias en el rover", all(0 <= x < 1e-4 for x in inter.values()) and DK is not None and gch >= 0.5,
        ", ".join("%s %.4f" % kv for kv in inter.items()) + " mm3; holgura frame-chasis %.2f mm" % gch)
    free = [t for t in range(-10, 26) if rover(frame.Shape, to_rover(t)).common(CH).Volume < 1e-3
            and rover(frame.Shape, to_rover(t)).common(A).Volume < 1e-3]
    chk("14. Margen para V2 ajustable", "INFO", "con este brazo el frame podría girar sin choques entre %+d° y %+d° (quitando el M3 de bloqueo)"
        % (min(free), max(free)) if free else "sin margen")
    chk("15. Altura sobre el suelo", "NO VERIFICABLE", "punto más bajo del frame Z=%.1f mm (bajo la cara superior del chasis)" % F.optimalBoundingBox().ZMin)
    sk_bad = [o.o.Name for o in SKETCHES if not o.o.FullyConstrained or o.o.ConflictingConstraints or o.o.RedundantConstraints]
    nexpr = sum(len(o.ExpressionEngine) for o in doc.Objects if hasattr(o, "ExpressionEngine"))
    chk("16. Editable", not sk_bad, "%d sketches totalmente restringidos%s; %d expresiones"
        % (len(SKETCHES), "" if not sk_bad else ", SALVO " + ", ".join(sk_bad), nexpr))

    print("\n== validaciones ==")
    for n_, st, d_ in R:
        print("[%s] %s: %s" % (st, n_, d_))
    print("fillet columnas:", fil)
    for n_, s_ in (("brazo", A), ("frame", Fl), ("prueba", test.Shape)):
        bb = s_.optimalBoundingBox()
        print("%-7s %.1f x %.1f x %.1f mm, %.2f cm3, ~%.1f g PLA macizo, válido %s, sólidos %d"
              % (n_, bb.XLength, bb.YLength, bb.ZLength, s_.Volume / 1000, masa(s_), s_.isValid(), len(s_.Solids)))

    for fn, shp in (("Rover_Camera_Arm.stl", arm_p), ("ESP32_CAM_Open_Frame.stl", fr_p), ("ESP32_CAM_OpenFrame_Test_V5_2mm.stl", te_p)):
        m, probs = validar_para_impresion(shp)
        if probs:
            raise RuntimeError("%s: %s" % (fn, probs))
        m.write(os.path.join(out, fn))
    tmp = []
    for nm, s_ in (("Arm", A), ("OpenFrame", F)):
        o = doc.addObject("Part::Feature", "EXP_" + nm); o.Shape = s_; tmp.append(o)
    Import.export(tmp, os.path.join(out, "assembly.step"))
    o = doc.addObject("Part::Feature", "EXP_Frame"); o.Shape = Fl; tmp.append(o)
    Import.export([o], os.path.join(out, "ESP32_CAM_Open_Frame.step"))
    for o in tmp:
        doc.removeObject(o.Name)
    for b in (arm, frame, test, ch, env):
        for o in b.Group:
            o.Visibility = (o == b.Tip)
    frame.Visibility = env.Visibility = test.Visibility = ko.Visibility = False
    doc.recompute()
    doc.saveAs(os.path.join(out, "ESP32_CAM_Open_Frame.FCStd"))
    with open(os.path.join(out, "validation.json"), "w") as fh:
        json.dump({"checks": R, "C": C, "D": D}, fh, indent=1, ensure_ascii=False)
    print("escrito en", out)


ejecutar(main)
