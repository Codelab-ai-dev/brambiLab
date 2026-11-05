# libraries

Librerías propias compartidas por los firmwares del rover. Se pasan a
`arduino-cli compile` con `--libraries projects/001-ai-rover/firmware/libraries`.

- [`RoverDriveLogic`](RoverDriveLogic/src/drive_logic.h): cinemática de `Twist`,
  saturación, mapa de duty, armado (`SafetyGate`) y cuarentena (`ArmGuard`).
  Sin dependencias de Arduino; pruebas en `RoverDriveLogic/test/`:

  ```sh
  cd RoverDriveLogic/test
  c++ -std=c++17 -Wall -Wextra -Werror -I../src test_drive_logic.cpp -o /tmp/t && /tmp/t
  ```

  La usan [`rover_motors_uros`](../rover_motors_uros/README.md) (ESP32, #69) y
  [`rover_motors_unoq`](../rover_motors_unoq/README.md) (STM32 del UNO Q, #71).
