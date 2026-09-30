#!/usr/bin/env bash
# ============================================================================
#  dump-front.sh — saca EL FRONT y SU CONFIG del VPS que funciona (SOLO LECTURA)
#  Deja todo en /root/front.txt
# ============================================================================
OUT=/root/front.txt
: > "$OUT"
exec > >(tee -a "$OUT") 2>&1
t(){ echo; echo "════════════════════════════════════════"; echo "$@"; echo "════════════════════════════════════════"; }

t "1) EL PROGRAMA QUE TIENE EL :80 (PandaScript / PDirect80.py)"
pid80=$(ss -ltnp 2>/dev/null | grep ':80 ' | grep -oE 'pid=[0-9]+' | cut -d= -f2 | head -1)
echo "  pid: $pid80"
echo "  cmd: $(tr '\0' ' ' < /proc/$pid80/cmdline 2>/dev/null)"
echo "  cwd: $(readlink -f /proc/$pid80/cwd 2>/dev/null)"
echo "  arrancado por: $(ps -o ppid= -p $pid80 2>/dev/null | xargs -I{} sh -c 'ps -o cmd= -p {}' 2>/dev/null)"

t "2) ARCHIVOS DEL FRONT (listado)"
for d in /etc/ADMcgh /etc/adm-lite /etc/ADM-lite /etc/pandascript /etc/panda; do
  [ -d "$d" ] && { echo "  ── $d ──"; ls -la "$d" 2>/dev/null; }
done
echo "  ── donde esté PDirect80.py ──"
timeout 25 find /etc /opt /root /usr/local /usr/bin -maxdepth 5 \( -name "PDirect*" -o -name "PandaScript*" \) 2>/dev/null | head -10

t "3) CONTENIDO DEL FRONT (el script del :80 — acá está el payload y los backends)"
for f in $(find /etc/ADMcgh /etc/adm-lite -maxdepth 2 -name "*.py" 2>/dev/null | head -8); do
  echo "  ┌──── $f ($(wc -l < $f) líneas) ────"
  head -80 "$f" 2>/dev/null | sed 's/^/  │ /'
  echo "  └──── fin $f ────"
done

t "4) CONFIGS (payload, puertos, backends)"
for f in $(find /etc/ADMcgh /etc/adm-lite /root -maxdepth 3 -name "*.json" -o -maxdepth 3 -name "*.conf" -o -maxdepth 3 -name "*.txt" 2>/dev/null | grep -viE "cache|log|ssh/|apt|systemd" | head -12); do
  echo "  ── $f ──"; head -30 "$f" 2>/dev/null | sed 's/^/     /'
done

t "5) ¿QUÉ LO ARRANCA? (cron/autoboot/systemd)"
echo "  ── /bin/autoboot ──"; [ -f /bin/autoboot ] && head -40 /bin/autoboot | sed 's/^/     /' || echo "     (no está)"
echo "  ── crontab ──"; crontab -l 2>/dev/null | sed 's/^/     /'
echo "  ── unidades con panda/adm/pdirect ──"
systemctl list-units --all --no-pager --no-legend 2>/dev/null | grep -iE "panda|adm|direct|ws|proxy" | head -10

t "6) LOS OTROS PUERTOS (apache :81 y badvpn :7300)"
echo "  ── apache sites ──"
ls /etc/apache2/sites-enabled/ 2>/dev/null | sed 's/^/     /'
for f in /etc/apache2/sites-enabled/*.conf; do
  [ -f "$f" ] && { echo "  ── $f ──"; grep -vE "^\s*#|^\s*$" "$f" | head -25 | sed 's/^/     /'; }
done
echo "  ── badvpn (udpgw) ──"
ps aux 2>/dev/null | grep -iE "badvpn|udpgw" | grep -v grep | sed 's/^/     /'

t "7) ¿TIENE V2RAY/OTROS BACKENDS?"
for p in 22 8443 10001 10000 2223 1194 8388 8080 8880 7300 3128; do
  B=$(timeout 2 bash -c "exec 3<>/dev/tcp/127.0.0.1/$p; head -c 40 <&3" 2>/dev/null | tr -d '\r\n')
  timeout 1 bash -c "exec 3<>/dev/tcp/127.0.0.1/$p" 2>/dev/null && echo "     :$p ACTIVO ${B:+· $B}" || echo "     :$p nada"
done

t "8) BUSCO 'PandaScript' EN TODO EL SISTEMA (dónde está definido)"
timeout 25 grep -rln "PandaScript" /etc/ADMcgh /etc/adm-lite /opt /usr/local/bin /root 2>/dev/null | head -10

t "9) RESUMEN"
echo "  informe en: $OUT   ($(wc -l < $OUT) líneas)"
