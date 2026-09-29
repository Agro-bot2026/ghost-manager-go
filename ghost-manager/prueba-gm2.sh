#!/bin/bash
# prueba-gm2.sh — pruebas FASE 2 y 3 del ghost-manager Go (todo con archivos falsos en /tmp)
# el binario: se puede forzar con GM_BIN (en el VPS está en /root/ghost-go/ghost-manager-go)
BIN="${GM_BIN:-/tmp/ghost-manager-go}"
[ -x "$BIN" ] || BIN=/root/ghost-go/ghost-manager-go
DIR=/tmp/gmtest2
rm -rf "$DIR" && mkdir -p "$DIR"/{ovpn,slowdns,udp,hcr}
export GM_NO_SYSTEM=1 GM_NO_CLEAR=1 GM_SIN_COLOR=1 GM_SIN_UPDATE=1
export GM_SSH_DB="$DIR/ssh_users.db" GM_V2RAY_DB="$DIR/v2ray_users.db" GM_TRAFICO_DB="$DIR/trafico.db"
export GM_CONFIG_DIR="$DIR/cfg"
export GM_CONFIG_JSON="$DIR/config.json" GM_OVPN_DIR="$DIR/ovpn" GM_DOMINIO_FILE="$DIR/dominio"
export GM_CA_OVPN="$DIR/ca.crt" GM_SS_PASS_FILE="$DIR/ss_password" GM_SS_USERS_SH="$DIR/ss_users.sh"
export GM_SLOWDNS_BIN="$DIR/slowdns/sldns-server" GM_SLOWDNS_SERVICE="$DIR/slowdns/sldns-server.service"
export GM_SLOWDNS_PUB="$DIR/slowdns/server.pub" GM_PSIPHON_ENTRY="$DIR/psiphon-entry.dat"
export GM_PSIPHON_CFG="$DIR/psiphond.config" GM_PSIPHON_LIC="$DIR/psiphon-lic.dat"
export GM_XRAY_UUID_SH="$DIR/no-existe.sh" GM_SSH_BANNER="$DIR/banner-ssh" GM_GEO_DB="$DIR/geo.db"
export GM_UDP_DIR="$DIR/udp" GM_UDP_BIN="$DIR/udp/udp-custom" GM_UDP_CFG="$DIR/udp/config.json"
export GM_HCR_KEY_FILE="$DIR/hcr.key" GM_HCR_DIR="$DIR/hcr"

# archivos de mentira
echo "test.local" > "$DIR/dominio"
echo '{"quota_gb":0,"ss_port":8388}' > "$DIR/config.json"
printf -- "-----BEGIN CERTIFICATE-----\nFAKECA123\n-----END CERTIFICATE-----\n" > "$DIR/ca.crt"
echo "3773058d97b5c003" > "$DIR/ss_password"
printf '#!/bin/bash\necho "shadow: pass /etc/ss_users.sh"\n' > "$DIR/ss_users.sh"; chmod +x "$DIR/ss_users.sh"
mkdir -p "$DIR/slowdns"; touch "$DIR/slowdns/sldns-server"; chmod +x "$DIR/slowdns/sldns-server"
printf 'ExecStart=/etc/slowdns/sldns-server -udp :5300 -privkey-file /etc/slowdns/server.key sdns.midominio.com\n' > "$DIR/slowdns/sldns-server.service"
echo "PUBKEY123456789" > "$DIR/slowdns/server.pub"
printf 'ENTRY-HEX-ABC123\n' > "$DIR/psiphon-entry.dat"
printf '{"SSHUserName":"psiphonuser"}\n' > "$DIR/psiphond.config"

ok=0; ko=0
chk() { if [ "$1" = "1" ]; then echo "  ✅ $2"; ok=$((ok+1)); else echo "  ❌ $2"; ko=$((ko+1)); fi; }

echo "=== F2-1) CREAR USUARIO OPENVPN + ARMAR EL .OVPN ==="
printf '3\npedro\n30\n1\n0\n' | $BIN > "$DIR/o-ovpn.txt" 2>&1
chk "$(grep -q "USUARIO OPENVPN CREADO" "$DIR/o-ovpn.txt" && echo 1 || echo 0)" "creó el usuario OpenVPN"
chk "$([ -f "$DIR/ovpn/pedro.ovpn" ] && echo 1 || echo 0)" "escribió el .ovpn en el disco"
chk "$(grep -q "remote test.local 80" "$DIR/ovpn/pedro.ovpn" && echo 1 || echo 0)" "el .ovpn apunta a test.local:80"
chk "$(grep -q "cipher none" "$DIR/ovpn/pedro.ovpn" && echo 1 || echo 0)" "cipher none (zero-rating)"
chk "$(grep -q "FAKECA123" "$DIR/ovpn/pedro.ovpn" && echo 1 || echo 0)" "el CA quedó embebido en el .ovpn"
chk "$(grep -q "TU-BUGHOST-1" "$DIR/o-ovpn.txt" && echo 1 || echo 0)" "muestra el PAYLOAD de Personal"
chk "$(grep -q "ovpn/pedro.ovpn" "$DIR/o-ovpn.txt" && echo 1 || echo 0)" "da el link de descarga"

