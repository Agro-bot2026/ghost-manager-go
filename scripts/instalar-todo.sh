#!/bin/bash
# ============================================================================
#  instalar-todo.sh — deja TODO instalado y andando en un VPS nuevo:
#    1) licencia  2) el stack completo (SSH/Psiphon/V2Ray/OpenVPN/WG/UDP/BadVPN/HCR)
#    3) el portero y el menú nuevos  →  y verifica que quedó todo arriba
#  Uso:  bash instalar-todo.sh [TOKEN]
#        TOKEN = tu token de licencia (si no lo pasás, te lo pide)
# ============================================================================
set -u
LIC_FILE="/etc/ctmanager/licencia.token"
STACK="https://raw.githubusercontent.com/Agro-bot2026/ghost-instalador/main/ghost-proxy-v14.sh"
MENU="https://raw.githubusercontent.com/Agro-bot2026/ghost-manager-go/main/scripts/instalar.sh"
API="configs.charly-tricks.dev"
verde(){ printf '\033[0;32m%s\033[0m\n' "$*"; }; rojo(){ printf '\033[0;31m%s\033[0m\n' "$*"; }
info(){ printf '\033[0;36m%s\033[0m\n' "$*"; }
[ "$(id -u)" = "0" ] || { rojo "❌ Ejecutá como root"; exit 1; }

info "═══ 1/4 · LICENCIA ═══"
mkdir -p /etc/ctmanager
if [ -s "$LIC_FILE" ]; then
  verde "✅ ya hay una licencia guardada en $LIC_FILE"
else
  T="${1:-}"
  [ -z "$T" ] && read -r -p "  🔑 Pegá tu token de licencia: " T
  [ -z "$T" ] && { rojo "❌ sin token no se puede instalar"; exit 1; }
  echo "$T" > "$LIC_FILE"; chmod 600 "$LIC_FILE"; verde "✅ licencia guardada"
fi

info "═══ 2/4 · STACK COMPLETO (ssh, psiphon, v2ray, openvpn, wg, udp, badvpn, hcr) ═══"
BASE="https://$API"
if ! curl -s --max-time 10 -o /dev/null -X POST "$BASE/api/licencia/validar" -H 'Content-Type: application/json' -d '{"token":"x","hostname":"t","ip":"1.1.1.1"}'; then
  info "   (el 443 no responde, uso el puerto directo del servidor de configs)"
  BASE="https://$API:8082"
fi
info "   servidor de licencias/binarios: $BASE"
curl -fsSL --max-time 60 "$STACK" -o /tmp/ghost-stack.sh || { rojo "❌ no pude bajar el instalador del stack"; exit 1; }
sed -i "s#https://configs.charly-tricks.dev#${BASE}#g" /tmp/ghost-stack.sh
info "   instalando el stack en modo automático…"
bash /tmp/ghost-stack.sh auto

info "═══ 3/4 · PORTERO + MENÚ NUEVOS ═══"
bash <(curl -fsSL "$MENU")

info "═══ 4/4 · VERIFICACIÓN ═══"
sep="────────────────────────────────────────"
echo "  ${sep}"
for s in ghostprox sshgo xray psiphon openvpn@server wg-quick@wg0 udp-custom hcr-server hcr-server-8080 dropbear squid; do
  est=$(systemctl is-active "$s" 2>/dev/null || echo n/a)
  if [ "$est" = "active" ]; then printf "  ✅ %-22s activo\n" "$s"; else printf "  ⚠️  %-22s %s\n" "$s" "$est"; fi
done
echo "  ${sep}"
if [ -x /usr/local/bin/ghost-manager ]; then echo "  ✅ menú: $(/usr/local/bin/ghost-manager --version 2>/dev/null)"; fi
if ss -ltn 2>/dev/null | grep -q ':80 '; then echo "  ✅ el :80 está escuchando (portero)"; else echo "  ⚠️  el :80 no está escuchando"; fi
echo "  ${sep}"
verde "LISTO. Escribí:  ghost-manager"
info "Si algo quedó en ⚠️, mirá:  journalctl -u <servicio> -n 30"
