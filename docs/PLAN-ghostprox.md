# 🚀 GHOST MANAGER EN GO — PLAN Y ESTADO

> Proyecto: pasar el "pymanager" (Python, 399 líneas) a **Go**,
> manteniendo TODO por el :80 (zero-rating) y sin tocar los demás servicios.
>
> Estado: **FASE 1 y FASE 2 (v1.1) LISTAS** — la v1.1 probada en el :8888, falta el pase al :80.

---

## 📦 LO QUE HAY HOY (relevado en Teramont 28/09/26)

### Servicios que corren (NO se tocan, son binarios aparte)
```
badvpn.service        → udpgw (UDP, para el ZR)
hcr-server-8080.service / hcr-server.service
openvpn@server.service  → :1194 (necesita el NAT 10.8.0.0/24 ✅)
psiphon.service         → IP-DE-TU-VPS:2223
pymanager.service       → EL QUE VAMOS A REESCRIBIR (:80)   (hoy APAGADO)
squid.service           → :8080
ssh.service             → :22 (sshd)
sshgo.service           → :2200 (las cuentas de los clientes)
xray.service            → :8443 (raw) · :8388 (ss) · :10001 (vmess)
ghostprox.service       → :80  (el pymanager en GO — v1.0.0 en producción)
```

### El pymanager (a reescribir)
```
/etc/ctmanager/websocket/proxy.py   → 399 líneas · "v17 by CHARLY_TRICKS"
/etc/ctmanager/websocket/config.json
Copia local: /root/ghost-go/ref/proxy.py  +  config.json
```

### Estructura del proxy.py (lo que hay que portar)
```
load_config()            → lee config.json
_registrar_trafico()     → MB por protocolo / sesión
_consumo_ip(dias=30)     → consumo por IP
_quota_excedida(cfg)     → control de cuota (quota_gb)
forward(src, dst)        → el bombeo bidireccional
split_http_header()      → separa el pedido HTTP
extraer_uuid_vless()     → saca el UUID del primer paquete
es_handshake_brook()     → detecta Brook (WS /ws)
detect_target(...)       → ⭐ EL CORAZÓN: decide el backend
```

### Lógica de `detect_target` (a replicar en Go)
```
1) puerto_payload (X-Online-Host) → si es 22 → sshd
                                   → si es 80/8080/8880 → sshgo
2) no sshgo y no payload pero hay dropbear → dropbear :444
3) BHP1 / Dtunnel (Frontera) → protocolo propio
4) startswith "SSH-2.0-Go" / "SSH-2.0-Psiphon" → psiphon
5) SSH normal → sshgo/sshd
6) vless (uuid) / shadowsocks / brook → sus puertos
```

---

## 🎯 EL PLAN

### FASE 1 — El corazón en Go (lo más valioso) ✅ HECHA (v1.0.0 "TUBO")
```
· Binario Go que escucha en un puerto de prueba (:8888) ✅ seguro
· Lee el payload (legacy_loopback-style) y detecta la ruta ✅
· LIMPIA los encabezados del payload antes del backend ✅
  → ¡esto es lo que faltaba para el v2ray/vmess! ("invalid request version")
· io.Copy bidireccional (2 goroutines por conexión) ✅
· Logs de diagnóstico (igual que hoy) ✅
```

### FASE 2 — Cuentas y cuota ✅ HECHA (v1.1.0 "TAXÍMETRO", ver el final)
```
· Lee los usuarios (misma fuente que hoy) ✅
· _registrar_trafico / _consumo_ip / _quota_excedida → en Go ✅
· Logs y estadísticas ✅
```

### FASE 3 — El cambio (sin cortar clientes)
```
· Probar en :8888 con TODO ✅ (hecho)
· Parar pymanager → arrancar el Go en el :80  (v1.0.0 ✅ HECHO · v1.1 PENDIENTE)
· El Python queda de respaldo (1 comando para volver) ✅
```

---

