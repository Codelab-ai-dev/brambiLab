# Workflow entre Gustavo y los agentes
1. Gustavo elige objetivo y prioridad.
2. El agente que especifica define contexto, alcance, aceptación y dependencias en una tarea.
3. El agente que implementa lee el estado actual, toma una rama y registra el trabajo.
4. Ejecuta verificaciones adecuadas; separa ejecución local, simulación y prueba física.
5. Entrega diff/PR y handoff con referencia al commit si existe.
6. La revisión compara cambios contra aceptación. Gustavo realiza o valida las pruebas físicas y decide sobre publicación cuando corresponda.
7. Registra aprendizajes y próxima tarea; un hito verificado puede producir contenido público.

## Fuente única y conflictos
Hasta migrar las tareas a GitHub Issues, tasks/backlog.md lleva el estado. Tras migración, los Issues lo llevan y los archivos sólo enlazan. No editar la misma rama desde dos agentes a la vez. Ante conflicto, inspeccionar ambos cambios y preservar la intención de ambos; nunca resolver descartando cambios sin revisión.

## Definición de terminado
Aceptación cumplida, instrucciones reproducibles, evidencia identificable, limitaciones descritas y handoff actualizado. Si falta hardware o acceso, marcar bloqueo y entregar lo verificable.

## Difusión por hito
Guardar objetivo, imagen o vídeo con permiso, resultado medido, limitación y enlace al proyecto. Una sesión puede producir una nota técnica y después un resumen social. No publicar automáticamente ni presentar renders como prototipos fabricados.
