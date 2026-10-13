#pragma once

void motors_begin();                    // deja los dos lados parados
void motors_set(int left, int right);   // -255..255 por lado
void motors_stop();
void motors_get(int *left, int *right);
void motors_watchdog();                 // para si caduco el ultimo comando
unsigned long motors_last_command_age();
