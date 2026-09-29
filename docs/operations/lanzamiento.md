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
| 10 | Alertas, runbooks y custodia de secretos documentados | parcial | 2026-09-28 | Chequeos, `ops-check`, página «Operación», aviso por webhook (desactivado) y [incidentes.md](incidentes.md). Falta aprobar el canal de avisos y el monitor externo de caída. |
| 11 | Autorización concreta de lanzamiento registrada | pendiente | — | — |

## Medición de carga (entorno local, no el VPS)
2026-09-28 · portátil de desarrollo (Colima, 3,8 GiB para Docker) · stack e2e · `go run ./cmd/loadprobe -c 8 -d 20s -media <id> -q filtro -contact`

| Endpoint | Peticiones | Errores | p50 | p95 | p99 |
|---|---|---|---|---|---|
| SSR inicio | 2407 | 0 | 23 ms | 43 ms | 72 ms |
| SSR listado | 2405 | 0 | 16 ms | 31 ms | 48 ms |
| Búsqueda API | 2402 | 0 | 6 ms | 13 ms | 17 ms |
| Sitemap | 2403 | 0 | 11 ms | 19 ms | 28 ms |
| Medio (Range 64 KiB) | 2404 | 0 | 2 ms | 6 ms | 10 ms |
| Contacto (simulador) | 2405 | 0 (2305 limitados por la cuota global) | 2 ms | 5 ms | 8 ms |

- **Consumo en pico con 8 clientes:** web 67 % de CPU y 233 MiB; postgres 62 % y 83 MiB; api 17 %; proxy 12 %.
- **Límite del resultado:** no dice nada de la capacidad del VPS. Hay que repetirlo allí con autorización (`-allow-remote`) y en un horario sin tráfico.

## Lista de comprobación del lanzamiento
Cada punto se marca con fecha, quién y evidencia. Sin evidencia, queda pendiente.

**Entorno real (tras autorización):**
- [ ] DNS y TLS del dominio; `http→https` y `/` → `/es`; `PUBLIC_ORIGIN` correcto en `api` y `web`.
- [ ] Ningún puerto interno público: desde fuera, sólo 443 y 80 (probar `nc` o `nmap` contra el VPS).
- [ ] `server preflight` sin `FAIL`; `server ops-check` en 0.
- [ ] Login y logout del propietario; otra cuenta rechazada; cookie `__Host-` Secure, HttpOnly y SameSite=Lax.
- [ ] Persistencia: recrear contenedores y comprobar que siguen los contenidos y los medios.
- [ ] Límites en toda la cadena real: subida de un MP4 de 250 MiB; 413 por encima.
- [ ] IP efectiva: `TRUSTED_PROXIES` fijado a la subred real; una cabecera falsificada no escapa de la cuota; dos clientes reales no comparten cuota.
- [ ] SSR sin JS; canonical, hreflang, OG y sitemap con el dominio real (sin localhost ni rutas internas); ningún fixture de prueba público.
- [ ] Backup real correcto y simulacro de restauración desde el destino real, con el RTO medido.
- [ ] Canal de avisos aprobado y probado con un problema simulado (por ejemplo, un backup fallido provocado).
- [ ] Monitor externo de caída configurado.
- [ ] Resend: la prueba real autorizada de [contacto-resend.md](contacto-resend.md) §4.
- [ ] Revisión consciente de la bio, los enlaces y el texto «En construcción» del hero.

**Compatibilidad** (dispositivo, navegador y versión como evidencia):
- [ ] Firefox (escritorio).
- [ ] Safari (macOS o iOS), incluido un MP4 H.264/AAC.
- [ ] Móvil real (Android o iOS).
- [ ] Teclado completo y lector de pantalla (VoiceOver o NVDA) en inicio, ficha, búsqueda, contacto y panel.
- [ ] Zoom al 200 % sin pérdida de contenido.

**Autorización:**
- [ ] Autorización concreta de lanzamiento de Gustavo (fecha, alcance).
