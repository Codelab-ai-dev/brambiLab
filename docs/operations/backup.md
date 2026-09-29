# Backup y restauración

Estado: **implementado y probado en local y en CI contra un destino simulado. Sin destino real todavía**: el servicio `backup` queda inactivo, y lo dice en su log, hasta que Gustavo elija el destino y la custodia de la clave. Especificación: [web-v1.md §15.2](../architecture/web-v1.md). Gates: [lanzamiento.md](lanzamiento.md).

## Qué se respalda y cómo
Cada día a la hora `BACKUP_TIME_UTC` (09:00 UTC = 03:00 en Ciudad de México), el servicio `backup` ejecuta `/ops/backup.sh`:
1. **Registro:** inserta una fila en `ops_backup_runs`, sólo una ejecución a la vez. Una ejecución colgada más de 3 h se cierra como fallida.
2. **Mantenimiento:** lo activa y espera a que la API confirme que no hay nada en curso (`BACKUP_ACK_TIMEOUT`, 120 s por defecto).
   - Si la API no responde, el backup falla y el mantenimiento se desactiva.
   - Si ya había un mantenimiento de otra persona, no lo toca y falla.
3. **Captura:** `pg_dump -Fc`, `tar` de `/data/media/objects` y un manifiesto `media.sha256` con el hash de cada objeto.
   - Comprueba que cada asset `ready` de la base tenga sus bytes con el mismo SHA-256; si falta uno, falla **sin subir nada**.
4. **Fin del mantenimiento:** se sale en cuanto los datos están capturados. Suele durar segundos; en la prueba local, 1 s con 73 archivos.
5. **Empaquetado:** `manifest.json` (fecha UTC, commit, versión de esquema, versión de `pg_dump` y hashes y tamaños de cada parte) más `db.dump`, `media.tar` y `media.sha256`, en un solo `.tar`.
6. **Cifrado:** GnuPG simétrico AES-256 con protección de integridad (`.tar.gpg`) y un `.sha256` aparte.
7. **Verificación local:** descifrar y comparar el hash, y `pg_restore -l`.
8. **Subida** con rclone y **verificación remota**: `rclone check --download` vuelve a descargar la copia y la compara.
9. **Retención**, sólo después de verificar la copia nueva: la más reciente de cada uno de los últimos 7 días, 4 semanas ISO y 3 meses.
10. **Cierre:** `succeeded` con bytes, número de assets y esquema. Cualquier fallo queda `failed`, con el paso en que ocurrió.

El código de salida es 0 sólo si la copia nueva quedó verificada fuera del VPS. Un archivo de tamaño mayor que cero no se considera éxito.

## Configuración (con autorización)
**Variables del servicio `backup` en Coolify:**
- `BACKUP_REMOTE`: por ejemplo `dest:brambilab-backups/prod`.
- `BACKUP_PASSPHRASE`: **secreta**.
- `RCLONE_CONFIG_DEST_TYPE`, `…_PROVIDER`, `…_ENDPOINT`, `…_REGION`, `…_ACCESS_KEY_ID` y `…_SECRET_ACCESS_KEY` (**secreta**).
- Opcionales: `BACKUP_TIME_UTC`, `BACKUP_KEEP_DAILY`, `BACKUP_KEEP_WEEKLY`, `BACKUP_KEEP_MONTHLY` y `BACKUP_COMMIT`.

**Destino (por decidir):** un bucket compatible con S3 fuera del VPS, por ejemplo Backblaze B2, Cloudflare R2 o Hostinger Object Storage si existe en tu cuenta.
- Usa una clave de acceso limitada a ese bucket: escritura, lectura, listado y borrado.
- Activa la retención o el versionado del proveedor si quieres protección contra borrados. Ten en cuenta que eso alarga la vida de los mensajes de contacto en las copias (ver Privacidad).
- **No se contrata nada** sin decisión de Gustavo.

**Clave de cifrado:**
- Genérala con, por ejemplo, `openssl rand -base64 32`.
- Guárdala **fuera del VPS**, en un gestor de contraseñas de Gustavo, además de en el secreto de Coolify.
- Sin ella las copias no se pueden restaurar. Si se pierde, las copias anteriores no sirven.