echo "=== F2-2) UDP CUSTOM (crear usuario) ==="
printf '5\nana\npersonal\n30\n' | $BIN > "$DIR/o-udp.txt" 2>&1
chk "$(grep -q "USUARIO UDP CUSTOM CREADO" "$DIR/o-udp.txt" && echo 1 || echo 0)" "creó el usuario UDP Custom"
chk "$(grep -q "personal" "$DIR/o-udp.txt" && echo 1 || echo 0)" "guardó la operadora (personal)"
chk "$(grep -q "1-65535" "$DIR/o-udp.txt" && echo 1 || echo 0)" "muestra el rango de puertos"

echo "=== F2-3) LINK SHADOWSOCKS ==="
printf '14\n' | $BIN > "$DIR/o-ss.txt" 2>&1
B64=$(printf "aes-256-gcm:3773058d97b5c003" | base64 -w0)
chk "$(grep -q "ss://$B64@" "$DIR/o-ss.txt" && echo 1 || echo 0)" "arma el link ss:// con la password real"
chk "$(grep -q "aes-256-gcm" "$DIR/o-ss.txt" && echo 1 || echo 0)" "método aes-256-gcm"
chk "$(grep -q "TU-BUGHOST-2" "$DIR/o-ss.txt" && echo 1 || echo 0)" "muestra el proxy remoto"
chk "$(grep -q ":8388#Ghost-SS" "$DIR/o-ss.txt" && echo 1 || echo 0)" "link directo con el puerto interno 8388"

echo "=== F2-4) VER LINK SHADOWSOCKS (lee ss_users.sh) ==="
printf '16\n' | $BIN > "$DIR/o-ssv.txt" 2>&1
chk "$(grep -q "ss_users.sh" "$DIR/o-ssv.txt" && echo 1 || echo 0)" "ejecuta ss_users.sh show"

echo "=== F2-5) DATOS SLOWDNS ==="
printf '17\n' | $BIN > "$DIR/o-sl.txt" 2>&1
chk "$(grep -q "sdns.midominio.com" "$DIR/o-sl.txt" && echo 1 || echo 0)" "saca el nameserver del service (sdns.midominio.com)"
chk "$(grep -q "PUBKEY123456789" "$DIR/o-sl.txt" && echo 1 || echo 0)" "muestra la pubkey"

echo "=== F2-6) CREAR USUARIO SLOWDNS (clave de 12 caracteres) ==="
printf '18\nluz\n30\n' | $BIN > "$DIR/o-slc.txt" 2>&1
CLAVE_SL=$(sqlite3 "$DIR/ssh_users.db" "SELECT password FROM users WHERE username='luz';")
chk "$(grep -q "USUARIO SLOWDNS CREADO" "$DIR/o-slc.txt" && echo 1 || echo 0)" "creó el usuario SlowDNS"
chk "$([ ${#CLAVE_SL} = 12 ] && echo 1 || echo 0)" "la clave tiene 12 caracteres (openssl -hex 6) → $CLAVE_SL"
chk "$(grep -q "sdns.midominio.com" "$DIR/o-slc.txt" && echo 1 || echo 0)" "le da el nameserver"

echo "=== F2-7) BANNER SSH (plantillas) ==="
printf '13\n2\n2\n0\n' | $BIN > "$DIR/o-ban.txt" 2>&1
chk "$(grep -q "GHOST VPN" "$DIR/banner-ssh" && echo 1 || echo 0)" "guardó la plantilla Ghost en /etc/ssh/banner"
printf '13\n5\n0\n' | $BIN > "$DIR/o-ban2.txt" 2>&1
chk "$(grep -q "E P R O . H C" "$DIR/banner-ssh" && echo 1 || echo 0)" "restauró la plantilla EPRO.HC"
printf '13\n4\n0\n' | $BIN > "$DIR/o-ban3.txt" 2>&1
chk "$(grep -q "E P R O . H C" "$DIR/banner-ssh" && echo 1 || echo 0)" "escribió la versión con colores ANSI"

