# 🦇 ghost-manager-go

**Administrador de accesos VPN con portero Zero-Rating, todo en Go.**

Dos programas, un solo lenguaje, sin dependencias raras (SQLite puro, sin cgo):

| Programa | Qué hace | Dónde escucha |
|---|---|---|
| **ghostprox** | El **portero**: recibe la conexión en el `:80`, adivina el protocolo por el payload, la manda al backend y **cuenta los GB** de cada IP y de cada cuenta. Aplica cuotas y corta. | `:80` (configurable) |
| **ghost-manager** | El **menú**: crear/renovar/cortar usuarios, armar configs (.ovpn, links V2Ray, Shadowsocks…), ver consumos, instalar servicios. | — (interactivo) |

> Firma: `ghost-manager v29.0-go "ESPELHO" by CHARLY_TRICKS`

---

## 📜 Historia (cómo se llegó acá)

1. **v1.0 — el pymanager (Python).** Un proxy en `:80` que adivinaba el protocolo y lo mandaba al backend. Sin menú, sin cuentas, sin contador. Un bug: nunca contaba la subida (`tx=0`).
2. **ghostprox v1.1 "TAXÍMETRO" (Go).** Port del portero a Go **conservando el mismo esquema de datos**. Suma: contador real de tráfico (rx+tx), límite de conexiones por IP y corte de cuentas V2Ray por vencimiento/baja.
3. **ghostprox v1.2 "BALANZA" (Go).** Cuenta los GB **por CUENTA** de V2Ray (no solo por IP), corta por consumo, y agrega los comandos `limite` y `reset`.
4. **ghost-manager v29.0-go "ESPELHO" (Go).** El menú de administración (antes un script bash de 2.406 líneas y 46 funciones) **portado completo a Go**: mismas opciones, mismos textos, más dos nuevas.

### ¿Por qué solo el portero y el menú están en Go?

Los **backends** siguen siendo los de siempre y no se tocan (es la regla del proyecto):

| Puerto | Quién atiende | Qué es | Lenguaje |
|---|---|---|---|
| :80 | **ghostprox** | el portero + contabilidad + cuotas | ✅ Go (este repo) |
| :2200 | sshgo | cuentas SSH (banner por usuario, corte por vencimiento/cuota) | Go (de terceros) |
| :8443 / :8388 / :10001 | xray | VLESS + Shadowsocks + VMess | Go (Xray oficial) |
| :2223 | psiphon-server | Psiphon | Go (oficial) |
| :1194 | openvpn | OpenVPN | C |
| :444 | dropbear | SSH liviano | C |
| :8880 / :8080 | hcr-server | método HCR (HTTP Custom) | binario propio |
| :7300 | badvpn-udpgw / bilola-btun | UDP | C / binario propio |
| :5300/:5301 | slowdns + socat | DNS tunneling | C |
| :3128 | squid | proxy | C |
| :18999 | brook | Brook | Go |

Reescribir xray, openvpn o psiphon sería tirar tiempo y arriesgar clientes: **el valor no estaba ahí**, estaba en el que decidía, contaba y cortaba (el pymanager), que **no existía en Go**.

---

## ⚙️ Cómo funciona el portero

```
cliente (HTTP Custom / Dtunnel / SSH / Psiphon…)
        │  payload en el :80
        ▼
   ghostprox  ── adivina el protocolo (heurística sobre los primeros bytes + payload)
        │         limpia el payload si corresponde
        │         cuenta rx_bytes / tx_bytes
        │         guarda: protocolo, IP, (cuenta si es VLESS)
        ▼
   backend (sshgo :2200 · xray :8443 · psiphon :2223 · openvpn :1194 · …)
```

**Identificación de la cuenta:** en **VLESS** el primer paquete trae el UUID en claro → se compara contra
`v2ray_users.db` y así se sabe **de quién** es el tráfico. Un UUID desconocido **siempre pasa** (nunca se
corta a un cliente legítimo sin registro). En **VMess o VLESS+TLS** el UUID va cifrado → ese tráfico solo
se puede contar **por protocolo/IP**, no por cuenta. Es una limitación real, no un bug.

