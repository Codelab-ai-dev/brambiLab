# Arquitectura BL-001
Propuesta, sujeta al inventario.

| Componente | Responsabilidad | Pendiente |
|---|---|---|
| CMF Phone 1 | Cámara, interfaz e inferencia experimental | Confirmar dispositivo, montaje y runtime |
| ESP32 | Control y parada por timeout | Identificar variante, pines y firmware |
| Driver y motores | Actuación | Identificar modelos, tensión y corriente |
| Alimentación | Energía por subsistema | Batería, protecciones y regulación |
| Pan/tilt | Orientación de cámara | Geometría, servos, carga y alimentación |

## Contrato de control por especificar
Comando de movimiento, límites, secuencia, caducidad, confirmación y parada explícita. Definir frecuencia y timeout tras pruebas; no fijar cifras sin evaluar el enlace.

## Fallos a cubrir
Pérdida de enlace, app detenida, reinicio ESP32, comando inválido y alimentación inestable. El firmware debe arrancar parado y dejar de accionar al caducar comandos. La prueba inicial será con ruedas sin contacto con el suelo.

## Límite de v1
Sin navegación autónoma ni integración ROS. La arquitectura final de cómputo se decidirá cuando existan necesidades medidas.
