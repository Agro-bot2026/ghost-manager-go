#!/bin/bash
# ============================================================================
#  instalador de ghost-manager-go  (portero ghostprox + menú ghost-manager)
#  Uso:  sudo bash instalar.sh [opciones]
#    --solo-menu        instala únicamente el menú
#    --solo-portero     instala únicamente el portero
#    --puerto N         puerto del portero (por defecto 80)
#    --sin-servicio     no crea el servicio systemd (solo deja los binarios)
#    --desinstalar      saca todo (deja las bases y configs intactas)
#  No toca nada que ya exista sin avisar: si hay binarios previos, los respalda.
# ============================================================================
set -u
VER="v29.0-go"
BIN_DIR="/usr/local/bin"
CFG_DIR="/etc/ctmanager/config"
CFG_JSON="/etc/ctmanager/websocket/config.json"
PUERTO="80"
MODO="todo"

while [ $# -gt 0 ]; do
  case "$1" in
    --solo-menu)    MODO="menu" ;;
    --solo-portero) MODO="portero" ;;
    --puerto)       PUERTO="${2:-80}"; shift ;;
    --sin-servicio) SIN_SERVICIO=1 ;;
    --desinstalar)  MODO="desinstalar" ;;
    -h|--ayuda)     sed -n '2,14p' "$0"; exit 0 ;;
    *) echo "opción desconocida: $1"; exit 1 ;;
  esac
  shift
done

rojo()  { printf '\033[0;31m%s\033[0m\n' "$*"; }
verde() { printf '\033[0;32m%s\033[0m\n' "$*"; }
amar()  { printf '\033[1;33m%s\033[0m\n' "$*"; }
info()  { printf '\033[0;36m%s\033[0m\n' "$*"; }

[ "$(id -u)" = "0" ] || { rojo "❌ Ejecutá como root"; exit 1; }

# ─── desinstalar ────────────────────────────────────────────────────────────
if [ "$MODO" = "desinstalar" ]; then
  info "── desinstalando servicios y binarios ──"
  systemctl stop ghostprox 2>/dev/null || true
  systemctl disable ghostprox 2>/dev/null || true
  rm -f /etc/systemd/system/ghostprox.service
  systemctl daemon-reload
  rm -f "$BIN_DIR/ghostprox" "$BIN_DIR/ghost-manager"
  verde "✅ listo. NO toqué las bases ($CFG_DIR) ni los backends."
  exit 0
fi

echo "════════ INSTALADOR ghost-manager-go $VER ════════"

# ─── 1) Go ──────────────────────────────────────────────────────────────────
necesito_go=0
if [ "$MODO" != "portero" ] || command -v go >/dev/null 2>&1; then :; fi
if ! command -v go >/dev/null 2>&1; then
  necesito_go=1
  amar "⚠️  No hay Go instalado. Intentando instalarlo…"
  if command -v apt-get >/dev/null 2>&1; then apt-get update -qq && apt-get install -y -qq golang-go || true
  elif command -v dnf >/dev/null 2>&1; then dnf install -y golang || true
  elif command -v apk >/dev/null 2>&1; then apk add go || true
  fi
fi
if ! command -v go >/dev/null 2>&1; then
  rojo "❌ No pude instalar Go. Instalalo a mano (necesita 1.19 o superior) y volvé a correr esto:"
  echo "     https://go.dev/dl/    (o: apt install golang-go)"
  exit 1
fi
GOVER=$(go version | awk '{print $3}')
verde "✅ Go disponible: $GOVER"
case "$GOVER" in
  go1.1[9]|go1.1[9].*|go1.[2-9]*|go[2-9]*) ;;
  *) amar "⚠️  Go muy viejo ($GOVER): el proyecto necesita 1.19+ para el driver SQLite puro" ;;
esac

# ─── 2) código fuente ───────────────────────────────────────────────────────
SRC=""
if [ -f "./ghost-manager/main.go" ]; then
  SRC="$(pwd)"
  info "── usando el código de esta carpeta ──"
else
  info "── bajando el código del repositorio ──"
  SRC="/usr/local/src/ghost-manager-go"
  mkdir -p "$SRC"
  REPO_URL="https://github.com/TU-USUARIO/TU-REPO"
  if command -v git >/dev/null 2>&1; then
    git clone --depth 1 "$REPO_URL.git" "$SRC" 2>/dev/null || true
  fi
  if [ ! -f "$SRC/ghost-manager/main.go" ]; then
    # sin git: bajamos el tar del repo
    curl -fsSL "$REPO_URL/archive/refs/heads/main.tar.gz" -o /tmp/gm.tar.gz || {
      rojo "❌ no pude bajar el código ($REPO_URL)"; exit 1; }
    tar xzf /tmp/gm.tar.gz -C "$SRC" --strip-components=1 && rm -f /tmp/gm.tar.gz
  fi
fi
[ -f "$SRC/ghost-manager/main.go" ] || { rojo "❌ no encuentro ghost-manager/main.go"; exit 1; }