**Cuotas:**
- **Por IP** (`quota_gb`): apagada por defecto. Motivo concreto: las operadoras usan **CGNAT** (muchos clientes comparten la misma IP pública) → cortar "por IP" rompería a clientes inocentes.
- **Por cuenta V2Ray** (`v2ray_quota_gb`): es la que se usa para vender GB. El corte es un HTTP 403 en el portero.
- **Límite por cuenta** (`limit_gb` en la base): tope individual, pisa la cuota general.

---

## 🚀 Instalación en un VPS nuevo

```bash
# como root
bash <(curl -fsSL https://raw.githubusercontent.com/TU-USUARIO/TU-REPO/main/scripts/instalar.sh)
```

O clonando:

```bash
git clone https://github.com/TU-USUARIO/TU-REPO.git ghost-manager-go
cd ghost-manager-go
sudo bash scripts/instalar.sh
```

El instalador:

1. chequea **Go 1.19+** (si falta, intenta instalarlo con apt/dnf/apk),
2. **compila** los dos programas (`CGO_ENABLED=0`, binario estático),
3. los instala en `/usr/local/bin/` (con respaldo `.bak` si ya existían),
4. crea `/etc/ctmanager/config/` y el `config.json` con valores razonables,
5. instala y arranca el servicio **`ghostprox.service`** (el portero),
6. te deja el menú listo: escribís **`ghost-manager`** y entrás.

Opciones: `--solo-menu` · `--solo-portero` · `--puerto 8080` · `--sin-servicio` (no crea systemd) · `--desinstalar`.

> ⚠️ **Los backends no vienen incluidos.** El instalador deja el portero y el menú. Los backends
> (sshgo, xray, psiphon, openvpn, hcr, udp-custom…) se instalan aparte; el propio menú tiene opciones
> para instalarlos bajando los binarios de **tus** URLs.

---

## 🖥️ El menú (`ghost-manager`)

```
1) 📊 Estado del sistema            19) 📡 Ver conexiones en vivo (con PAÍS)
2) 👤 Crear usuario SSH             20) 📊 Ver consumo (cuota por IP / por cuenta)
3) 🛡️ Crear usuario OpenVPN         21) 📊 Consumo por CUENTA (SSH + V2Ray)
4) 🚀 Crear usuario V2Ray           22) 📊 Consumo Psiphon por IP
5) 🛰️ Crear usuario UDP Custom       31) 📏 Poner/quitar el límite de GB de una cuenta  ← NUEVO
6) 👥 Listar usuarios               32) 🧹 Resetear el contador de GB (al renovar)      ← NUEVO
7) 🔄 Renovar (más días)            23) 🤖 Ayuda IA
8) ✏️ Editar (plan/clave/límite)    24) 🗑️ Eliminar usuario VLESS
9) 🗑️ Eliminar (cortar acceso)      25) 🤖 Bot de administración Telegram
10) 🌐 Psiphon                      26) 🔑 API Key ePro
11) 🔐 WireGuard                    27) 🚀 Instalar HCR Server
12) 🌐 Panel web :8303              28) 🦇 Protocolos Bilola (BHTTP/XHTTP/BTUN)
13) 🎨 Banner SSH (5 plantillas)    29) 🔄 Actualizar
14) 🕶️ Link Shadowsocks            30) 🛰️ Instalar UDP Custom
15) 🕶️ Regenerar password SS
16) 🕶️ Ver link SS actual
17) 🐢 Datos SlowDNS
18) 🐢 Crear usuario SlowDNS
```

Comandos directos (sin entrar al menú):

```bash
ghost-manager estado     # estado del sistema
ghost-manager listar     # usuarios SSH y V2Ray con consumo y límite
ghost-manager consumo    # consumo por cuenta + top IPs (con país)
ghost-manager --version  # firma
ghost-manager --firma    # cartel
ghost-manager --ayuda    # ayuda
```

