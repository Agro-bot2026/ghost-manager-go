#!/bin/bash
# prueba-gm.sh — pruebas del ghost-manager en Go (NO toca nada real: usa bases de /tmp y GM_NO_SYSTEM=1)
# el binario: se puede forzar con GM_BIN (en el VPS está en /root/ghost-go/ghost-manager-go)
BIN="${GM_BIN:-/tmp/ghost-manager-go}"
[ -x "$BIN" ] || BIN=/root/ghost-go/ghost-manager-go
DIR=/tmp/gmtest
rm -rf "$DIR" && mkdir -p "$DIR"

export GM_NO_SYSTEM=1 GM_NO_CLEAR=1 GM_SIN_COLOR=1
export GM_SSH_DB="$DIR/ssh_users.db" GM_V2RAY_DB="$DIR/v2ray_users.db" GM_TRAFICO_DB="$DIR/trafico.db"
export GM_CONFIG_JSON="$DIR/config.json" GM_LINKS_FILE="$DIR/v2ray_links.txt" GM_BANNER_FILE="$DIR/banner.txt"
export GM_XRAY_UUID_SH="$DIR/no-existe.sh" GM_DOMINIO_FILE="$DIR/dominio"
echo "test.local" > "$DIR/dominio"
echo '{"quota_gb":0,"quota_dias":30,"v2ray_quota_gb":50,"v2ray_quota_dias":30,"max_connections_per_user":50}' > "$DIR/config.json"

ok=0; ko=0
chk() { if [ "$1" = "1" ]; then echo "  ✅ $2"; ok=$((ok+1)); else echo "  ❌ $2"; ko=$((ko+1)); fi; }
grep_q() { [ -n "$(sqlite3 "$1" "$2" 2>/dev/null)" ] && echo 1 || echo 0; }

echo "=== 0) FIRMA ==="
V=$($BIN --version)
chk "$(echo "$V" | grep -q "v29.0-go" && echo 1 || echo 0)" "firma v29.0-go → $V"

echo "=== 1) ESTADO (con bases de prueba) ==="
$BIN estado > "$DIR/out-estado.txt" 2>&1
chk "$(grep -q "ESTADO DEL SISTEMA" "$DIR/out-estado.txt" && echo 1 || echo 0)" "muestra el estado"
chk "$(grep -q "Puerto 80" "$DIR/out-estado.txt" && echo 1 || echo 0)" "muestra el puerto 80 (portero)"
head -14 "$DIR/out-estado.txt" | sed 's/^/     /'

echo "=== 2) CREAR USUARIO SSH (plan multi + 3 GB) ==="
printf '2\njuan\n30\n2\n3\n3\n2\n' | $BIN > "$DIR/out-crear.txt" 2>&1
chk "$(grep -q "USUARIO CREADO" "$DIR/out-crear.txt" && echo 1 || echo 0)" "creó el usuario"
chk "$(grep -q "MULTI" "$DIR/out-crear.txt" && echo 1 || echo 0)" "guardó el plan MULTI"
chk "$(grep_q "$DIR/ssh_users.db" "SELECT username FROM users WHERE username='juan';")" "quedó en ssh_users.db"
chk "$(grep_q "$DIR/ssh_users.db" "SELECT limit_gb FROM users WHERE username='juan' AND limit_gb=3;")" "guardó el límite de 3 GB"
chk "$(grep_q "$DIR/ssh_users.db" "SELECT max_conn FROM users WHERE username='juan' AND max_conn=3;")" "guardó 3 conexiones"
chk "$(grep_q "$DIR/ssh_users.db" "SELECT limit_unit FROM users WHERE username='juan' AND limit_unit='GB';")" "escribió limit_unit (lo que lee sshgo)"
grep -E "Usuario|Password|Expira|Plan|Límite" "$DIR/out-crear.txt" | sed 's/^/     /'

echo "=== 3) CREAR USUARIO V2RAY (VLESS + 25 GB) ==="
printf '4\n1\nlucas\n30\n25\n' | $BIN > "$DIR/out-v2.txt" 2>&1
chk "$(grep -q "USUARIO V2RAY CREADO" "$DIR/out-v2.txt" && echo 1 || echo 0)" "creó la cuenta V2Ray"
chk "$(grep_q "$DIR/v2ray_users.db" "SELECT uuid FROM users WHERE username='lucas';")" "guardó el uuid"
chk "$(grep_q "$DIR/v2ray_users.db" "SELECT limit_gb FROM users WHERE username='lucas' AND limit_gb=25;")" "guardó el límite de 25 GB"
chk "$(grep -q "vless://" "$DIR/out-v2.txt" && echo 1 || echo 0)" "mostró el link vless://"
chk "$(grep -q "vless://" "$DIR/v2ray_links.txt" && echo 1 || echo 0)" "guardó el link en v2ray_links.txt"
grep -E "UUID|Link|Expira" "$DIR/out-v2.txt" | sed 's/^/     /'