## 🛡️ REGLAS DE ORO
```
✅ TODO por el :80 (zero-rating)
✅ Los puertos de los backends NO se tocan
✅ Probar en :8888 ANTES de tocar el :80
✅ Siempre con vuelta atrás (systemctl start pymanager)
✅ Si el Ghost manager anda: no romper lo que ya sirve
```

---

## 📊 DATOS DEL SERVER
```
Teramont: IP-DE-TU-VPS · 1 núcleo ⚠️ · 1932 MB RAM · 26 GB libres
Go: go1.19.8 (el del apt) — alcanza; SQLite puro en Go, sin cgo
Puertos libres para probar: 8888 · 8899 · 9000
Firewall (nft): abre 22,80,443,1194,2200,2223,8388,8443,8080,8880,7300 + UDP
  (el :8888 NO está abierto hacia afuera → las pruebas se hacen DESDE el server)
NAT: nft masquerade 10.8.0.0/24 (¡el que hace navegar al OpenVPN!)
SSH: puerto 22 con la llave /root/.ssh/id_hermes_teramont (cada reset la borra)
```

## 📚 LECCIONES DE LA SESIÓN ANTERIOR (28/09/26)
```
· Salome/ws-epro: el CF-RAY necesita legacy_loopback: true
· v2ray+payload: debe ser VMESS :10001 (el vless raw falla)
· Salome NO limpia los encabezados → el v2ray no anda (por eso el Go)
· El pymanager SÍ adivina el protocolo (por eso funciona con todo)
· HTTP Custom NO sanea el payload (GET → "T"; GEGET pasa literal)
· Cada reset: borra la llave SSH de Hermes → hay que reagregarla
```

---

## ✅ FINAL: 2026-09-28 20:47 — ¡EL GO ANDA!

**Estado productivo (verificado con la app del dueño):**
- `:80` = **ghostprox (GO)** ✅ habilitado (arranca solo). El python quedó APAGADO.
- `:8443` = xray (inbound VLESS raw) ✅
- El dueño probó: **conecta con WiFi ✅ y con chip Personal ✅** ("V2Ray: conectado correctamente")

**Los 4 bugs que faltaban (encontrados comparando con el pymanager):**
1. **El banner falso**: yo mandaba `SSH-2.0-Go` después del 101. El pymanager manda
   SOLO el 101 (su config `"payload": "101"`). Ese banner rompía a la app.
2. **La regla 6 del detect_target**: si el dato empieza con `0x00`/`0x01` → **V2Ray**
   (vless RAW). Yo lo mandaba a sshgo/shadowsocks.
3. **datosUtiles**: si no había dato, yo devolvía el payload (el pymanager deja VACÍO
   → eso va al ssh).
4. **El corte del payload**: el payload trae VARIOS `\r\n\r\n`; hay que buscar hacia
   atrás el que SÍ arranca con un protocolo (soporta el payload del chip y del wifi).

**Datos del flujo correcto (copia fiel del pymanager):**
- Contesta `HTTP/1.1 101 Switching Protocols\r\nUpgrade: websocket\r\nConnection: Upgrade\r\n\r\n` (SOLO eso).
- Espera y junta el dato (varios paquetes).
- Detecta por el DATO (`split_http_header` con **rfind** = el ÚLTIMO `\r\n\r\n`).
- Al backend le manda **SOLO el dato** (`dest.sendall(first_packet)`).
- El SSH: el payload manda el puerto (`X-Online-Host: ip:22` → sshd :22; :80 → sshgo :2200).
- El SS es el FALLBACK (regla 7): todo lo no detectado va a :8388.

**Vuelta atrás:** `bash /root/ghost-go/volver-al-pymanager.sh`

## 🎯 ROADMAP para superar a salome ws-epro

**Firma/versión: HECHA** — `ghostprox v1.0.0 "TUBO" by CHARLY_TRICKS`
(`--version` y `--firma`; y sale en el log al arrancar). FIRMA=0 la apaga.

**v1.2 — TLS :443 (WSS)** para cubrir también el camino del :443.

**v1.3 — Extras**: panel de estadísticas en vivo, recarga de config sin
reiniciar (SIGHUP), y los logs por usuario.

