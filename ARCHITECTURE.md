# Arquitectura v1
## Repositorio
Un repositorio inicial contiene gobierno, proyectos y contenido. Cada proyecto conserva documentación junto al código. `docs/projects/` es un índice, no una copia de las fichas de `projects/`.

## Fuente de verdad
Antes de crear GitHub: archivos versionables y tareas en `tasks/`. Después de migrar: Git para documentos/código; Issues para estado de tareas y aceptación; PRs para revisión. `tasks/current.md` conserva sólo enlaces y el punto de reanudación. No mantener dos backlogs activos.

## Contenido
Proyecto = desarrollo con objetivo y validación. Experimento = prueba acotada que puede alimentar un proyecto. Investigación = análisis de alternativas con fuentes y fecha. Build log = relato factual de una sesión.

## Plataforma futura
La web consumirá contenido de estas fuentes; su framework, hosting, dominio y formato de ingestión están pendientes. v1 no requiere base de datos ni servicios externos.

## BL-001: propuesta de responsabilidades
Android/CMF Phone 1: interfaz, cámara y evaluación de inferencia local.
ESP32: control de motores, recepción de comandos y parada por pérdida de comunicación.
Driver: entrega de potencia a motores. Alimentación: diseñar tras identificar componentes y consumos.
Transporte Android–ESP32: pendiente de ADR y prueba de conectividad.

La app no debe ser la única responsable de la parada ante desconexión. Timeout, rango de comandos y estado de arranque seguro deberán especificarse y probarse en el firmware antes de movimiento libre.

## Recursos y versiones
Guardar fuentes mecánicas junto a exportaciones STL cuando estén disponibles; no tratar una imagen conceptual como plano validado. Evitar vídeos y modelos grandes en Git: usar referencias y checksum; decidir almacenamiento al disponer de los archivos. No importar materiales antiguos sin inspeccionarlos.
