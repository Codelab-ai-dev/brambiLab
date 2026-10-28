#!/bin/zsh
# Uso: measure.sh <etiqueta> [host]. Mide /status (latencia y heap) y los fps
# del vídeo durante 10 s. El vídeo se guarda en $TMPDIR, no en el repositorio.
B=http://${2:-192.168.0.108}; T=${TMPDIR:-/tmp}; L=$1
st=$(curl -s -m 3 $B/status) || { echo "$L: sin respuesta"; exit 1; }
echo "$L status: $st"
lat=()
for i in $(seq 1 20); do lat+=$(curl -s -m 3 -o /dev/null -w "%{time_total}" $B/status); done
print -l $lat | sort -n | awk -v L="$L" '{a[NR]=$1*1000} END{printf "%s latencia /status ms: min %.0f, mediana %.0f, p90 %.0f, max %.0f (n=%d)\n", L, a[1], a[int(NR/2)+1], a[int(NR*0.9)], a[NR], NR}'
curl -s -m 10 -o $T/stream_$L.bin $B:81/stream
python3 -c "import sys;d=open(sys.argv[1],'rb').read();n=d.count(b'\xff\xd9');print(f'{sys.argv[2]} vídeo: {n} fotogramas en 10 s = {n/10:.1f} fps, {len(d)/10/1024:.0f} kB/s')" $T/stream_$L.bin $L
