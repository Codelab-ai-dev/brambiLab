# Gates de lanzamiento (WEB-008, #37)

Registro canónico. Cada gate queda como **aprobado**, **fallido**, **bloqueado** o **pendiente**, con fecha y evidencia. Una prueba local no sustituye la evidencia del VPS.

| # | Gate | Estado | Fecha | Evidencia / motivo |
|---|---|---|---|---|
| 1 | Recursos del VPS, versiones de Coolify y Docker, dominio y red registrados (sin secretos) | bloqueado | 2026-09-28 | Sin acceso al VPS ni al panel de Coolify en esta sesión. |
| 2 | Configuración productiva revisada, reproducible y sin componentes de prueba | parcial | 2026-09-28 | `compose.yaml` pasa sólo las variables reales; `server preflight` rechaza valores de prueba y localhost. Falta ejecutarlo con la configuración real. |
| 3 | Release, migraciones y reversión ensayadas en el VPS, con el commit desplegado registrado | bloqueado | 2026-09-28 | Runbook listo ([release.md](release.md)); sin acceso ni autorización de despliegue. |
| 4 | DB y medios persisten tras recrear; red, TLS, cookies y proxies verificados | pendiente | — | Localmente, `--force-recreate` conserva los volúmenes. Falta verificarlo en el VPS. |
| 5 | Destino, política, RPO y RTO aprobados; backup cifrado externo automatizado con fallo detectable | bloqueado | 2026-09-28 | Implementado y probado contra un destino simulado ([backup.md](backup.md), 11 casos de fallo en CI). Falta la decisión de Gustavo sobre destino, presupuesto, custodia de la clave y RPO/RTO. |
| 6 | Restauración desde la copia externa en un entorno aislado, con hashes y referencias válidos y RTO medido | parcial | 2026-09-28 | Simulacro local desde el destino simulado: OK, RTO de 25 s con 179 traducciones y 73 assets. Falta repetirlo desde el destino real y en el VPS. |
| 7 | Recuperación de jobs, sesiones y purgas documentada sin efectos externos | aprobado (local) | 2026-09-28 | [backup.md §Restauración real](backup.md): tareas desactivadas, sesiones revocadas, conciliación de contacto y publicaciones; el simulacro lo comprueba. |
| 8 | Recorrido integral y compatibilidad (Firefox, Safari, lector de pantalla, zoom al 200 %, móvil, H.264/AAC) | pendiente | — | Sólo Chromium automatizado hasta ahora. |
| 9 | Resend real configurado y prueba autorizada con evidencia | bloqueado | 2026-09-28 | Contacto desactivado; faltan dominio, clave y autorización ([contacto-resend.md](contacto-resend.md)). |
| 10 | Alertas, runbooks y custodia de secretos documentados | parcial | 2026-09-28 | Runbook de release y mantenimiento; faltan alertas y runbooks de incidentes. |
| 11 | Autorización concreta de lanzamiento registrada | pendiente | — | — |