### Cómo verificar cada cosa (sin romper nada)
- Se prueba en el :8888 primero, y recién después se pasa al :80.
- Vuelta atrás siempre: `bash /root/ghost-go/volver-al-pymanager.sh`

---

## ✅ v1.1.0 "TAXÍMETRO" — CUOTA + CUENTAS (EN PRODUCCIÓN desde el 29/09)

**Dónde está:** `/root/ghost-go/v11/` (main.go · go.mod · go.sum · binario `ghostprox-v11`)
**Estado:** ✅ **EN PRODUCCIÓN en el :80 desde el 29/09 00:37** (pase hecho con respaldo y
vuelta atrás: `respaldos-20260928-213746/ghostprox-v1.0.0.bak`).
**Config activa:** `quota_gb = 0` (sin cuota) · `max_connections_per_user = 50` ·
`v2ray_check_users = true` (corta cuentas V2Ray vencidas/inactivas).

### Qué agrega (lo que el python hacía y el Go no)
1. **Lee la config del pymanager** (`/etc/ctmanager/websocket/config.json`): puertos,
   `payload`, `quota_gb`, `quota_dias` y `max_connections_per_user`.
   **SIGHUP** = recarga la config (cuota y límites) sin cortar a nadie.
2. **Contador de tráfico** (`_registrar_trafico` + `_consumo_ip`): suma rx/tx por
   protocolo + IP en `/etc/ctmanager/config/trafico.db`, tabla `trafico` (MISMA DDL y
   misma IP en formato `::ffff:1.2.3.4`, para que los totales sigan sumando).
   → esto era lo que quedó **congelado el 28/09 23:13** al pasar a Go: el panel dejó de ver tráfico nuevo.
   → de paso ahora **también cuenta la subida** (el python dejaba tx_bytes en 0 siempre).
3. **Cuota por IP** (`_quota_excedida`): si la IP pasó `quota_gb` en `quota_dias` días
   (30 por defecto) → contesta `HTTP/1.1 403 Forbidden` y corta, con el aviso en el log.
   `quota_gb = 0` (o sin la clave) = SIN límite, igual que antes.
4. **`max_connections_per_user`**: tope de conexiones simultáneas por IP (hoy 50).
5. **CUENTAS** (esto el python NO lo tenía):
   - `ghostprox cuentas` → usuario · estado · vence · días · usado · límite · conexiones
   - `ghostprox top` → tráfico por protocolo + top 10 de IPs (en MB o GB)
   - al arrancar avisa: `👥 cuentas: N activas de M`
   - lee la MISMA fuente que sshgo: `/etc/ctmanager/config/ssh_users.db`
6. **Cuentas de V2Ray** (opcional, `v2ray_check_users: true` en config.json, **apagado por
   defecto**): al VLESS le saca el UUID del primer paquete y lo busca en
   `v2ray_users.db` → corta las cuentas vencidas/inactivas. UUID desconocido = NO se corta.

**SQLite: driver puro en Go** (`modernc.org/sqlite v1.23.1`, compila con el Go 1.19.8 del
VPS) → **no se instaló NADA** en el servidor (ni apt, ni cgo, ni libsqlite3-dev).

### Pruebas hechas (medidas, no "debería andar")
- **Local: 21/21 OK** — contador, cuota+403, recarga por SIGHUP, límite de conexiones,
  cuentas de V2Ray (vencida/inactiva/desconocida), comandos `cuentas` y `top`.
  (`/root/ghost-go/prueba_v11.py`; usa bases en `/tmp`, no toca nada real)
- **Teramont :8888 (backends REALES)** — SSH por el payload → `127.0.0.1:2200` y contesta
  el sshgo (`SSH-2.0-Go` + KEX); VLESS raw → `127.0.0.1:8443` (xray). Se anotaron las filas
  (`sshgo … rx=16 tx=764` y `v2ray … rx=25`) en una base de prueba (`/tmp`).
