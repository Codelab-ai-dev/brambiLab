# Incidentes: qué hacer

Guías breves para los fallos previstos. Antes de actuar en producción, anota la hora y el síntoma en el handoff. Las acciones que borran datos, restauran o envían correos requieren autorización de Gustavo.

## Cómo se detecta un problema
- **Página «Operación»** del panel (`/admin/operacion`) y **`server ops-check`** (`docker exec <api> /server ops-check`; termina con código 1 si hay algo crítico). Ambos usan los mismos chequeos:

  | Chequeo | Atención | Crítico |
  |---|---|---|
  | Base de datos | — | no responde |
  | Backup | sin ninguna copia correcta registrada | la última falló, o la última correcta tiene más de 26 h |
  | Disco (volumen de medios) | menos de 2 GiB libres | menos de 512 MiB (la API rechaza subidas por debajo de su reserva) |
  | Mantenimiento | activo | activo más de 30 min |
  | Tareas en segundo plano | — | `BACKGROUND_JOBS=off` |
  | Publicaciones programadas | fallidas en 7 días, o vencidas mientras hay pausa | vencidas hace más de 5 min sin pausa (programador atascado) |
  | Envíos de contacto (si está activo) | fallidos o inciertos en 7 días | vencidos hace más de 5 min sin pausa |

  Los umbrales son iniciales, no una medida de capacidad: se ajustan con datos del VPS.
- **Avisos:** la API evalúa los chequeos cada 5 min y los problemas se registran en el log (`ops check`).
  - Sólo si se configura `ALERT_WEBHOOK_URL` (un canal aprobado por Gustavo, por ejemplo un webhook de Discord, Slack o ntfy), envía un aviso cuando aparece un problema nuevo y otro cuando se recupera, nunca en bucle.
  - Sin canal aprobado, no se envía nada.
- **Servicio caído:** la API no puede avisar de su propia caída. Hace falta un monitor externo de `https://<dominio>/api/v1/health/ready`, por ejemplo el de Coolify o un servicio gratuito aprobado. **Pendiente de elegir.**

## Disco lleno o casi lleno
1. `df -h` en el VPS y `docker system df`. Los candidatos habituales son las imágenes antiguas, el volumen `backup_work`, los logs y los medios.
2. **Seguro:** `docker image prune` (imágenes sin usar). Los logs ya rotan a 10 MB × 5 por servicio.
3. **Nunca:** `docker compose down -v`, `docker volume rm` sobre `postgres_data` o `media_data`, ni borrar archivos a mano en `/data/media/objects`. Un asset con referencias no se puede borrar desde el panel, y eso es intencionado.
4. **Si son medios:** revisa en el panel los archivos sin uso y bórralos desde ahí. Si hace falta, amplía el disco.
5. Tras liberar espacio, `server ops-check` debe volver a «ok».

## Backup fallido o vencido
1. Mira `/admin/operacion` o `SELECT … FROM ops_backup_runs ORDER BY id DESC LIMIT 5`. La columna `step` indica dónde falló.
2. **Según el paso:**

   | Paso | Causa probable |
   |---|---|
   | `maintenance` | La API no confirmó (¿está caída?) u otro mantenimiento estaba activo |
   | `check-assets` | Falta el archivo de algún asset `ready`: investiga antes de nada; puede haber pérdida de datos |
   | `upload`, `verify-remote` | Credenciales o destino (rclone). El log del servicio `backup` muestra el error sin secretos |
   | `encrypt` | Configuración de GnuPG o passphrase ausente |

3. **Tras corregir:** `docker exec <backup> /ops/backup.sh`. La retención no borra nada mientras la copia nueva no se haya verificado.
4. **Si el mantenimiento quedó activo** (no debería, porque el script lo desactiva al salir): `UPDATE app_maintenance SET active = false, reason = '' WHERE reason = 'backup'`.

## Login de GitHub roto
1. `curl $O/api/v1/auth/status` y los logs de `api`. Con `503 auth_not_configured`, faltan variables.
2. **Comprueba:**
   - el callback de la OAuth App (`${PUBLIC_ORIGIN}/api/v1/auth/github/callback`, exacto);
   - `ADMIN_GITHUB_USER_ID`, que es el ID numérico y no el login;
   - que `PUBLIC_ORIGIN` sea el mismo en `api` y `web`;
   - la hora del VPS.
3. Si el secreto se filtró o caducó, regenera el client secret en GitHub y actualízalo en Coolify. Las sesiones existentes siguen válidas hasta caducar; revócalas con `DELETE FROM sessions` si hubo compromiso.

## Base de datos no disponible
1. `docker compose ps postgres` y sus logs. Suele ser espacio en disco (ver arriba) o memoria.
2. La API responde 503 en `/health/ready` y el sitio muestra la página «no disponible» (no un falso 404). Las tareas se reanudan solas al volver la base.
3. No reinicialices el volumen. Si está dañado, sigue la restauración de [backup.md](backup.md).

## Migración fallida
1. El servicio `migrate` termina con error y `api` no arranca (`depends_on` con `service_completed_successfully`). **Verificar** que Coolify lo respeta.
2. Lee el error. **No** ejecutes migraciones `down`.
3. Vuelve a desplegar el commit anterior, que funciona con el esquema actual porque las migraciones son aditivas. Corrige la migración en un PR nuevo.
4. Si la migración dejó datos a medias (no debería: cada archivo de goose va en una transacción), valora restaurar la copia previa a la release, con autorización.

## Contacto con «resultado incierto» (`unknown`)
1. En `/admin/contacto?estado=unknown`, abre el mensaje y anota la hora y el asunto (`Contacto BrambiLab #xxxxxxxx`).
2. Búscalo en el panel de Resend y en tu buzón.
3. **Si ya llegó:** no reintentes.
4. **Si no llegó:** «Reintentar con clave nueva (posible duplicado)». Queda auditado.
5. Cambiar las variables `CONTACT_*` no altera los mensajes existentes, porque su payload quedó congelado al recibirlos.

## Mantenimiento que no se desactiva
Aparece como «Mantenimiento» crítico tras 30 min. Comprueba que no haya un backup en curso (`ops_backup_runs` con `status = 'running'`). Si no lo hay: `UPDATE app_maintenance SET active = false, reason = ''`.

## Reversión de una release
Ver [release.md §5](release.md): se vuelve a desplegar el commit anterior, nunca `down -v` ni migraciones `down`. Una restauración de datos requiere plan y autorización.