echo "=== F2-8) PSIPHON / WIREGUARD / PANEL ==="
printf '10\n1\n' | $BIN > "$DIR/o-psi.txt" 2>&1
chk "$(grep -q "ENTRY-HEX-ABC123" "$DIR/o-psi.txt" && echo 1 || echo 0)" "muestra el HEX del server-entry"
chk "$(grep -q "psiphonuser" "$DIR/o-psi.txt" && echo 1 || echo 0)" "lee el SSHUserName del config"
printf '11\n' | $BIN > "$DIR/o-wg.txt" 2>&1
chk "$(grep -qE "wg0" "$DIR/o-wg.txt" && echo 1 || echo 0)" "muestra el estado de wg0"
printf '12\n' | $BIN > "$DIR/o-pan.txt" 2>&1
chk "$(grep -q "8303" "$DIR/o-pan.txt" && echo 1 || echo 0)" "habla del panel :8303"

echo "=== F3-1) INSTALADOR UDP CUSTOM (no toca nada en prueba) ==="
printf '30\n' | $BIN > "$DIR/o-udpi.txt" 2>&1
chk "$(grep -q "INSTALAR UDP CUSTOM" "$DIR/o-udpi.txt" && echo 1 || echo 0)" "abre el instalador de UDP Custom"
chk "$(grep -q "prueba: no descargo" "$DIR/o-udpi.txt" && echo 1 || echo 0)" "en prueba NO descarga el binario"
chk "$(grep -q "36712" "$DIR/udp/config.json" && echo 1 || echo 0)" "escribió el config.json con el puerto 36712"

echo "=== F3-2) MENÚ BILOLA ==="
printf '28\n4\n0\n' | $BIN > "$DIR/o-bil.txt" 2>&1
chk "$(grep -q "PROTOCOLOS BILOLA" "$DIR/o-bil.txt" && echo 1 || echo 0)" "dibuja el menú Bilola"
chk "$(grep -q "BHTTP" "$DIR/o-bil.txt" && echo 1 || echo 0)" "lista los 3 protocolos (BHTTP)"
chk "$(grep -q "no instalado" "$DIR/o-bil.txt" && echo 1 || echo 0)" "estado: dice que no están instalados"
chk "$(grep -q "Desarrollo original: equipo del protocolo" "$DIR/o-bil.txt" && echo 1 || echo 0)" "mantiene la nota del equipo original"

echo "=== F3-3) INSTALAR HCR (pide clave, en prueba no baja) ==="
printf '27\nCLAVE-DE-PRUEBA\n' | $BIN > "$DIR/o-hcr.txt" 2>&1
chk "$(grep -q "INSTALAR HCR" "$DIR/o-hcr.txt" && echo 1 || echo 0)" "abre el instalador de HCR"
chk "$(grep -q "prueba: no descargo" "$DIR/o-hcr.txt" && echo 1 || echo 0)" "en prueba NO descarga el binario"

echo "=== F3-4) API KEY ePRO + BOT TELEGRAM (en prueba no guardan) ==="
printf '26\nMI-KEY-EPRO-123\n' | $BIN > "$DIR/o-epro.txt" 2>&1
chk "$(grep -q "API KEY ePRO" "$DIR/o-epro.txt" && echo 1 || echo 0)" "abre el configurador de la API key"
chk "$(grep -q "prueba: no guardo la key" "$DIR/o-epro.txt" && echo 1 || echo 0)" "en prueba NO guarda la key"
printf '25\n123456:ABC-DEF\n123456789\n' | $BIN > "$DIR/o-bot.txt" 2>&1
chk "$(grep -q "BOT DE ADMINISTRACIÓN" "$DIR/o-bot.txt" && echo 1 || echo 0)" "abre el configurador del bot"
chk "$(grep -q "prueba: no guardo token" "$DIR/o-bot.txt" && echo 1 || echo 0)" "en prueba NO guarda token"
printf '25\n123456:ABC-DEF\nNO-NUMERICO\n' | $BIN > "$DIR/o-bot2.txt" 2>&1
chk "$(grep -q "User ID inválido" "$DIR/o-bot2.txt" && echo 1 || echo 0)" "rechaza un User ID no numérico"

echo "=== F3-5) QUE NO QUEDE NINGUNA OPCIÓN PENDIENTE ==="
printf '1\n0\n' | $BIN > "$DIR/o-nada.txt" 2>&1
chk "$(grep -q "todavía no está en la versión Go" "$DIR/o-nada.txt" && echo 0 || echo 1)" "no queda ninguna opción sin portar"

echo ""; echo "════════ RESUMEN F2/F3 ════════"; echo "  ✅ $ok pruebas OK"
if [ "$ko" != "0" ]; then echo "  ❌ $ko FALLARON"; exit 1; fi
echo "  TODAS PASARON ✅"