- **La base de producción quedó intacta**: 241 filas, última 28/09 23:13.
- **El :80 nunca se tocó**: siguió el `ghostprox` v1.0.0 (pid 176402) todo el tiempo.

### El pase al :80 — HECHO ✅ (29/09 00:37) con `pase-v1.1-al-80.sh`
```
El script: respalda (binario + config + unidad + volver-al-pymanager.sh) →
activa v2ray_check_users → PARA el servicio → copia el binario → arranca →
verifica is-active + "v1.1.0" en el log; si falla, repone el .bak solo.
⚠️ PITFALL: con el servicio andando, `cp` sobre el binario falla con "Text file busy"
   (por eso el script lo para ANTES de copiar).
Verificado en producción (desde afuera, 29/09 00:38):
  · SSH por el payload → sshgo:2200 (banner SSH-2.0-Go + KEX) ✅
  · VLESS raw → xray:8443, con el UUID REAL de la cuenta activa (lucas) pasa ✅
  · 3 filas nuevas en trafico.db (sshgo rx=16 tx=764 · v2ray rx=25, IP ::ffff:IP-DE-TU-VPS) ✅
  · todos los servicios activos y carga 0.01 (todo igual que antes) ✅
Vuelta atrás: `systemctl stop ghostprox; cp -a respaldos-<fecha>/ghostprox-v1.0.0.bak ghostprox;
systemctl start ghostprox`  ·  del todo: `bash /root/ghost-go/volver-al-pymanager.sh`
```
**Opcional (para cobrar por GB):** poner `quota_gb` en config.json.
⚠️ OJO: la cuota por IP es peligrosa con las operadoras (CGNAT: muchos clientes comparten
una IP). La cuota POR PERSONA ya la corta sshgo (por usuario) — ahí es donde conviene cobrar.
Para V2Ray, la v1.2 debería contar los GB por UUID y cortar por consumo (hoy solo corta
por vencimiento/inactividad).


---

## ✅ v1.2.0 "BALANZA" — CUOTA POR CUENTA EN V2RAY (EN PRODUCCIÓN en el :80 desde el 29/09 00:52)

**Dónde está:** `/root/ghost-go/v12/` (main.go · go.mod · go.sum · binario `ghostprox-v12`)
**Config activa:** `v2ray_check_users=true` · `v2ray_quota_gb=50` · `v2ray_quota_dias=30`
(50 GB por cuenta cada 30 días; cada cuenta puede tener su propio `limit_gb` que manda sobre el general)
**Respaldos del pase:** `/root/ghost-go/respaldos-20260928-215412/` (binario v1.1, config, unidad,
las bases `trafico.db` y `v2ray_users.db`, y los dos main.go)
**Qué agrega (sobre la v1.1, que ya está en producción):**
1. **Consumo POR CUENTA en V2Ray**: cada sesión de VLESS se le anota a la cuenta (tabla nueva
   `v2ray_user_traffic` en trafico.db, espejo de la que usa sshgo para SSH). Antes se contaba
   solo por protocolo+IP → ahora se sabe "cuántos GB gastó cada cliente".
2. **Corte por consumo**: si la cuenta se pasa de su cuota → se corta la conexión y queda el
   motivo en el log. El límite sale de la columna **`limit_gb`** de `v2ray_users.db` (se
   agrega sola si la base es vieja, con ALTER TABLE aditivo: las cuentas que ya existen quedan
   en 0 = sin límite, así NADA cambia hasta que el dueño ponga un número). Si la cuenta no
   tiene límite propio, vale el general **`v2ray_quota_gb`** de config.json (0 = sin límite).
3. **`ghostprox cuentas`** ahora muestra las DOS secciones: sshgo (SSH) y V2Ray, con estado,
   vencimiento, días, GB usados y límite. `ghostprox cuentas ssh|v2ray` filtra.
4. **`ghostprox reset <usuario>`** → cuánto gastó esa cuenta de V2Ray (no borra nada);
   con `--confirmo` lo pone en cero (para cuando renueva).
5. El corte de la v1.1 (vencidas / dadas de baja) sigue igual.

