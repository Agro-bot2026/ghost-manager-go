# 🦇 GHOST-MANAGER EN GO — PLAN DE PORT (v29.0-go "ESPELHO")

> Fuente a portar: `/usr/local/bin/ghost-manager` (v28.0 · **2.406 líneas de bash** · **46 funciones** · 30 opciones).
> Objetivo del dueño: **todo en Go**, sin bash, para que quede un solo lenguaje y un solo binario.

## ✅ ESTADO: PORT COMPLETO (F1 + F2 + F3) — 82 pruebas OK

```
FASE 1 ✅ menú (30 opciones + 2 nuevas) · dashboard · estado · crear/listar/renovar/editar/eliminar
          SSH y V2Ray · consumos (conexiones en vivo con país, por IP, por cuenta, top, cuotas)
          → 40/40 pruebas OK (prueba-gm.sh)
FASE 2 ✅ OpenVPN (+.ovpn armado) · UDP Custom · SlowDNS · Psiphon · Shadowsocks · WireGuard
          panel web · banner SSH (5 plantillas + ANSI) → parte de las 42/42 (prueba-gm2.sh)
FASE 3 ✅ instaladores UDP Custom, Bilola (BHTTP/XHTTP/BTUN), HCR · bot Telegram · API ePro
          ayuda IA · auto-actualización · GeoIP local → 42/42 pruebas OK
TOTAL    ✅ 82/82 pruebas, todo con bases/archivos de mentira en /tmp (GM_NO_SYSTEM=1)
```

## 📁 ARCHIVOS

```
/root/ghost-go/gm/            (fuente Go: 11 archivos .go + go.mod)
   main.go        menú, argumentos, firma, recuadro
   ui.go          colores, banner EPRO.HC, entradas, barras
   datos.go       rutas, config.json, SQLite, cuentas SSH/V2Ray, consumo, SIGHUP al portero
   sistema.go     comandos, servicios, IP/dominio, usuarios del sistema, dashboard
   usuarios.go    crear/listar/renovar/editar/eliminar + links V2Ray
   consumos.go    conexiones en vivo, cuota por IP, consumo por cuenta, límite y reset de GB
   estado.go      estado + dashboard
   accesos.go     OpenVPN, UDP Custom, Psiphon, Shadowsocks, SlowDNS, WireGuard, banner
   instaladores.go UDP Custom / Bilola / HCR (detectan antes de instalar)
   extras.go      ayuda IA, bot Telegram, API ePro, auto-actualización
   geo.go         base de países para las conexiones
   helpers.go     descargas, chpasswd, crontab, aleatorios
   prueba-gm.sh / prueba-gm2.sh   las 82 pruebas
   build.sh / build-estatico.sh   compilación (estático, sin cgo)
   pase-menu-go.sh                pase al server con respaldo y vuelta atrás
   pase-menu-go.sh --activar      deja `ghost-manager` = versión Go
/root/ghost-go/docs/spec-instaladores.md   spec extraída del bash
/root/ghost-accesos-spec.md                spec extraída del bash
/root/ghost-go/ref/ghost-manager-teramont  copia del bash original (referencia)
```

## 🧪 PRUEBAS (todas sin tocar nada real)

```
GM_NO_SYSTEM=1   → no crea/borra usuarios del sistema ni toca cron/sshd
GM_NO_CLEAR=1 · GM_SIN_COLOR=1 · GM_SIN_UPDATE=1
GM_*_DB / GM_*_FILE → apuntan a bases y archivos de /tmp
Comando:  bash gm/prueba-gm.sh  ·  bash gm/prueba-gm2.sh
```

## 🔄 VUELTA ATRÁS

```
bash /root/ghost-go/volver-al-menu-bash.sh   → vuelve el menú al bash original
```

## DIFERENCIAS CONSCIENTES vs el bash (documentadas)

