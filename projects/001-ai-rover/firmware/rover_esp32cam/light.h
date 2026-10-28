#pragma once

void light_begin();          // apagada
void light_set(int level);   // 0..255, escalado hasta LIGHT_MAX_DUTY
int light_get();             // ultimo nivel pedido, 0..255