# ─── 3) compilar ────────────────────────────────────────────────────────────
info "── compilando (estático, sin cgo) ──"
export CGO_ENABLED=0 GOOS=linux GOARCH=amd64
compilar() { # $1 = carpeta, $2 = nombre de salida
  ( cd "$1" && GOFLAGS=-mod=mod go build -trimpath -ldflags "-s -w" -o "/tmp/$2" . )
}
case "$MODO" in
  todo|portero) compilar "$SRC/ghostprox" ghostprox.build || { rojo "❌ falló la compilación del portero"; exit 1; } ;;
esac
case "$MODO" in
  todo|menu) compilar "$SRC/ghost-manager" ghost-manager.build || { rojo "❌ falló la compilación del menú"; exit 1; } ;;
esac
verde "✅ compilado"

# ─── 4) instalar binarios (con respaldo) ────────────────────────────────────
instalar_bin() { # $1 = archivo origen, $2 = nombre destino
  if [ -f "$BIN_DIR/$2" ]; then
    cp -a "$BIN_DIR/$2" "$BIN_DIR/$2.bak-$(date +%Y%m%d-%H%M%S)"
    amar "   (respaldé el $2 anterior)"
  fi
  install -m 755 "$1" "$BIN_DIR/$2"
  verde "✅ $BIN_DIR/$2"
}
case "$MODO" in
  todo|portero) instalar_bin /tmp/ghostprox.build ghostprox ;;
esac
case "$MODO" in
  todo|menu) instalar_bin /tmp/ghost-manager.build ghost-manager ;;
esac

# ─── 5) config y bases (sin pisar lo existente) ─────────────────────────────
info "── config ──"
mkdir -p "$CFG_DIR" "$(dirname "$CFG_JSON")"
if [ ! -f "$CFG_JSON" ]; then
  cat > "$CFG_JSON" <<'JSON'
{
  "quota_gb": 0,
  "quota_dias": 30,
  "max_connections_per_user": 50,
  "v2ray_check_users": true,
  "v2ray_quota_gb": 50,
  "v2ray_quota_dias": 30,
  "sshgo": 2200,
  "sshd": 22,
  "dropbear": 444,
  "psiphon": 2223,
  "ss": 8388,
  "vless": 8443,
  "openvpn": 1194
}
JSON
  verde "✅ config nuevo en $CFG_JSON"
else
  amar "⚠️  ya existía $CFG_JSON (no lo toco)"
fi
if [ ! -f "$CFG_DIR/dominio" ]; then
  IP=$(curl -4 -s --max-time 5 ifconfig.me 2>/dev/null || hostname -I 2>/dev/null | awk '{print $1}')
  echo "$IP" > "$CFG_DIR/dominio"
  verde "✅ dominio/dirección: $IP (cambialo en $CFG_DIR/dominio cuando tengas dominio)"
fi

# ─── 6) servicio del portero ────────────────────────────────────────────────
if [ "$MODO" != "menu" ] && [ -z "${SIN_SERVICIO:-}" ]; then
  info "── servicio ghostprox (puerto $PUERTO) ──"
  cat > /etc/systemd/system/ghostprox.service <<UNIT
[Unit]
Description=Ghostprox - portero Zero-Rating (puerto $PUERTO)
After=network.target

[Service]
Type=simple
Environment=LISTEN=:$PUERTO
Environment=GHOST_CONFIG_JSON=$CFG_JSON
ExecStart=$BIN_DIR/ghostprox
Restart=always
RestartSec=3

[Install]
WantedBy=multi-user.target
UNIT
  systemctl daemon-reload
  systemctl enable ghostprox >/dev/null 2>&1 || true
  systemctl restart ghostprox
  sleep 2
  if systemctl is-active --quiet ghostprox; then
    verde "✅ ghostprox ACTIVO en el puerto $PUERTO"
  else
    rojo "❌ no arrancó — mirá:  journalctl -u ghostprox -n 30"
    amar "   (si el puerto $PUERTO está ocupado, probá:  --puerto 8080)"
  fi
fi

# ─── 7) cierre ──────────────────────────────────────────────────────────────
echo
verde "════ LISTO ════"
cat <<FIN

  Menú:      escribí  ghost-manager
  Portero:   $BIN_DIR/ghostprox cuentas | top | limite <usuario> <GB> | reset <usuario> --confirmo
  Config:    $CFG_JSON   (recarga en caliente:  kill -HUP \$(pgrep -f ghostprox))

  Recordá completar los placeholders del código si vas a usar los instaladores internos
  (HCR / Bilola / panel web): TU-SERVIDOR y TU-USUARIO/TU-REPO.

  Los BACKENDS (sshgo, xray, psiphon, openvpn, hcr, udp-custom…) NO se instalan acá:
  el menú tiene las opciones para instalarlos, bajando los binarios de tus URLs.

  Para sacarlo todo:   bash instalar.sh --desinstalar
FIN
