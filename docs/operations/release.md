# Release y operación en Coolify

Estado: **preparado, no desplegado.** Ningún paso de esta guía se ha ejecutado en el VPS. Todo lo marcado como «verificar» depende de la versión de Coolify instalada y del VPS real (Hostinger KVM 2), y queda pendiente de acceso y autorización de Gustavo. Especificación: [web-v1.md §14–§15.1](../architecture/web-v1.md). Contacto: [contacto-resend.md](contacto-resend.md).

## 1. Topología
Visitante → TLS en el proxy de Coolify (Traefik) → `proxy` (Caddy :8000) → `web` (SSR :3000) o `api` (Go :8080) → `postgres`.

| Servicio | Red | Puertos públicos | Volumen |
|---|---|---|---|
| `proxy` | public | **sólo** el dominio de Coolify (no publicar puertos en el host) | — |
| `web` | public | ninguno | — |
| `api` | internal, public | ninguno | `media_data` → `/data/media` |
| `postgres` | internal (sin salida) | ninguno | `postgres_data` → `/var/lib/postgresql` |
| `migrate` | internal | ninguno | — (tarea única antes de `api`) |

- **Dominio:** se asocia **sólo** al servicio `proxy`.
- **Verificar:** que Coolify no publique el `127.0.0.1:${PUBLIC_HOST_PORT}:8000` de `compose.yaml` en el host. Si lo hace, sólo es accesible desde el propio VPS, pero conviene eliminarlo en la configuración de Coolify.
- **Verificar:** los nombres efectivos de los volúmenes. Coolify les añade un prefijo de proyecto; anótalos para el backup.

## 2. Variables (lo que el código lee realmente)
Secretos sólo en Coolify, nunca en Git ni en chats.

| Variable | Servicio | Obligatoria | Secreta | Notas |
|---|---|---|---|---|
| `POSTGRES_DB`, `POSTGRES_USER` | postgres, migrate, api | sí | no | Compose las pasa como `PG*`. |
| `POSTGRES_PASSWORD` | postgres, migrate, api | sí | **sí** | 16 o más caracteres aleatorios, sin `$`. |
| `PUBLIC_ORIGIN` | api, web | sí | no | `https://<dominio>`, idéntico en ambos. |
| `ADMIN_GITHUB_USER_ID` | api | sí | no | ID numérico del propietario (no el login). |
| `GITHUB_CLIENT_ID` | api | sí | no | OAuth App con callback `${PUBLIC_ORIGIN}/api/v1/auth/github/callback`. |
| `GITHUB_CLIENT_SECRET` | api | sí | **sí** | |
| `SESSION_TTL` | api | no | no | 12 h por defecto; entre 5m y 168h. |
| `TRUSTED_PROXIES` | api | recomendada | no | Subred Docker del proxy de Coolify y de Caddy (ver §6). Vacía: se confía en todos los rangos privados. |
| `CONTACT_ENABLED`, `RESEND_API_KEY` (**secreta**), `CONTACT_FROM`, `CONTACT_TO` | api | no | — | Ver [contacto-resend.md](contacto-resend.md). `false` por defecto. |
| `CONTACT_RATE_PER_CLIENT`, `CONTACT_RATE_GLOBAL`, `CONTACT_RETENTION_DAYS` | api | no | no | 5 / 100 / 30 por defecto. |
| `MEDIA_MAX_IMAGE_BYTES`, `MEDIA_MAX_VIDEO_BYTES`, `MEDIA_MAX_RESOURCE_BYTES`, `MEDIA_FREE_RESERVE_BYTES` | api | no | no | 20 MiB / 250 MiB / 100 MiB / 512 MiB. Subir el de vídeo por encima de 256 MiB exige cambiar también Caddy. |
| `BACKGROUND_JOBS` | api | no | no | **Nunca `off` en producción**: sólo en restauraciones aisladas. |
| `INTERNAL_API_URL` | web | fija | no | `http://api:8080` (red privada). |

Sólo para pruebas, y **no deben existir** en producción (el preflight las rechaza): `GITHUB_AUTHORIZE_URL`, `GITHUB_TOKEN_URL`, `GITHUB_USER_URL`, `RESEND_API_URL`, `FAKE_*`, `TEST_DATABASE_URL`, `AUTHTEST_LOGS`. Los servicios `fakegithub` y `fakeresend` sólo existen en `compose.e2e.yaml`: nunca se usa ese archivo en Coolify.