echo "=== 4) LISTAR ==="
printf '6\n' | $BIN > "$DIR/out-listar.txt" 2>&1
chk "$(grep -q "juan" "$DIR/out-listar.txt" && echo 1 || echo 0)" "lista al usuario SSH"
chk "$(grep -q "lucas" "$DIR/out-listar.txt" && echo 1 || echo 0)" "lista la cuenta V2Ray"

echo "=== 5) RENOVAR (30 días más) ==="
VENCE_ANTES=$(sqlite3 "$DIR/ssh_users.db" "SELECT expires_at FROM users WHERE username='juan';")
printf '7\njuan\n30\n' | $BIN > "$DIR/out-renovar.txt" 2>&1
VENCE_DESPUES=$(sqlite3 "$DIR/ssh_users.db" "SELECT expires_at FROM users WHERE username='juan';")
chk "$(grep -q "renovado" "$DIR/out-renovar.txt" && echo 1 || echo 0)" "renovó"
chk "$([ "$VENCE_ANTES" != "$VENCE_DESPUES" ] && echo 1 || echo 0)" "cambió el vencimiento ($VENCE_ANTES → $VENCE_DESPUES)"
chk "$([ -z "$(sqlite3 "$DIR/v2ray_users.db" "SELECT username FROM users WHERE username='juan';")" ] && echo 1 || echo 0)" "NO metió al usuario SSH en la base de V2Ray"
chk "$(grep -q "V2Ray renovado" "$DIR/out-renovar.txt" && echo 0 || echo 1)" "no dice que renovó V2Ray (no existía)"

echo "=== 6) EDITAR (plan solo + clave nueva) ==="
CLAVE_ANTES=$(sqlite3 "$DIR/ssh_users.db" "SELECT password FROM users WHERE username='juan';")
printf '8\njuan\n1\n1\n' | $BIN > "$DIR/out-editar.txt" 2>&1
chk "$(grep_q "$DIR/ssh_users.db" "SELECT tipo FROM users WHERE username='juan' AND tipo='solo';")" "cambió a plan solo"
chk "$(grep_q "$DIR/ssh_users.db" "SELECT max_conn FROM users WHERE username='juan' AND max_conn=1;")" "1 conexión"
printf '8\njuan\n3\n' | $BIN > "$DIR/out-clave.txt" 2>&1
CLAVE_DESPUES=$(sqlite3 "$DIR/ssh_users.db" "SELECT password FROM users WHERE username='juan';")
chk "$([ "$CLAVE_ANTES" != "$CLAVE_DESPUES" ] && echo 1 || echo 0)" "cambió la clave ($CLAVE_ANTES → $CLAVE_DESPUES)"

echo "=== 7) CONSUMO POR CUENTA (con tráfico de prueba) ==="
sqlite3 "$DIR/trafico.db" "CREATE TABLE IF NOT EXISTS v2ray_user_traffic (id INTEGER PRIMARY KEY AUTOINCREMENT, username TEXT, rx_bytes INTEGER, tx_bytes INTEGER, fecha TEXT DEFAULT (datetime('now')));"
sqlite3 "$DIR/trafico.db" "CREATE TABLE IF NOT EXISTS ssh_user_traffic (id INTEGER PRIMARY KEY AUTOINCREMENT, username TEXT, rx_bytes INTEGER, tx_bytes INTEGER, fecha TEXT DEFAULT (datetime('now')));"
sqlite3 "$DIR/trafico.db" "CREATE TABLE IF NOT EXISTS trafico (id INTEGER PRIMARY KEY AUTOINCREMENT, protocolo TEXT, src_ip TEXT, rx_bytes INTEGER DEFAULT 0, tx_bytes INTEGER DEFAULT 0, conexiones INTEGER DEFAULT 1, fecha TEXT DEFAULT (datetime('now')));"
sqlite3 "$DIR/trafico.db" "INSERT INTO v2ray_user_traffic (username,rx_bytes,tx_bytes) VALUES ('lucas',1500000000,500000000);"
sqlite3 "$DIR/trafico.db" "INSERT INTO ssh_user_traffic (username,rx_bytes,tx_bytes) VALUES ('juan',200000000,100000000);"
sqlite3 "$DIR/trafico.db" "INSERT INTO ssh_user_traffic (username,rx_bytes,tx_bytes) VALUES ('lucas',300000000,100000000);"
sqlite3 "$DIR/trafico.db" "INSERT INTO trafico (protocolo,src_ip,rx_bytes,tx_bytes,conexiones) VALUES ('sshgo','::ffff:1.2.3.4',5000000,1000000,3);"
printf '21\n' | $BIN > "$DIR/out-consumo.txt" 2>&1
chk "$(grep -q "CONSUMO POR CUENTA" "$DIR/out-consumo.txt" && echo 1 || echo 0)" "muestra el consumo por cuenta"
chk "$(grep -q "1.86 GB" "$DIR/out-consumo.txt" && echo 1 || echo 0)" "ve los 1.86 GB de lucas"
chk "$(grep -q "PASADO" "$DIR/out-consumo.txt" && echo 0 || echo 1)" "lucas (bajo el tope de 50) no está PASADO"
grep -E "lucas|juan" "$DIR/out-consumo.txt" | sed 's/^/     /'

