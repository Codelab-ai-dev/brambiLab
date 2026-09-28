# Punto de reanudación
Actualizado: 2026-09-27

Foundation versionada y licenciada en https://github.com/Codelab-ai-dev/brambiLab (repositorio público). FND-001 y FND-002 terminadas.

Prioridad actual: WEB-004 en curso: estado en [Issue #20](https://github.com/Codelab-ai-dev/brambiLab/issues/20), rama `feat/WEB-004-media`, en tres PR con base `main`: #21 (almacenamiento/API), #22 (entrega autorizada) y el panel con editor y e2e. WEB-003 terminada: [Issue #12](https://github.com/Codelab-ai-dev/brambiLab/issues/12), PR #15 (modelo/API), #16 (editor/Markdown) y #17 (panel/e2e), integrados en `main` con el PR de integración desde `feat/WEB-003-editor` (#16 y #17 se habían unido en ramas intermedias). WEB-001 y WEB-002 terminadas (#7, #11, #13). Especificación: [web-v1.md](../docs/architecture/web-v1.md); arranque y verificación en [platform/README.md](../platform/README.md). Web priorizada por Gustavo sin esperar a completar BL-001. FND-003 sigue en review.

En curso independiente: BL-001-001 (inventario), preparación ya unida en `main`. La plantilla del inventario está lista; la tarea espera los datos físicos de Gustavo.
Consulta [backlog.md](backlog.md) para aceptación y estado. Antes de modificar, lee [AGENTS.md](../AGENTS.md).

## Información necesaria para BL-001-001
No inferir estos datos de la planificación anterior. Fotos según [conventions.md](../docs/conventions.md) (nombre con fecha, sin EXIF).
1. Control: placa actual, fotos de ambas caras con la serigrafía legible. ¿Hay algún ESP32 disponible y de qué variante?
2. Potencia: modelo del driver y foto de su cableado; modelo o etiqueta de los motores y cuántos son.
3. Batería: química, tensión nominal, capacidad, conector y cargador. ¿Hay interruptor, fusible o BMS?
4. Funcionamiento actual: con qué se controla hoy (mando, app, nada) y qué funciona. Un vídeo corto con las ruedas levantadas sirve como prueba V0.
5. Mecánica: medidas del chasis (largo, ancho y alto), separación entre fijaciones, espacio libre para el teléfono y tipo de ruedas o transmisión.
6. Teléfono y soporte: disponibilidad del CMF Phone 1, ubicación de los CAD/STL del soporte pan/tilt y modelos de servos si los hay.

## Última entrega
2026-09-27 · BL-001-001 (preparación): hardware.md y bom.md alineados y ampliados con los elementos que faltaban; convención de nombres y limpieza EXIF para fotos; `.gitignore` para multimedia y builds; prueba de corte físico propuesta en validation.md; enlaces de los índices de docs/. Verificado: enlaces Markdown relativos. Sin pruebas físicas.

Anteriores: FND-001 (commit inicial `332e5f0`) y FND-002 (licencias, [ADR-007](../docs/architecture/ADR-007-licencias.md)).

## Entregas web
2026-09-28 · WEB-004 en tres PR: #21, #22 y el panel. Handoffs: [almacenamiento](../docs/handoffs/2026-09-28-web-004-storage.md), [entrega](../docs/handoffs/2026-09-28-web-004-delivery.md), [panel](../docs/handoffs/2026-09-28-web-004-panel.md).

2026-09-27 · WEB-003 en tres PR apilados: #15 (modelo y API), #16 (editor y Markdown) y el panel con e2e. Handoffs: [API](../docs/handoffs/2026-09-27-web-003-api.md), [editor](../docs/handoffs/2026-09-27-web-003-editor.md), [panel](../docs/handoffs/2026-09-27-web-003-panel.md).

2026-09-27 · WEB-002: login del propietario con GitHub, sesiones, CSRF, proxy de entrada y e2e con GitHub simulado. [Handoff](../docs/handoffs/2026-09-27-web-002.md).

2026-09-27 · WEB-001 seguimiento: credenciales PostgreSQL mediante variables `PG*` y actualización del seguimiento tras la revisión de Codex. [Handoff](../docs/handoffs/2026-09-27-web-001-followup.md).

2026-09-27 · WEB-001: scaffold SSR/Go/PostgreSQL/Compose, OpenAPI base, migraciones y CI. [Handoff](../docs/handoffs/2026-09-27-web-001.md).

2026-09-27: requisitos de la entrevista plasmados en especificación y ADR-006; roadmap y alcance alineados. [Handoff](../docs/handoffs/2026-09-27-web-v1.md). Destino externo de backup pendiente antes de lanzar.
