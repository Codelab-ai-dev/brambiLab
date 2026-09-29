# Gates de lanzamiento (WEB-008, #37)

**WEB-008 cerrada el 2026-09-28 con excepciones aceptadas por Gustavo** (ver al final). Los pendientes siguen en [#43](https://github.com/Codelab-ai-dev/brambiLab/issues/43).

Registro canónico. Cada gate queda como **aprobado**, **fallido**, **bloqueado** o **pendiente**, con fecha y evidencia. Una prueba local no sustituye la evidencia del VPS.

| # | Gate | Estado | Fecha | Evidencia / motivo |
|---|---|---|---|---|
| 1 | Recursos del VPS, versiones de Coolify y Docker, dominio y red registrados (sin secretos) | parcial · excepción | 2026-09-28 | Dominio `brambilab.dev` → `2.25.251.183` (VPS Hostinger con Coolify). Sin datos de CPU, RAM, disco ni versiones ([#43](https://github.com/Codelab-ai-dev/brambiLab/issues/43)). |
| 2 | Configuración productiva revisada, reproducible y sin componentes de prueba | parcial | 2026-09-28 | `compose.yaml` pasa sólo las variables reales; `server preflight` rechaza valores de prueba y localhost. Falta ejecutarlo con la configuración real. |
| 3 | Release, migraciones y reversión ensayadas en el VPS, con el commit desplegado registrado | parcial · excepción | 2026-09-28 | Desplegado por Gustavo; `/api/v1/health/ready` 200 y #41 visible en producción. Sin commit registrado ni ensayo de reversión ([#43](https://github.com/Codelab-ai-dev/brambiLab/issues/43)). |
| 4 | DB y medios persisten tras recrear; red, TLS, cookies y proxies verificados | parcial · excepción | 2026-09-28 | TLS, redirecciones, cabeceras y puertos verificados desde fuera (ver «Evidencia de producción»). Faltan la persistencia tras recrear, la cookie tras iniciar sesión y `TRUSTED_PROXIES` real ([#43](https://github.com/Codelab-ai-dev/brambiLab/issues/43)). |
| 5 | Destino, política, RPO y RTO aprobados; backup cifrado externo automatizado con fallo detectable | bloqueado | 2026-09-28 | Implementado y probado contra un destino simulado ([backup.md](backup.md), 11 casos de fallo en CI). Falta la decisión de Gustavo sobre destino, presupuesto, custodia de la clave y RPO/RTO. |
| 6 | Restauración desde la copia externa en un entorno aislado, con hashes y referencias válidos y RTO medido | parcial | 2026-09-28 | Simulacro local desde el destino simulado: OK, RTO de 25 s con 179 traducciones y 73 assets. Falta repetirlo desde el destino real y en el VPS. |
| 7 | Recuperación de jobs, sesiones y purgas documentada sin efectos externos | aprobado (local) | 2026-09-28 | [backup.md §Restauración real](backup.md): tareas desactivadas, sesiones revocadas, conciliación de contacto y publicaciones; el simulacro lo comprueba. |
| 8 | Recorrido integral y compatibilidad (Firefox, Safari, lector de pantalla, zoom al 200 %, móvil, H.264/AAC) | pendiente | — | Sólo Chromium automatizado hasta ahora. |
| 9 | Resend real configurado y prueba autorizada con evidencia | bloqueado | 2026-09-28 | Contacto desactivado; faltan dominio, clave y autorización ([contacto-resend.md](contacto-resend.md)). |
| 10 | Alertas, runbooks y custodia de secretos documentados | parcial | 2026-09-28 | Chequeos, `ops-check`, página «Operación», aviso por webhook (desactivado) y [incidentes.md](incidentes.md). Falta aprobar el canal de avisos y el monitor externo de caída. |
| 11 | Autorización concreta de lanzamiento registrada | aprobado | 2026-09-28 | Gustavo desplegó el sitio y decidió cerrar WEB-008 con las excepciones de abajo. |

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

## Evidencia de producción (2026-09-28, desde fuera, sólo lectura)
Comprobado con `curl`, `openssl` y `nc` contra `https://brambilab.dev`. No se enviaron formularios ni escrituras.

**TLS y respuestas:**
- **TLS:** Let's Encrypt para `brambilab.dev`, válido hasta el 2026-12-28.
- **Redirecciones:** `http://` → `https://` (302); `/` → `/es`.
- **Páginas públicas:** `/es`, `/en`, las secciones y `/es/proyectos/rover` dan 200 con `Cache-Control: no-store`. `/es/buscar` lleva `X-Robots-Tag: noindex` y una página inexistente da 404.
- **Panel:** `/admin` redirige al login, con `no-store, private` y `noindex, nofollow`.

**SEO:**
- HTML con `<html lang="es">`, canonical, hreflang es/en y `og:url` sobre `https://brambilab.dev`.
- Sin `localhost`, direcciones internas ni datos de prueba en el HTML ni en el sitemap.
- `robots.txt` excluye `/admin` y `/api/` y apunta a `https://brambilab.dev/sitemap.xml`. El sitemap tiene 11 URLs canónicas.

**Login y contacto:**
- Login activado; el inicio de OAuth usa `redirect_uri=https://brambilab.dev/api/v1/auth/github/callback` con PKCE (S256).
- `GET /api/v1/public/contact` responde `available:false`, lo esperado mientras no se configure Resend.

**Medios y #41:**
- `/media/{id}`: `image/jpeg`, `Content-Security-Policy: default-src 'none'; sandbox`, `Cross-Origin-Resource-Policy: same-origin`.
- #41 desplegado: la foto del rover aparece como tarjeta compacta.

**Puertos:**
- Al empezar la revisión, **8000, 6001 y 6002 de Coolify estaban abiertos al público**, con el panel en `:8000` por HTTP.
- Gustavo activó el firewall de Hostinger, que acepta sólo TCP 22, 80 y 443 y descarta el resto.
- **Comprobado después:** 22, 80 y 443 abiertos; 3000, 5432, 6001, 6002, 8000 y 8080 cerrados. El sitio siguió respondiendo.

**Coolify (pendiente en #43):** el acceso desde el hPanel usa el `:8000`. `coolify.brambilab.dev` resuelve y tiene certificado, pero hoy sirve el sitio de BrambiLab. Hay que decidir entre túnel SSH, 8000 limitado a una IP o dominio propio.

## Excepciones aceptadas al cierre (2026-09-28, Gustavo)
WEB-008 se cierra sin estos puntos, trasladados a [#43](https://github.com/Codelab-ai-dev/brambiLab/issues/43):
1. **Backup externo sin destino ni clave:** hoy no hay copias fuera del VPS y no se hizo el simulacro con una copia real (gates 5 y 6). Es el riesgo principal.
2. **Resend sin configurar:** el contacto sigue desactivado y no hay prueba real (gate 9).
3. **Preflight y ops-check de producción** sin salida registrada; commit desplegado y datos del VPS sin registrar (gates 1–3).
4. **Persistencia tras recrear**, límites de subida en la cadena real, cookie y `TRUSTED_PROXIES` sin verificar en el VPS (gate 4).
5. **Canal de avisos y monitor externo de caída** sin configurar (gate 10).
6. **Compatibilidad sin probar:** Firefox, Safari (incluido H.264/AAC), móvil real, lector de pantalla y zoom (gate 8).
7. **Acceso a Coolify** pendiente de decidir y `coolify.brambilab.dev` sirviendo el sitio.
