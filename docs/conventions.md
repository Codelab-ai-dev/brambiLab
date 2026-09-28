# Identificadores y convenciones
| Prefijo | Uso | Ejemplo |
|---|---|---|
| BL-NNN | Proyecto | BL-001 AI Rover |
| BL-NNN-NNN | Tarea de proyecto | BL-001-001 Inventario |
| FND-NNN | Tarea de Foundation | FND-001 Revisión inicial |
| EXP-NNN | Experimento | EXP-001 Transporte Android–ESP32 |
| RES-NNN | Investigación | RES-001 Opciones de transporte |
| ADR-NNN | Decisión arquitectónica | ADR-005 Transporte |

Reserva un ID al crear el archivo/tarea; no reutilices IDs cerrados. Números de GitHub Issue y IDs BL son distintos. Fechas ISO YYYY-MM-DD y zona horaria al medir tiempos. Costes con moneda, fecha y origen; valores desconocidos como pendiente, nunca cero.

Estados de tarea: backlog, ready, in-progress, blocked, review, done. Máximo recomendado: una tarea activa por persona/agente. Los archivos multimedia deben incluir fecha, descripción y relación con el hito. Nombre: `YYYY-MM-DD_descripcion-corta.ext` en minúsculas y sin espacios, por ejemplo `2026-09-28_placa-control-frontal.jpg`. Antes de subirlos, borrar los metadatos EXIF (sobre todo la ubicación GPS), por ejemplo con `exiftool -all= archivo.jpg`. No incluir personas, pantallas con datos ni direcciones sin permiso.