1. **`limit_unidad` vs `limit_unit`**: el bash escribía `limit_unidad`, sshgo lee `limit_unit`
   (por eso el banner del cliente mostraba siempre GB). El Go escribe **las dos**.
2. **Instaladores**: el bash reinstala sin preguntar; el Go **detecta primero y pregunta**
   (y en HCR respeta el `if` de "ya instalado" — el bash tenía un bloque duplicado que
   hacía que **siempre** re-descargara).
3. **Menú del pymanager**: las 4 partes desenganchadas (estado :80, conexiones, servicios,
   guardar cuota) ahora miran **ghostprox** y la cuota se aplica con **SIGHUP** (sin cortar a nadie).
4. **Auto-actualización (opción 29)**: no se puede "actualizar" un binario Go desde el repo del
   bash; el Go avisa si hay bash nuevo en GitHub y **pide confirmación** antes de pisar nada.
5. **Xray**: sigue usando `/usr/local/bin/xray_uuid.sh` (probado en producción).


> Fuente a portar: `/usr/local/bin/ghost-manager` (v28.0 · **2.406 líneas de bash** · **46 funciones** · 30 opciones).
> Objetivo del dueño: **todo el menú y todos los protocolos en Go**, sin perder nada de lo que ya hacía.
> Copia del fuente leída: `/root/ghost-go/ref/ghost-manager-teramont` (relevamiento hecho el 29/09).

---

## 📋 INVENTARIO DEL FUENTE (qué hay que portar)

### Núcleo del menú
| Función | Qué hace |
|---|---|
| `menu()` | el menú principal: 30 opciones en secciones (USUARIOS · SERVICIOS · CONSUMOS · HERRAMIENTAS · TELEGRAM · MÉTODO NUEVO) |
| `banner()`, `dashboard()`, `barra_uso()`, `celda()` | cartel EPRO.HC + panel de arriba (usuarios activos, vencidos, servicios) |
| `estado()`, `get_ip()`, `get_domain()` | estado del sistema, IP pública (curl ifconfig.me), dominio |
| `check_update()`, `actualizar_ghost_manager()` | se auto-actualiza desde `TU-USUARIO/TU-REPO` (GitHub) |

### Usuarios (8)
| Función | Toca |
|---|---|
| `crear_ssh()` | `useradd`+`chpasswd`+`chage -E`, `/etc/ssh/sshd_config.d/<user>.conf` (MaxSessions), `pam_limits` + `/etc/security/limits.conf` (maxlogins), `ssh_users.db` (username, password, expires_at, **tipo** solo/multi, **max_conn**, **limit_gb**, **limit_unidad**) |
| `crear_v2ray()` | `v2ray_users.db` (username, uuid, expires_at) + config de xray |
| `crear_openvpn()` | usuario de sistema + `.ovpn` armado (`/etc/ctmanager/ovpn/<user>.ovpn` con CA embebido) |
| `crear_udpcustom()` | usuario udp (`udp1`/`udp2`…) |
| `crear_slowdns()` | usuario + datos del nameserver/public key |
| `crear_psiphon()` | datos del psiphon (regiones, upstream) |
| `listar()` | tabla de usuarios (SSH + V2Ray) con días restantes y barra de uso |
| `renovar()`, `editar()`, `eliminar()`, `eliminar_vless()` | UPDATE/DELETE en las dos bases + usuario de sistema + clave |

### Configuraciones y links (6)
`ver_ovpn()`, `link_shadowsocks()`, `regenerar_shadowsocks()`, `ver_link_shadowsocks()`, `datos_slowdns()`, `editar_banner()`
(esta última escribe `sshgo_banner.txt` con plantillas de banner — el que muestra EPRO.HC)

### Consumos (4 + GeoIP)
`ver_conexiones()` (lee el log del proxy, agrupa por IP, **geolocaliza el país**), `ver_consumo()` (cuota por IP desde config.json), `ver_consumo_vless()` (top por uuid), `ver_consumo_psiphon()`
+ `ensure_geo_db()`, `geo_of()`, `flag_of()` (base GeoIP local + banderitas)