**Cómo se identifica la cuenta:** el UUID viaja en el primer paquete de VLESS (versión 1B +
UUID 16B) y se busca en `v2ray_users.db`. Si el UUID es desconocido, o si el protocolo es
VMess/TLS (el uuid va cifrado), NO se corta: se cuenta solo por protocolo (nunca romper a un
cliente). Eso queda anotado como límite conocido de la v1.2.

### Pruebas medidas
- **Local: 28/28 OK** (`prueba_v12.py`): migración de base vieja, conteo por cuenta (rx y tx),
  corte por consumo, corte por vencimiento, uuid desconocido, cuota general, `reset` (con y sin
  `--confirmo`), y que el corte NO toque otras cuentas.
- **Sin regresiones:** la suite de la v1.1 (20 pruebas) pasa contra el binario v1.2 (solo falla
  el chequeo del nombre de versión).
- **Teramont :8888 con los backends REALES: 14/14 OK** (`prueba_v12_teramont.py`), usando una
  COPIA de `v2ray_users.db` con cuentas de prueba (la del dueño no se tocó):
   · la cuenta real del dueño (lucas) rutea al xray :8443, se le anota el consumo y NO se corta
   · test-libre pasa · test-tope se corta por consumo · test-vencida se corta por vencimiento
   · uuid desconocido pasa · el SSH sigue entrando al sshgo :2200
- **La migración ya corrió**: `v2ray_users.db` del dueño tiene la columna `limit_gb` (lucas = 0
  = sin tope). Respaldo: `/root/ghost-go/v2ray_users.db.bak-20260928-214817`.
- **Bug encontrado y arreglado en el camino**: con el esquema viejo (sin limit_gb) la búsqueda
  fallaba y el corte no se hacía nunca → ahora hay consulta de respaldo.

⚠️ **NUANCE conocida (vieja, viene del pymanager)**: el detector mira los primeros 8 bytes
buscando la firma de OpenVPN (0x38/0x08/0x28). Un UUID de VLESS *al azar* puede caer ahí
(~unos pocos %) y la conexión se rutea como OpenVPN. Es fiel al python (por eso no lo toqué:
OpenVPN de los clientes anda y no quiero romperlo). Si algún día molesta, se arregla
priorizando el 0x00/0x01 del inicio como VLESS.


### Comandos nuevos de la v1.2 (para el día a día)
```
/root/ghost-go/ghostprox cuentas              → SSH + V2Ray, con GB usados y límite
/root/ghost-go/ghostprox cuentas v2ray        → solo V2Ray
/root/ghost-go/ghostprox limite <usuario>     → cuánto gastó y qué límite le toca
/root/ghost-go/ghostprox limite <usuario> 100 → le pone 100 GB de tope a esa cuenta
/root/ghost-go/ghostprox limite <usuario> 0   → le saca el tope propio (vuelve al general)
/root/ghost-go/ghostprox reset <usuario>            → cuánto consumió (NO borra nada)
/root/ghost-go/ghostprox reset <usuario> --confirmo → lo pone en cero (al renovar)
```
Cambiar la cuota general sin reiniciar: editar `v2ray_quota_gb` en config.json y `kill -HUP <pid>`.

### Límites conocidos (honestos)
- La cuenta se identifica por el UUID del primer paquete: funciona en **VLESS**. En **VMess** o
  **VLESS+TLS** el uuid va cifrado → se cuenta por protocolo/IP pero NO por cuenta (no se corta).
- El consumo por cuenta se guarda en `v2ray_user_traffic` (trafico.db). `reset` borra SOLO esa tabla.

### Verificado en producción (29/09 00:52-00:54)
- SSH por el payload → sshgo:2200 ✅ · VLESS con el UUID real → xray:8443 ✅
- El log rutea identificando la cuenta: `🚇 Tunel <ip> -> 127.0.0.1:8443 (V2RAY · cuenta lucas)` ✅
- Consumo anotado por cuenta en la base real ✅ · `limite lucas` → "límite: 50.0 GB ← general" ✅
- Los 8 servicios activos, carga 0.07 ✅
