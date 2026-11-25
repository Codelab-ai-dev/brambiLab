# Caja de batería y plataforma de sensores v1 (battery-box-v1)

Caja modular en PLA para el rover: aloja el pack de baterías (75 × 67 × 20), guarda el cable de carga y organiza la salida de alimentación. La tapa es plana y maciza: sirve de plataforma para montar después el Arduino UNO Q y el LiDAR.

**Estado:** diseño verificado en CAD, **sin imprimir**. La tapa no lleva todavía agujeros para el UNO Q ni para el LiDAR, y la base no lleva el patrón de fijación al deck.

## Archivos (`out/`)
| Archivo | Contenido |
|---|---|
| `ROVER_Battery_Sensor_Enclosure.FCStd` | Modelo paramétrico: hoja `ROVER_BATTERY_SENSOR_BOX`, cuerpos `Base` y `Lid`, referencias `Battery_Ref`, `Compartment_Env`, `Zone_LiDAR` y `Zone_UNOQ` |
| `ROVER_Battery_Sensor_Enclosure.step` | Base y tapa montadas, sólidos exactos |
| `ROVER_Battery_Box_Base.stl` | Base, fondo sobre la cama |
| `ROVER_Battery_Box_Lid.stl` | Tapa ya girada: cara superior sobre la cama, labio y nervaduras hacia arriba |
| `ROVER_Battery_Sensor_Enclosure_plano.pdf` | BL-BOX-001 · A3 1:1 · base superior, frontal, lateral, sección A-A y tapa inferior, acotadas |
| `renders/*.png` | Isométrica cerrada y abierta, explosionada, base superior, tapa inferior, frontal, lateral, sección y sección de la torre |
| `validation.json` | Resultado de las 18 validaciones |

## Regenerar
```bash
F=~/.claude/skills/freecad/scripts/fc.sh
$F run build_box.py -- --out "$PWD/out"            # modelo, STL, STEP y validaciones
$F run render_views.py -- --dir "$PWD/out"         # PNG
$F run make_drawing.py -- --model "$PWD/out/ROVER_Battery_Sensor_Enclosure.FCStd" --out "$PWD/out"
$F gui export_pdf.py -- --dir "$PWD/out"           # PDF (abre FreeCAD unos segundos)
$F gui fix_display.py -- --file "$PWD/out/ROVER_Battery_Sensor_Enclosure.FCStd"   # visibilidad y colores
```
Los 15 sketches están totalmente restringidos y atados a la hoja con expresiones. Los ajustes puntuales se pueden hacer en la hoja desde FreeCAD.

## Coordenadas y distribución
El origen está en una esquina inferior de la caja: X a lo largo (0–128.92), Y a lo ancho (0–76) y Z=0 en la cara de apoyo. El pack ocupa el extremo X bajo (compartimento X 12–89) y la zona de cableado el extremo X alto (X 92–125.92), donde está la salida de cable.

## Diseño
- **Torres de esquina (4), de 12 × 12 desde la esquina exterior y altura completa.** Cada torre cumple dos funciones:
  - **Boss de tapa.** Arriba tiene un alojamiento Ø4 × 6 para inserto M3 termofijado, un piloto Ø3.4 de 16 mm y una ranura para tuerca M3 a Z=19, abierta hacia el interior. Cualquiera de las dos opciones funciona y no hay que decidir todavía.
  - **Mounting pad.** De Z=0 a Z=15, bajo el piloto, la torre es maciza. Ahí se perfora la fijación M3 al deck desde abajo.
  - El boss Ø8 del encargo queda inscrito en la torre. Las dos torres del lado de la batería sirven además de tope de fondo del pack.
- **Topes.** Hay 2 topes longitudinales de 3 × 15 × 10 en el lado de cables y dejan un hueco central de 40 mm para los cables (es el divisor parcial). Hay 2 topes laterales de 30 mm pegados a las paredes largas. Las aristas hacia el pack llevan fillet de 1 mm. El pack queda con 1 mm de holgura por lado y apoya sólo en el fondo.
- **Salida de cable.** Es una ventana de 8 × 5 con r1.5 en la pared del extremo X alto, desde Z=7. Antes está el **anclaje de brida**: un puente con túnel de 10 × 3, con los cables apoyados encima a Z=8. La brida abraza cables y puente, así que un tirón no llega al pack.
- **Ventilación.** 12 ranuras de 18 × 2 (2 por grupo, 3 grupos por pared larga) a Z=17 y Z=22, por encima de los topes y lejos de las torres y de la salida.
- **Tapa.** Placa de 3 mm, plana, con 4 agujeros M3 en las esquinas. El labio de alineación (2.4 × 2) tiene 0.3 mm de holgura y se interrumpe en las torres. **Las nervaduras (2 longitudinales + 2 transversales, 3 × 3) sólo están sobre la zona de cables.** Sobre el pack no hay nervaduras, bosses ni tornillos. La holgura tapa–pack es de 6 mm en el labio y 8 mm en la placa.
- **Zonas conceptuales.** LiDAR 50 × 50 en X 75.92–125.92, centrada en Y. UNO Q en el resto. Son objetos de referencia y no tocan la geometría.

## Desviaciones respecto al encargo
1. **Zona de cableado de 33.92 mm, no 45.** Con un interior de 122.92 mm, compartimento de 77 y tope de 3, ni siquiera sin torres cabrían 45 (quedarían 42.92). Las torres del lado del pack restan otros 9 mm. Con insertos solamente (sin tuerca cautiva), `mounting_pad_size` = 10 devuelve 2 mm.
2. **Topes laterales de 0.5 mm.** El interior mide 70 de ancho y el compartimento 69, así que un tope de 3 mm por lado no cabe. Los topes laterales se suman a la pared hasta dejar los 69 exactos.
3. **Nervaduras sólo sobre la zona de cables.** El pack ocupa todo el ancho, así que cualquier nervadura longitudinal completa pasaría sobre él (punto 16). Sobre el pack la tapa es una placa de 3 mm apoyada en todo su perímetro.
4. **Fillets interiores.** Las esquinas interiores de las torres y las exteriores de la caja tienen r2. Las uniones cóncavas pared–torre y pared–tope quedan vivas.

## Impresión (PLA, 0.4 / 0.2, 4 paredes, 5/5 capas, 30–40 %)
- **Base:** fondo sobre la cama, con chaflán de 0.5 mm contra la pata de elefante. Los únicos voladizos son puentes: ventilación (2 mm), techo de la salida, tuercas (5.9 mm) y túnel de la brida (10 mm).
- **Tapa:** boca abajo (el STL ya viene girado), sin voladizos.
- Masa del sólido macizo: base ~88 g y tapa ~40 g. Con relleno pesarán menos, pero no hay slicer instalado para dar una cifra real.

## Pendiente de verificar con la pieza real
- Diámetro del inserto M3 que se use (`lid_insert_diameter` = 4.0).
- Por dónde salen los cables del pack y en qué lado del rover conviene la salida (hoy: extremo X alto, centrada).
- Patrón de fijación al deck, y agujeros del UNO Q y del LiDAR (al perforar la zona LiDAR hay nervaduras debajo en X 92–125: perforar entre ellas).
