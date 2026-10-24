# Bitácora BL-001
## 2026-09-27 — Foundation
Objetivo: formalizar el proyecto y el proceso de documentación.
Realizado: estructura, alcance, inventario pendiente y tareas de validación.
Evidencia: documentos de esta entrega; sin evidencia física nueva.
Aprendizaje: separar lo diseñado de lo fabricado/probado evita presentar capacidades inexistentes.
Siguiente: completar BL-001-001 con fotografías, referencias y prueba del estado actual.

## 2026-10-09 — Inventario del ESP32-CAM y firmware de conducción
Objetivo: saber qué contenía la placa y pasar el control del rover al ESP32-CAM
con el L298N, dejando fuera el Arduino UNO Q.

Realizado:
- Lectura del chip por `/dev/cu.usbserial-310`: ESP32-D0WD-V3 rev3.1, MAC
  `3c:8a:1f:c2:40:d4`, flash 4 MB, OV2640. Llevaba el ejemplo `CameraWebServer`
  sin tocar, **sin control de motores**, con SSID y contraseña en claro.
- Copia íntegra del flash en `firmware/backup/`, sha256 `14c42b1b…b84d24b`.
- Firmware nuevo en `firmware/rover_esp32cam/`: cámara, interfaz de conducción,
  mezcla diferencial, timeout de 500 ms y parada al caer el WiFi.
- Pinout tomado del instructivo del kit TSCINBUNY (IO2/IO14/IO15/IO13 → IN1..IN4).

Evidencia: volcado del flash, log de arranque capturado por serie, salida de
`arduino-cli compile` (1.059.537 bytes, 33 % de `huge_app`).

Aprendizaje: la cámara genera su XCLK con LEDC_TIMER_0 por fuera del gestor de
canales de Arduino, así que el PWM de los motores tiene que pedir timer y canal
a mano. `analogWrite()` habría podido quedarse con el timer 0 y tumbar la imagen.

Siguiente: cablear el L298N, cargar el firmware y ejecutar las pruebas de
[validation.md](validation.md) con las ruedas en el aire. Queda por verificar
qué lado es cuál y si los motores arrancan por debajo de `MOTOR_MIN_DUTY`.

## 2026-10-09 — Primera conducción en el suelo (registrada el 2026-10-13)
Objetivo: comprobar el firmware `rover_esp32cam` con los motores alimentados.

Realizado: Gustavo condujo el rover en el suelo con una batería de litio desde
la página de control. Avance, retroceso y giros correctos sin invertir ningún
lado; para al soltar y con STOP; sin reinicios al acelerar; con la velocidad
mínima del deslizador las ruedas giran.

Evidencia: informe verbal de Gustavo; sin vídeo ni fotos.

Aprendizaje: el pinout del instructivo TSCINBUNY y los sentidos por defecto
coinciden con el cableado real.

Siguiente: grabar vídeo de una prueba completa (arranque, conducción, caída del
WiFi), anotar la batería y la alimentación del ESP32, e instalar el interruptor
de potencia.

## 2026-10-23 — App Android, fase 1: conexión y vídeo (ROVER-MOBILE-001)
Objetivo: primera versión de ROVER Control en Flutter que valide el rover por
`/status` y muestre el vídeo de `:81/stream`, sin tocar el firmware ([#58](https://github.com/Codelab-ai-dev/brambiLab/issues/58)).

Realizado: proyecto Android en [`mobile/`](mobile/README.md) con pantalla de
conexión (host guardado), vídeo MJPEG decodificado en la app, estados de
conexión, error y reconexión, y cierre del stream al salir o pasar a segundo
plano. Permisos y HTTP sin TLS documentados.

Evidencia: `flutter analyze` sin avisos, 48 pruebas automáticas (incluida una
contra un socket local que comprueba que el servidor ve el cierre al cancelar),
APK debug y release compilados. Sin teléfono conectado: no hay prueba en
dispositivo ni contra el rover.

Aprendizaje: el servidor de vídeo del ESP32 atiende un solo cliente, así que
la app y el panel web se excluyen mutuamente; por eso la app suelta el stream
siempre que no está a la vista y, si no recibe vídeo pero `/status` responde,
lo explica.

Siguiente: ejecutar las pruebas de [mobile/README.md](mobile/README.md#probar-en-un-teléfono-android-real)
en un teléfono real; después, issue de control de motores.

## 2026-10-23 — App Android en el teléfono real (ROVER-MOBILE-001, ronda 1)
Objetivo: probar la app de [#58](https://github.com/Codelab-ai-dev/brambiLab/issues/58)
en el CMF Phone 1 contra el rover.

Realizado: instalada por ADB inalámbrico en el Nothing A015 (Android 16).
Primero no había vídeo por tres causas ajenas a la app, en este orden: el
teléfono estaba en la red de 5 GHz; el cableado del rover estaba mal tras los
intentos de carga (un lado de ruedas girando, corregido por Gustavo); y la
antena IPEX del ESP32-CAM estaba suelta (−80 a −88 dBm, 1–7 kB/s). Con todo
corregido: vídeo a 10–16 fps con −38 a −44 dBm.

La prueba destapó fallos de la app, corregidos en un PR de seguimiento:
`rover.local` resolvía para `/status` y no para el vídeo; la pista de «un solo
stream» salía en casos que no correspondían; la primera imagen tenía solo 4 s;
el RSSI no se actualizaba; y la cámara va girada 90°, así que se añadió giro y
espejo guardados en el teléfono.

Intento de recargar el firmware sin éxito: la placa arranca en modo normal
(`boot:0x13`) en vez de entrar en modo de carga. El firmware actual sigue igual.

Evidencia: capturas de pantalla del teléfono por ADB y medidas de `/status` y
`/capture` desde el Mac, sin versionar.

Aprendizaje: en un ESP32-CAM con conector IPEX, una señal muy baja con el router
cerca apunta a la antena antes que a la distancia.

Siguiente: pruebas AC 3–5 (Wi-Fi caído, segundo plano y panel web) y revisar la
placa MB para poder cargar firmware.
