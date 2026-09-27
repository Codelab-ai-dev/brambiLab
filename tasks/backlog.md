# Backlog inicial
Fuente de estado hasta migrar a GitHub Issues. Prioridad por orden; no son Issues remotos creados.

## FND-001 — Revisar Foundation y primer commit
Estado: done (2026-09-27, commit `332e5f0`; remoto público Codelab-ai-dev/brambiLab). Responsable: Gustavo con agente local.
Aceptación: revisar alcance y convenciones; crear commit local; registrar hash y elegir posteriormente remoto/visibilidad. Evidencia: hash real. No crear historial ficticio.

## BL-001-001 — Inventario y estado inicial V0
Estado: ready. Dependencia: ninguna técnica.
Alcance: completar hardware.md, bom.md y build-log.md con datos observados.
Aceptación: identificar placa, driver, motores y alimentación; documentar control actual; fotos referenciadas; dimensiones de montaje; cada desconocido explícito; descripción de una prueba del estado inicial o bloqueo que impide ejecutarla.
Fuera: comprar componentes, diseñar electrónica nueva, implementar firmware.

## BL-001-002 — Arquitectura del primer control y parada
Estado: backlog. Depende: BL-001-001.
Aceptación: diagrama de conexiones con pinout verificado; límites de potencia sustentados en fichas técnicas; comandos y estados; timeout y parada definidos; plan de prueba con ruedas levantadas. Registrar ADR cuando corresponda.

## BL-001-003 — Control ESP32 mínimo
Estado: backlog. Depende: BL-001-002.
Aceptación: firmware compilable con placa/toolchain registrados; avance, retroceso, giro y parada; arranque parado; rechazo de comandos inválidos; pérdida de enlace lleva a parada dentro del timeout acordado; evidencia de prueba física por Gustavo. Elegir transporte de prueba en la especificación previa.

## BL-001-004 — Documentar primer hito reproducible
Estado: backlog. Depende: BL-001-003.
Aceptación: instrucciones desde cero, versión, cableado, BOM, vídeo o fotos, pruebas, fallos y siguiente paso; diferenciar demostrado de planeado.

## RES-001 — Evaluar enlace Android–ESP32
Estado: backlog. Depende: BL-001-001.
Aceptación: comparar Wi-Fi, BLE y USB cuando sean compatibles con los modelos reales; latencia requerida, recuperación y esfuerzo; fuentes fechadas; proponer ADR-005 sin darlo por aprobado.

## FND-002 — Definir licencias y publicación
Estado: done (2026-09-27, [ADR-007](../docs/architecture/ADR-007-licencias.md)).
Aceptación: Gustavo elige términos para código, documentación y diseños; revisar derechos de recursos de terceros; registrar la decisión antes de etiquetar contenido como open source.

## FND-003 — Especificar web v2
Estado: backlog. Depende: BL-001-004.
Aceptación: mapa de páginas, modelo de contenido y requisitos derivados del primer proyecto; ADR de stack; sin implementar un CMS por anticipado.