### Servicios e instaladores (7)
`estado_wg()`, `panel_web()` (puerto 8303), `bilola_estado()`, `instalar_hcr()` (baja el binario de `TU-SERVIDOR/hcr`), `bilola_instalar_bhttp/xhttp/btun()` (bajan binarios + unidades systemd + abren puertos), `instalar_udpcustom()` (baja binario UDP Custom + 2 unidades + script de excepciones), `menu_bilola()`

### Extras (3)
`configurar_bot_telegram()` (unidad `ghost-bot-admin.service`), `configurar_api_epro()` (`/etc/ghost-license/epro-api.key`), `ghost-ai-help` (ayuda IA, otro binario)

### Dependencias externas que usa hoy (a reemplazar por Go o a conservar)
`openssl rand` → `crypto/rand` · `python3` (decimales) → matemática de Go ·
`sqlite3` → driver puro en Go · `useradd/chpasswd/chage` → se conservan (son del sistema) ·
`systemctl`/`journalctl` → `os/exec` · `curl` (IP, updates, descargas) → `net/http` ·
GeoIP → misma base local.

---

## 🎯 FASES (cada una se prueba y se entrega andando)

### FASE 1 — Menú + Usuarios + Estado + Consumos  ⭐ (lo que usás todos los días)
- Menú con las mismas secciones y los mismos números (sin cambiar tu costumbre)
- Dashboard/estado con los servicios **reales** (y el `:80` mirando a **ghostprox**, no al pymanager)
- Crear/listar/renovar/editar/eliminar **SSH** (con tipo solo/multi, max_conn, límite MB/GB/TB)
- Crear/listar/renovar/eliminar **V2Ray** (uuid + edición del config de xray + recarga)
- Consumos: conexiones en vivo (del log de ghostprox) con país, consumo por IP, **consumo por CUENTA (V2Ray)**, top IPs, y manejo de cuotas (por IP y por cuenta, recarga por SIGHUP)

### FASE 2 — Los otros accesos
OpenVPN (usuario + `.ovpn`), UDP Custom, SlowDNS, Psiphon, Shadowsocks (link/regenerar/ver), WireGuard (estado), banner SSH, panel web

### FASE 3 — Instaladores + extras
HCR, Bilola (BHTTP/XHTTP/BTUN), instalador de UDP Custom, bot Telegram, API ePro, ayuda IA, auto-actualización
⚠️ En Go, los **instaladores** van a **detectar primero** y preguntar antes de instalar/reinstalar (hoy el bash reinstala sin preguntar: es la parte con más riesgo de romper lo que ya anda)

### Cierre
- El `ghost-manager` (comando que escribís) pasa a ser el **binario Go**; el bash queda como `ghost-manager.bash` (respaldo, un comando para volver)
- Firma y versión: **`ghost-manager v29.0-go "ESPELHO" by CHARLY_TRICKS`** (`--version` / `--firma`)

---

## 🔎 DOS COSAS QUE ENCONTRÉ Y CONVIENE ARREGLAR EN EL PORT
1. **`limit_unidad` vs `limit_unit`**: el menú escribe `limit_unidad` y **sshgo lee `limit_unit`** → el banner puede estar mostrando la unidad equivocada (siempre GB). En el port escribo las dos columnas, así el banner dice lo correcto.
2. **El menú mira al `pymanager` en 4 lugares** (estado del :80, conexiones en vivo, lista de servicios y al guardar la cuota) → con el `:80` en ghostprox esos 4 quedan "mudos". El port los apunta a ghostprox.

## 🛡️ REGLAS (las mismas de siempre)
- El `:80` no se toca al portar (el menú es otro programa)
- Nada de reinstalar servicios al aire: se detecta, se avisa y se pregunta
- Cada fase probada con bases de prueba en `/tmp` + verificación de que producción quedó intacta
- El bash original **no se borra** hasta que el Go esté completo y probado