**Primera verificación:**
1. `docker exec <backup> /ops/backup.sh` y comprobar que termina en `backup succeeded`.
2. Ejecutar el simulacro de restauración de abajo.

## Seguimiento
```sql
SELECT started_at, finished_at - started_at AS duracion, status, step, bytes, assets, schema_version, error
FROM ops_backup_runs ORDER BY id DESC LIMIT 10;
```
Hay una alerta pendiente (entrega 3 de WEB-008): el último `succeeded` tiene más de 26 h o el último run es `failed`.

## Simulacro de restauración aislada (sin tocar producción)
```sh
cd platform
RESTORE_PASSPHRASE=… \
RESTORE_REMOTE=dest:brambilab-backups/prod RESTORE_RCLONE_TYPE=s3 … \
ops/restore-test.sh latest        # KEEP=1 lo deja arriba en http://localhost:8100
```
- Levanta un proyecto Compose **distinto** (`brambilab-restore`), con volúmenes nuevos y puerto 8100, y se niega a usar el nombre del proyecto de producción.
- `restore.sh` descarga la copia, verifica su `.sha256`, la descifra (falla si fue modificada o si la clave no es correcta) y verifica los hashes del manifiesto.
- **Se niega a restaurar sobre una base con tablas** o un volumen con objetos. Deja los medios con el dueño de la API (65532).
- **Arranque seguro:** con `BACKGROUND_JOBS=off`, el contacto desactivado y el GitHub simulado para el login.
  - Las sesiones restauradas se **revocan** antes de abrir.
  - El mantenimiento vuelve activo, porque la copia se hizo en mantenimiento, así que las escrituras quedan bloqueadas.
- **Comprobaciones:** readiness, el sitemap con todas las traducciones visibles, una página publicada, la búsqueda, un medio público y el login. Mide el RTO.
- **Resultado local** (2026-09-28): RTO de 25 s con 179 traducciones visibles y 73 assets. Es una medida de portátil, no del VPS.

## Restauración real (recuperación ante desastre)
Requiere un plan explícito y **autorización de Gustavo antes de reemplazar una base existente**:
1. **Elegir la copia:** fijar el punto de recuperación y la pérdida aceptada: todo lo escrito después de esa copia se pierde.
2. **Restaurar en volúmenes nuevos**, como en el simulacro, con el `PUBLIC_ORIGIN` real pero **sin DNS** hasta decidir. Con el mantenimiento activo y `BACKGROUND_JOBS=off`.
3. **Conciliar antes de reactivar las tareas:**
   - **Contacto:** revisa `/admin/contacto` con los estados pendiente, en reintento, en curso e incierto. Un mensaje que fue aceptado por Resend **después** de la copia aparecerá como pendiente. Búscalo en Resend por hora y asunto. Si ya salió, no lo reenvíes; márcalo con SQL o déjalo sin reactivar el envío hasta decidir.
   - **Ventana de idempotencia:** los reintentos con la misma clave sólo son seguros dentro de las 23 h desde el primer intento.
   - **Publicaciones programadas:** revisa las vencidas (`SELECT … FROM publication_jobs WHERE status = 'scheduled' AND run_at < now()`). Decide cuáles publicar, cancelar o reprogramar desde el panel. Con las tareas desactivadas, nada se publica solo.
   - **Sesiones:** revocadas; el propietario vuelve a iniciar sesión.
4. **Reactivar:** quita `BACKGROUND_JOBS=off`, desactiva el mantenimiento (`UPDATE app_maintenance SET active = false, reason = ''`), comprueba el sitio y, sólo entonces, apunta el dominio o el servicio.

## Privacidad
- Las copias contienen los mensajes de contacto vigentes el día de la copia, que la aplicación purga a los 30 días.
- Con la retención propuesta (hasta 3 meses), un mensaje puede sobrevivir en copias unos 4 meses.
- **Tras restaurar**, la purga de retención de la aplicación vuelve a borrar lo que tenga más de 30 días en cuanto se reactiven las tareas.
- Documenta esta ventana en el aviso de privacidad antes del lanzamiento, o reduce la retención mensual.