## 3. Primer despliegue (con autorización)
1. En Coolify, crea un recurso **Docker Compose** desde el repositorio, rama `main` y archivo `platform/compose.yaml`. **Autodeploy desactivado**: fusionar, desplegar y publicar son operaciones distintas.
2. Configura las variables de §2 y asocia el dominio a `proxy`.
3. Ejecuta el preflight con la imagen nueva antes de arrancarla (§4.2).
4. Despliega. **Verificar** que Coolify respeta `depends_on`: `migrate` debe terminar con éxito y sólo entonces arrancar `api`. Si no lo respeta, ejecuta la migración como tarea previa (`/server migrate`) y bloquea el arranque si falla.
5. Haz las comprobaciones de §4.4 y registra la evidencia en el handoff de WEB-008.

## 4. Cada release
1. **Congelar la versión:** despliega un commit concreto (`git rev-parse HEAD`) y anota ese commit y los digests de las imágenes construidas (`docker image inspect --format '{{.Id}}' <imagen>`).
2. **Preflight** sin imprimir secretos: `docker run --rm --env-file <variables de api> <imagen-api> preflight`, o desde la terminal de Coolify en un contenedor de la imagen nueva: `/server preflight`. Cualquier `FAIL` bloquea la release.
3. **Backup verificado antes de cambios de esquema** (guía de backup, siguiente entrega de WEB-008). Todas las migraciones de v1 (00001–00009) son aditivas: una API anterior sigue funcionando con el esquema nuevo.
4. **Comprobaciones tras desplegar** (sustituye `$O` por `PUBLIC_ORIGIN`):
   ```sh
   curl -fsS $O/api/v1/health/ready            # {"status":"ready"}
   curl -sI $O/ | grep -i '^location: /es'      # 302 a /es
   curl -fsS $O/es | grep -q '<html lang="es"'
   curl -fsS $O/robots.txt | grep -q "Sitemap: $O/sitemap.xml"
   curl -fsS $O/sitemap.xml | grep -c "<loc>$O/"   # sin localhost ni rutas internas
   curl -s $O/api/v1/public/contact             # {"available":false,...} hasta activar Resend
   curl -sI $O/admin | grep -i 'x-robots-tag: noindex'
   ```
   Luego, en el navegador: login como propietario y logout; una cuenta de GitHub distinta debe ser rechazada.

## 5. Reversión
- **Sólo aplicación:** vuelve a desplegar el commit anterior. Es seguro mientras las migraciones sean aditivas, como todas las de v1.
- **Nunca** se ejecutan migraciones `down` en producción ni se hace `docker compose down -v`: `-v` borra los volúmenes.
- **Restaurar datos** exige un plan explícito:
  1. activar el mantenimiento;
  2. elegir el punto de recuperación y la pérdida aceptada;
  3. restaurar en volúmenes nuevos;
  4. **autorización antes** de reemplazar la base existente.
- **Recrear contenedores** conserva los datos: `docker compose up -d --force-recreate` mantiene `postgres_data` y `media_data`. Publicaciones y contacto no duplican efectos al reiniciar (lease, idempotencia y ventana; ver §8.1 y §13.1).

## 6. IP efectiva y proxies
Go toma la primera dirección no confiable desde la derecha de `X-Forwarded-For`. Caddy (`trusted_proxies static private_ranges`) conserva la cabecera sólo si su salto previo es privado (Traefik) y, si no, la reescribe.

**Verificar en el VPS:** la subred de la red Docker que comparten Traefik y `proxy` (`docker network inspect <red> --format '{{range .IPAM.Config}}{{.Subnet}}{{end}}'`). Fija `TRUSTED_PROXIES` a esa subred y a la red interna de Caddy→api, en lugar de todos los rangos privados. Después comprueba:
- **cabecera falsificada:** un `X-Forwarded-For` enviado desde fuera no cambia la cuota; con 6 envíos seguidos llega el 429;
- **clientes distintos:** dos conexiones reales distintas (por ejemplo, la red del móvil y el wifi) no comparten cuota.

## 7. Mantenimiento manual
Actívalo antes de una operación delicada y espera el acuse de la API:
```sql
UPDATE app_maintenance SET active = true, reason = 'motivo', activated_at = now(), acknowledged_at = NULL;
SELECT acknowledged_at >= activated_at AS listo FROM app_maintenance;   -- esperar true
UPDATE app_maintenance SET active = false, reason = '';                -- al terminar
```
Mientras está activo, el sitio se puede leer, pero todo lo que escribe (panel, subidas, formulario) responde 503 y las tareas en segundo plano se detienen.

## 8. Pendiente de datos reales
Queda para anotar en el handoff cuando exista acceso:
- CPU, RAM y disco libre del VPS;
- versiones de Coolify y Docker;
- dominio y DNS;
- subred de proxies;
- OAuth App final;
- destino de backup;
- RPO y RTO acordados.

No se deducen del nombre del plan.
