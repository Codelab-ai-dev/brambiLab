# Roadmap de la plataforma
Estado: propuesta (2026-11-25). Sin fechas comprometidas. Cada etapa empieza solo cuando la anterior cumple su condición de salida y Gustavo la prioriza. La seguridad forma parte de cada etapa, no de una etapa final.

| # | Etapa | Resultado | Condición de salida |
|---|---|---|---|
| 0 | **Fundación** (FND-V2-001, [#76](https://github.com/Codelab-ai-dev/brambiLab/issues/76)) | Visión, PRD, arquitectura, [ADR-011](../../docs/architecture/ADR-011-evolucion-plataforma.md) y este roadmap | PR fundacional revisado y aprobado; decisión sobre el repositorio privado registrada |
| 1 | **Contrato de conectores y laboratorio POS** | ADR del contrato de conectores; POS simulado con semilla; reportes de referencia; catálogo de herramientas tipadas; prueba estática de «sin escrituras» | Laboratorio reproducible con un comando; catálogo revisado; ADRs de stack y contrato aceptados |
| 2 | **Gateway de solo lectura** | Servidor MCP local con Guard mínimo (validación, autorización, límites), errores tipados y auditoría | Métricas M-01 a M-06 del [PRD](requirements.md#8-métricas-de-aceptación-del-mvp) cumplidas en el laboratorio. **Esto es el MVP** |
| 3 | **Edge y controles de producción** | Edge instalable con conexión saliente, identidad por cliente, gestión de secretos, actualización y reversión, monitorización | ADRs de transporte, identidad y ejecución local aceptados; revisión de seguridad; prueba en un entorno que imite la red de un cliente |
| 4 | **Studio y piloto** | Interfaz de administración; piloto con un cliente real en solo lectura | Autorización escrita del cliente; métricas de exactitud y de integridad repetidas sobre su sistema; aprendizajes documentados sin datos del cliente en este repositorio |
| — | **Posterior** | Discovery asistido; Flow (acciones con escritura) | Cada una con su propia especificación, su modelo de amenazas y su aprobación explícita |

## Laboratorio opcional: ROVER
El [AI Rover (BL-001)](../../projects/001-ai-rover/README.md) sigue como investigación independiente y no bloquea ninguna etapa. Más adelante puede servir de segundo origen de datos para probar el contrato de conectores con un sistema distinto: **telemetría de solo lectura** desde ROS 2 (por ejemplo `/rover/state`), **sin control de motores**. Exponer cualquier acción sobre el rover requeriría su propia especificación de seguridad.

## Siguiente incremento
Especificación técnica del laboratorio POS de solo lectura (etapa 1), después de revisar y aprobar el PR fundacional.
