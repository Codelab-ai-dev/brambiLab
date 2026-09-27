# Instrucciones compartidas para agentes
## Inicio de sesión
Lee README.md, PROJECT.md, ARCHITECTURE.md, DECISIONS.md y tasks/current.md. Lee el alcance y aceptación de la tarea. Inspecciona el estado Git antes de editar y conserva cambios ajenos.

## Ejecución
- Trabaja una tarea acotada por rama; evita ediciones simultáneas sobre los mismos archivos. Con sesiones concurrentes usa ramas/worktrees separados.
- Identifica hechos confirmados, propuestas y pendientes. No inventes hardware, resultados, precios ni pruebas.
- Implementa y verifica lo que permita el entorno. Las pruebas físicas requieren evidencia de Gustavo; no las marques como aprobadas por simulación.
- No atribuyas a otro agente acceso automático a esta conversación o al repositorio. Entrega contexto mediante archivos, diff o PR.
- Mantén documentación canónica junto al proyecto; enlaza en lugar de duplicar.
- Registra decisiones transversales en DECISIONS.md y un ADR cuando haya alternativas relevantes.
- No incluyas credenciales, datos privados ni binarios grandes en commits.
- No reescribas historial ni sobrescribas trabajo ajeno sin instrucción expresa. Publicar, desplegar o comprar requiere una instrucción que lo autorice.

## Cierre
Reporta archivos modificados, pruebas realmente ejecutadas y resultado, pendientes y siguiente tarea. Actualiza tasks/current.md y deja un handoff con docs/templates/handoff.md. Una tarea se termina cuando satisface sus criterios o se reporta explícitamente bloqueada; crear documentación no equivale a validar hardware.

## Convenciones
Español para documentación inicial. Rutas e identificadores en inglés. Commits: tipo(alcance): descripción; tipos docs, feat, fix, refactor, test, chore. Ramas: docs/BL-001-001-inventory o feat/BL-001-003-control. PR pequeño con contexto, cambios, validación y límites.