echo "=== 8) LÍMITE POR CUENTA (31) y RESET (32) ==="
printf '31\nlucas\n100\n' | $BIN > "$DIR/out-limite.txt" 2>&1
chk "$(grep_q "$DIR/v2ray_users.db" "SELECT limit_gb FROM users WHERE username='lucas' AND limit_gb=100;")" "puso el tope de 100 GB"
chk "$(grep -q "100" "$DIR/out-limite.txt" && echo 1 || echo 0)" "avisó el cambio"
printf '32\nlucas\ns\ns\n' | $BIN > "$DIR/out-reset.txt" 2>&1
chk "$(grep -q "contador en cero" "$DIR/out-reset.txt" && echo 1 || echo 0)" "reseteó el contador"
chk "$(grep_q "$DIR/trafico.db" "SELECT COUNT(*) FROM v2ray_user_traffic;")" "…"
chk "$([ "$(sqlite3 "$DIR/trafico.db" "SELECT COUNT(*) FROM v2ray_user_traffic WHERE username='lucas';")" = "0" ] && echo 1 || echo 0)" "no quedan filas de lucas en V2Ray"
chk "$([ "$(sqlite3 "$DIR/trafico.db" "SELECT COUNT(*) FROM ssh_user_traffic WHERE username='lucas';")" = "0" ] && echo 1 || echo 0)" "también reseteó el de SSH cuando dijo que sí"
chk "$([ "$(sqlite3 "$DIR/trafico.db" "SELECT COUNT(*) FROM ssh_user_traffic WHERE username='juan';")" = "1" ] && echo 1 || echo 0)" "no tocó el contador de otro usuario (juan)"

echo "=== 9) ELIMINAR (con confirmación) ==="
printf '9\njuan\nn\n' | $BIN > "$DIR/out-no.txt" 2>&1
chk "$(grep -q "cancelado" "$DIR/out-no.txt" && echo 1 || echo 0)" "si dice NO, no borra"
chk "$(grep_q "$DIR/ssh_users.db" "SELECT username FROM users WHERE username='juan';")" "el usuario sigue ahí"
printf '9\njuan\ns\n' | $BIN > "$DIR/out-si.txt" 2>&1
chk "$([ "$(sqlite3 "$DIR/ssh_users.db" "SELECT COUNT(*) FROM users WHERE username='juan';")" = "0" ] && echo 1 || echo 0)" "si dice SÍ, lo borra"

echo "=== 11) ESQUEMA VIEJO (base sin columnas nuevas) ==="
sqlite3 "$DIR/viejo.db" "CREATE TABLE users (id INTEGER PRIMARY KEY AUTOINCREMENT, username TEXT UNIQUE NOT NULL, password TEXT NOT NULL, created_at TEXT, expires_at TEXT, activo INTEGER DEFAULT 1);"
sqlite3 "$DIR/viejo.db" "INSERT INTO users (username,password,expires_at,activo) VALUES ('viejito','pass123','2027-01-01 00:00:00',1);"
GM_SSH_DB="$DIR/viejo.db" $BIN listar > "$DIR/out-viejo.txt" 2>&1
chk "$(grep -q "viejito" "$DIR/out-viejo.txt" && echo 1 || echo 0)" "lista un usuario de una base con esquema VIEJO (sin limit_unidad)"
chk "$(grep -q "(sin usuarios)" "$DIR/out-viejo.txt" && echo 0 || echo 1)" "no dice (sin usuarios) teniendo uno"

echo "=== 10) MENÚ COMPLETO (dibujo) ==="
printf '0\n' | $BIN > "$DIR/out-menu.txt" 2>&1
chk "$(grep -q "MENÚ PRINCIPAL" "$DIR/out-menu.txt" && echo 1 || echo 0)" "dibuja el menú"
chk "$(grep -q "31)" "$DIR/out-menu.txt" && echo 1 || echo 0)" "tiene la opción nueva 31"
chk "$(grep -q "32)" "$DIR/out-menu.txt" && echo 1 || echo 0)" "tiene la opción nueva 32"
chk "$(grep -c ")" "$DIR/out-menu.txt" | awk '{print ($1>=28)?1:0}')" "están las 30 opciones + las 2 nuevas"

echo
echo "════════ RESUMEN ════════"
echo "  ✅ $ok pruebas OK"
if [ "$ko" != "0" ]; then echo "  ❌ $ko FALLARON"; exit 1; fi
echo "  TODAS PASARON ✅"
