# Entrega de sesión
Fecha y zona: 2026-11-25 (hora local) | Agente: Claude Code (Opus 5.5) | Tarea: FND-V2-001 · Foundation v2, plataforma de integración con MCP ([#76](https://github.com/Codelab-ai-dev/brambiLab/issues/76))
Rama: docs/FND-V2-001-enterprise-ai-foundation | Commit base/final (si existen): b56fa8b / el del PR

## Objetivo y aceptación
Documentar la evolución de BrambiLab hacia una plataforma de integración empresarial y de modernización no invasiva mediante MCP, conservando el AI Rover como investigación y la web como presencia pública. Solo documentación. Criterios de #76:

| Criterio | Estado |
|---|---|
| BrambiLab como marca y producto; ROVER como investigación | Cumplido: README, PROJECT, ADR-011 |
| PRD con problema, usuarios, alcance, exclusiones, requisitos y métricas verificables | Cumplido: [requirements.md](../../products/integration-platform/requirements.md) |
| Arquitectura que separa CMS público, núcleo comercial, Gateway y ejecución local, con límites de confianza y flujo de datos | Cumplido: [architecture.md](../../products/integration-platform/architecture.md) y ARCHITECTURE.md |
| ADR con alternativas de repositorio y licencia y decisiones pendientes marcadas | Cumplido: [ADR-011](../architecture/ADR-011-evolucion-plataforma.md) |
| MVP POS con un flujo reproducible propuesto, sin datos reales ni escrituras | Cumplido como propuesta (PRD §7 y §8) |
| Afirmaciones obsoletas corregidas con evidencia (#71, #73, #75, #43) | Cumplido; detalle abajo |
| Enlaces y consistencia verificados; handoff | Cumplido; ver «Verificaciones ejecutadas» |
| Sin cambios en código, firmware, base de datos, DNS, licencias ni despliegues | Cumplido |
| PR abierto sin fusionar | Abierto al cerrar esta sesión, sin fusionar |

## Cambios y archivos
Nuevos:
- `products/integration-platform/README.md`, `vision.md`, `requirements.md`, `architecture.md`, `roadmap.md`.
- `docs/architecture/ADR-011-evolucion-plataforma.md`.
- Este handoff.

Modificados:
- `README.md`, `PROJECT.md`, `ARCHITECTURE.md`, `ROADMAP.md`: misión, producto, investigación y estado verificable por separado.
- `DECISIONS.md`: fila de ADR-011 y lista de ADRs posteriores propuestos.
- `docs/architecture/README.md`: el índice incluye ADR-006, ADR-010 y ADR-011 (faltaban los dos primeros).
- `docs/projects/README.md`: estado de BL-001 y enlace al producto.
- `projects/001-ai-rover/README.md`: estado y «Resultado actual».
- `tasks/backlog.md`: UI-53 terminada.
- `tasks/current.md`: punto de reanudación y entrega. Se conservan los pendientes del ROVER y de la web.

Afirmaciones obsoletas corregidas, con su evidencia:
- README «sin desplegar, sin firmware, app ni pruebas físicas» → web desplegada con pendientes en #43 ([lanzamiento.md](../operations/lanzamiento.md)); firmware, app y pruebas ([validation.md](../../projects/001-ai-rover/validation.md)).
- BL-001 README «el UNO Q quedó fuera del proyecto; el rover no se ha movido» → arquitectura vigente de ADR-009, con #71 y #73 cerrados (validation.md, rondas de #71 y #73).
- `tasks/current.md` «#73 en `feat/ROVER-MOBILE-004-teleop`; revisar el PR» → #73 cerrado el 2026-11-06 y PR #74 unido.
- `tasks/backlog.md` «UI-53 en PR» → #53 cerrado y PR #55 unido.
- PROJECT «fuera de v1: MCP propio, ROS» → MCP es ahora el eje del producto y ROS ya se usa en el rover.

## Verificaciones ejecutadas
- Enlaces Markdown relativos y anclas de encabezado en los 17 archivos tocados, comprobados con un script de Python local (existencia del archivo de destino y del ancla con el criterio de GitHub): **0 errores**. Los enlaces externos no se comprobaron.
- Estado de #43, #71, #73, #75, #53 y de los PR #55, #74 y #77 consultado con `gh`.
- `git diff --stat`: solo archivos `.md`.
- No hay tests ni build para documentación: **no ejecutados porque no existen**.
- El diagrama Mermaid no se ha comprobado renderizado en GitHub.

## Pendientes y bloqueos
- Gustavo: aprobar (o cambiar) la alternativa B de ADR-011 y el nombre del repositorio privado.
- Decidir si el laboratorio POS sintético vive en este repositorio o en el privado.
- Estado de ADR-010: sigue «propuesto» aunque #73 se cerró. Lo decide Gustavo; no se ha cambiado.
- Pendientes del ROVER (#75, 10 kΩ, corte con interruptor, Wi-Fi del UNO Q) y de la web (#43) sin cambios.

## Decisiones propuestas o tomadas
- Tomadas por Gustavo (#76): marca única BrambiLab, POS heredado en solo lectura como primer caso, ROVER independiente, web sin cambios y reparto de roles.
- Propuestas: núcleo comercial en el repositorio privado `brambilab-core`; capacidades como módulos de un solo proceso en el MVP; ADRs posteriores sin número reservado (stack, contrato de conectores, transporte MCP, identidad y aislamiento, ejecución local); herramientas iniciales del laboratorio en RF-02.

## Próxima acción concreta
Revisión del PR por ChatGPT y aprobación de Gustavo. Después, una tarea nueva: especificación técnica del laboratorio POS de solo lectura (etapa 1), con el ADR del contrato de conectores y el de stack.

## Material para revisión
PR desde `docs/FND-V2-001-enterprise-ai-foundation` vinculado a #76. Sin credenciales ni datos de clientes.
