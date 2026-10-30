# Piso superior modular v1 (deck-v1)

Segundo piso impreso en PLA para el rover 4WD: plataforma nervada para el Arduino UNO Q, zona LiDAR, puntos de expansión M3 y 4 soportes desmontables que se posicionan en los orificios cuadrados del chasis.

**Estado:** diseño verificado en CAD, **sin imprimir ni montar**. Hay valores provisionales (ver abajo) que hay que medir antes de imprimir.

## Archivos (`out/`)
| Archivo | Contenido |
|---|---|
| `rover_deck_v1.FCStd` | Modelo paramétrico: hoja `Params`, cuerpos `Deck`, `Support`, 4 `App::Link` de soportes, referencias `ChassisRef` y `UnoQ_Env` |
| `rover_deck_v1_deck.stl` | Piso, orientado para imprimir **boca abajo**: cara superior plana sobre la cama |
| `rover_deck_v1_support.stl` | Soporte, tumbado sobre su cara exterior plana. Imprimir 4 |
| `rover_deck_v1_deck.step`, `_support.step`, `_assembly.step` | Sólidos exactos (el ensamblaje incluye los 4 soportes colocados) |
| `rover_deck_v1_plano_piso.pdf` | BL-DECK-001 · A3 1:1 · vistas superior, inferior, lateral, frontal e isométrica, con cotas |
| `rover_deck_v1_plano_posiciones.pdf` | BL-DECK-002 · A3 1:1 · cotas ordenadas de todas las interfaces y tabla de coordenadas |
| `rover_deck_v1_plano_soporte.pdf` | BL-DECK-003 · A4 2:1 · soporte acotado |
| `renders/*.png` | Vistas superior, inferior, lateral e isométricas (piso, ensamblaje y soporte) |
| `validation.json` | Resultado de las 10 validaciones |

## Regenerar
Todo sale de los scripts; no editar los archivos de `out/` a mano.
```bash
F=~/.claude/skills/freecad/scripts/fc.sh
$F run build_deck.py -- --out "$PWD/out"            # modelo, STL, STEP, validaciones
$F run render_views.py -- --dir "$PWD/out"          # PNG
$F run make_drawing.py -- --model "$PWD/out/rover_deck_v1.FCStd" --out "$PWD/out"
$F gui export_pdf.py -- --dir "$PWD/out"            # PDF (abre FreeCAD unos segundos)
```
Para ajustes puntuales también se puede editar la hoja `Params` en FreeCAD: el modelo se recalcula (probado con `deck_height` 35/45 y `group_1_x` 28, sin errores y con los sketches todavía restringidos).

## Coordenadas
Origen en la esquina de la placa del chasis, X a lo largo (0–160), Y a lo ancho (0–90) y Z=0 en la cara superior del acrílico. **Se supone que X=0 es el frente**: la zona LiDAR y los puntos de expansión frontales quedan allí, sobre los soportes. Si el frente real es el otro extremo, hay que reflejar la distribución.

## Diseño
- **Soportes (4 iguales):** cada uno usa los orificios 2 y 3 (`support_hole_index` = 1) de un grupo. Una guía cuadrada de 2.55 mm con punta cónica entra en un orificio de 2.89 mm (0.17 mm de holgura por lado), y un M2.5 entra desde abajo por el otro orificio a un piloto de Ø2.2. Arriba, un M2.5 atraviesa la orejeta del piso y rosca en la columna. La columna (16 × 8) encaja en un alojamiento del piso con 0.15 mm de holgura por lado, lo que bloquea el giro. Tienen pie con transición a 45° y 3 nervaduras hacia el interior.
- **Piso:** placa de 3 mm con retícula inferior de nervaduras de 2.4 × 6 mm y marco perimetral. Lleva refuerzos cilíndricos bajo cada punto M3 y cada separador del UNO Q, y una orejeta por soporte (la huella llega a 145 × 88.18 mm).
- **UNO Q:** 4 pilotos de Ø2.2 para **separadores comerciales M2.5** (macho-hembra, `standoff_height` = 6 mm), centrados en X=106. Debajo hay 24 ranuras de ventilación (693 mm²), la cara superior queda libre para un disipador y la placa no queda encerrada.
- **Zona frontal:** zona plana de 50 × 50 para LiDAR con 4 M3, más 2 puntos M3 de expansión (cámara o pan/tilt).
- **Tercer nivel:** 4 puntos M3 en las esquinas. Los M3 llevan alojamiento hexagonal para la tuerca desde abajo.
- **Cableado:** zona de potencia en Y baja (rótulo PWR, 2 pasos de cable) y zona de señales en Y alta y en la franja trasera (rótulo SIG, con pasos GPIO, USB, UART e I2C). Hay un paso frontal para la cámara y 5 pares de ranuras para bridas.

## Valores provisionales (medir antes de imprimir)
1. **Arduino UNO Q:** `unoq_length` y `unoq_width` usan la forma UNO (68.6 × 53.4). `unoq_hole_spacing_x/y` (58.6 × 43.4) son un **marcador, no el patrón real**; el UNO R3 tiene un patrón no rectangular. Hay que medir la placa o tomar su plano oficial, y revisar después que los refuerzos no crucen nervaduras (`build_deck.py` lo comprueba y se detiene si pasa).
2. **LiDAR:** `lidar_spacing_x/y` = 40 × 40 provisional.
3. **Espesor del acrílico:** `chassis_thickness` = 3 mm supuesto. La guía mide 2.5 mm y no debe asomar por debajo.
4. **`group_1_x` = 26:** se interpretó como el **centro del primer orificio** del grupo 1, medido desde X=0.
5. **Ruedas y motores:** no hay geometría, así que la interferencia **no está verificada**. Todo el diseño queda dentro de la huella del chasis y por encima de él.

## Impresión (Artillery Genius Pro, PLA)
- El piso se imprime **boca abajo**: unos 10 200 mm² de placa apoyan en la cama, las nervaduras crecen hacia arriba y no hay puentes ni soportes. El borde que apoya lleva un chaflán de 0.5 mm contra la pata de elefante. Los únicos voladizos son el fondo de los rótulos grabados y el de los pilotos ciegos de Ø2.2.
- La v1 inicial se imprimía sobre las nervaduras y falló 3 veces. Por eso se cambió la orientación y los separadores del UNO Q pasaron a ser comerciales.

## Riesgos conocidos
- Los soportes quedan en X 36.5 y 86.5, así que el piso vuela unos 66 mm por detrás hasta X=152.5, justo bajo el UNO Q y los puntos traseros del tercer nivel. Con la carga prevista (UNO Q de unos 40 g) es aceptable; si se cargan los puntos traseros, conviene añadir un apoyo atrás.
- Los tornillos inferiores de los soportes se ponen por debajo del chasis: hay que comprobar el acceso entre los motores.
- Masa del sólido macizo: piso de unos 73 g y soporte de unos 7.6 g. Con 4 paredes y 30–40 % de relleno pesará menos, pero no hay slicer instalado para dar una cifra real.
