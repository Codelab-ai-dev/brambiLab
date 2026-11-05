#pragma once

#include <drive_logic.h>  // firmware/libraries/RoverDriveLogic

void motors_begin();                         // pines en bajo antes que nada
void motors_apply(const rover::Wheels &w);   // -255..255 por lado
