#!/usr/bin/env bash
# ============================================================================
#  radiografia.sh — SACA TODO lo que importa de un VPS para comparar dos setups
#  Uso:  bash radiografia.sh        (deja el informe en /root/radiografia.txt)
#  Es SOLO LECTURA: no instala, no toca, no reinicia nada.
# ============================================================================
OUT=/root/radiografia.txt
: > "$OUT"
s(){ echo "$@" >> "$OUT"; }
t(){ echo; echo "════════════════════════════════════════"; echo "$@"; echo "════════════════════════════════════════"; }

exec > >(tee -a "$OUT") 2>&1

t "1) SISTEMA Y RED (MTU/MSS es clave para 'algunos sitios sí, otros no')"
hostname; uname -r; cat /etc/os-release 2>/dev/null | grep -E "^PRETTY"
echo "── interfaces y MTU ──"; ip -o link show 2>/dev/null | awk '{print $2, $0}' | grep -oE "^[a-z0-9@.-]+|mtu [0-9]+" | paste - - 2>/dev/null | head -10
echo "── rutas ──"; ip route 2>/dev/null | head -6
echo "── sysctl relevante ──"
for k in net.ipv4.ip_forward net.ipv4.tcp_mtu_probing net.ipv4.tcp_congestion_control net.core.default_qdisc; do
  printf "  %s = %s\n" "$k" "$(sysctl -n $k 2>/dev/null)"
done
echo "── iptables (nat/mangle, busca MSS clamp y DNAT) ──"
iptables -t nat -S 2>/dev/null | head -20
iptables -t mangle -S 2>/dev/null | grep -iE "mss|tcp" | head -10
echo "── DNS del server ──"; cat /etc/resolv.conf 2>/dev/null | grep -vE "^#" | head -5

t "2) QUÉ ESTÁ ESCUCHANDO (y quién es)"
ss -ltnp 2>/dev/null | grep -vE "127.0.0.1|::1" | head -30
echo "── todos los puertos ──"; ss -ltn 2>/dev/null | awk 'NR>1{print $4}' | sort -u | tr '\n' ' '; echo
echo "── servicios activos del stack ──"
systemctl list-units --type=service --state=running --no-pager --no-legend 2>/dev/null | awk '{print $1}' | grep -viE "systemd|dbus|cron|ssh.service|getty|user@|networkd|resolved|udev|journald|logind|polkit|rsyslog|unattended|multipathd|packagekit|irqbalance|chrony|snapd" | head -30

t "3) EL PORTERO / FRONT: qué programa recibe el :80 y con qué config"
for p in $(ss -ltnp 2>/dev/null | grep ':80 ' | grep -oE 'pid=[0-9]+' | cut -d= -f2 | sort -u); do
  echo "  pid $p → $(tr '\0' ' ' < /proc/$p/cmdline 2>/dev/null)"
  echo "     carpeta: $(readlink -f /proc/$p/cwd 2>/dev/null)"
  echo "     unidad : $(systemctl status $p 2>/dev/null | head -1)"
done
echo "── configs del stack (busco payloads, hosts y puertos) ──"
for d in /etc/ctmanager /opt/hcr-server /opt/ghost-configs /root/ws /etc/ws-epro /etc/ghost; do
  [ -d "$d" ] && { echo "  ── $d ──"; ls -la "$d" 2>/dev/null | head -12; }
done
echo "── archivos de config con 'payload' o 'bughost' o ':80' ──"
grep -rlniE "payload|101 switching|bughost|upgrade" /etc /opt /root --include="*.json" --include="*.yml" --include="*.yaml" --include="*.conf" --include="*.py" --include="*.sh" 2>/dev/null | grep -vE "node_modules|/root/ghost-go|/root/ws-nuevo" | head -12

t "4) NGINX / CADDY (si el front es un proxy web)"
for f in /etc/nginx/nginx.conf /etc/nginx/conf.d/*.conf /etc/nginx/sites-enabled/* /etc/caddy/Caddyfile; do
  [ -f "$f" ] && { echo "  ── $f ──"; grep -vE "^\s*#|^\s*$" "$f" 2>/dev/null | head -40; }
done

t "5) DOMINIOS QUE USA (los saco de las configs)"
grep -rhoE "[a-z0-9][a-z0-9.-]{3,}\.(com|net|online|top|dev|fun|ar|xyz|site|cloud)\b" /etc /opt /root/ws 2>/dev/null | grep -viE "example|localhost|schema|w3\.org|github|python|ubuntu|debian|nginx|caddy" | sort | uniq -c | sort -rn | head -20

t "6) CERTIFICADOS (TLS) Y SU VENCIMIENTO"
for f in $(find /etc/letsencrypt/live /etc/ssl/certs /etc/caddy /opt -maxdepth 4 -name "*.crt" -o -maxdepth 4 -name "fullchain.pem" 2>/dev/null | head -10); do
  D=$(openssl x509 -noout -enddate -subject 2>/dev/null < "$f" | tr '\n' ' ')
  [ -n "$D" ] && echo "  $f → $D"
done

t "7) BACKENDS: a dónde se manda cada protocolo"
for p in 22 443 1194 2223 8388 8443 10001 8080 8880 7300 3128; do
  if timeout 1 bash -c "exec 3<>/dev/tcp/127.0.0.1/$p" 2>/dev/null; then echo "  :$p → ACTIVO"; else echo "  :$p → nada"; fi
done
echo "── banner de cada backend activo (para identificar qué es) ──"
for p in 22 2200 8443 2223 10001; do
  B=$(timeout 3 bash -c "exec 3<>/dev/tcp/127.0.0.1/$p; head -c 30 <&3" 2>/dev/null | tr -d '\r\n')
  [ -n "$B" ] && echo "  :$p → $B"
done

t "8) CRONTAB Y ARRANQUES"
crontab -l 2>/dev/null | head -10
ls -la /etc/cron.d/ 2>/dev/null | head -6

t "9) RESUMEN PARA COMPARAR"
echo "  IP pública : $(curl -s --max-time 8 https://api.ipify.org 2>/dev/null)"
echo "  MTU (default iface): $(ip route get 1.1.1.1 2>/dev/null | grep -oE 'dev [a-z0-9]+' | head -1)"
echo "  front :80  : $(ss -ltnp 2>/dev/null | grep ':80 ' | head -1 | awk '{print $NF}')"
echo "  informe en : $OUT"