---

## 📊 El portero por consola

```bash
ghostprox cuentas                  # SSH + V2Ray, con GB usados y límite
ghostprox cuentas v2ray            # solo V2Ray
ghostprox cuentas ssh              # solo SSH
ghostprox top                      # tráfico por protocolo + top 10 IPs
ghostprox limite <usuario>         # cuánto gastó y qué tope le toca
ghostprox limite <usuario> <GB>    # le pone tope de GB
ghostprox limite <usuario> 0       # le saca el tope propio (vuelve a la cuota general)
ghostprox reset <usuario>          # muestra el consumo (NO borra)
ghostprox reset <usuario> --confirmo   # contador en cero (renovación)
ghostprox --version | --firma | --ayuda
```

### `config.json`

```json
{
  "quota_gb": 0,                    // cuota por IP en GB (0 = sin límite; apagada por CGNAT)
  "quota_dias": 30,
  "max_connections_per_user": 50,   // conexiones simultáneas por IP
  "v2ray_check_users": true,        // cortar cuentas V2Ray vencidas / dadas de baja
  "v2ray_quota_gb": 50,             // cuota por CUENTA de V2Ray en GB (0 = sin límite)
  "v2ray_quota_dias": 30
}
```

Se recarga **en caliente**: `kill -HUP $(pgrep -f ghostprox)` (el menú lo hace solo al guardar una cuota).

---

## 🧪 Pruebas

```bash
cd ghost-manager
GM_NO_SYSTEM=1 bash prueba-gm.sh     # 42 pruebas: menú, usuarios, consumos
GM_NO_SYSTEM=1 bash prueba-gm2.sh    # 42 pruebas: accesos, instaladores, extras
```

**84 pruebas** que **no tocan nada real**: usan bases y archivos de mentira en `/tmp`
(`GM_NO_SYSTEM=1` desactiva `useradd/chpasswd/cron/systemd/descargas`; las rutas se pisan con variables
`GM_*`). Se probaron localmente y **en el servidor real contra los backends reales**.

---

## 📁 Estructura

```
ghost-manager/     main.go · ui.go · datos.go · sistema.go · usuarios.go · consumos.go
                   estado.go · accesos.go · instaladores.go · extras.go · geo.go · helpers.go
                   build.sh · build-estatico.sh · prueba-gm.sh · prueba-gm2.sh
ghostprox/         main.go (el portero v1.2 "BALANZA")
scripts/           instalar.sh
docs/              PLAN-ghostprox.md · PLAN-ghost-manager.md · COMO-SE-USA.md
```

---

## 🔒 Notas de seguridad y datos a completar

Este repositorio **no contiene datos de producción**: no hay claves, tokens, bases de datos ni binarios.
Lo que ves como placeholder **son valores que hay que poner con los propios**:

| Placeholder | Qué reemplaza |
|---|---|
| `TU-DOMINIO` | el dominio que apunta al VPS |
| `IP-DE-TU-VPS` | la IP pública del servidor |
| `TU-BUGHOST-1/2/3` | los hosts "fachada" del payload (no se publican) |
| `TU-SERVIDOR` | el servidor desde donde se baja el binario propio (HCR) |
| `TU-USUARIO/TU-REPO` | repositorio/s del que se bajan los binarios auxiliares (bilola, panel web, watchdog) |

Las opciones del menú que **instalan** cosas (HCR, Bilola, UDP Custom, panel web, auto-actualización)
**detectan primero y preguntan** antes de tocar nada, y respetan lo que ya está instalado y funcionando.

---

## Requisitos

- Linux (probado en Ubuntu 24.04) con `systemd`
- **Go 1.19+** sólo para compilar (los binarios son estáticos: no necesitan Go para correr)
- SQLite **no** hace falta instalarlo: el driver es puro Go

## Créditos

Portero y menú: port a Go del stack original del autor (pymanager en Python + menú en bash).
Los protocolos Bilola (BHTTP/XHTTP/BTUN) son de desarrollo original de su equipo.
