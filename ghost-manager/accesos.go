// ghost-manager en Go — accesos y configs: OpenVPN, UDP Custom, Psiphon, Shadowsocks,
// SlowDNS, WireGuard, panel web y banner SSH (FASE 2)
package main

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// ─── RUTAS DE LA FASE 2 (pisables por entorno para pruebas) ─────────────────
func rutaCAOpenVPN() string  { return env0("GM_CA_OVPN", "/etc/openvpn/easy-rsa/pki/ca.crt") }
func rutaSSPassword() string { return env0("GM_SS_PASS_FILE", RutaConfigDir+"/ss_password") }
func rutaSSUsersSh() string  { return env0("GM_SS_USERS_SH", "/usr/local/bin/ss_users.sh") }
func rutaSlowDNSServer() string {
	return env0("GM_SLOWDNS_BIN", "/etc/slowdns/sldns-server")
}
func rutaSlowDNSService() string {
	return env0("GM_SLOWDNS_SERVICE", "/etc/systemd/system/sldns-server.service")
}
func rutaSlowDNSPub() string    { return env0("GM_SLOWDNS_PUB", "/etc/slowdns/server.pub") }
func rutaPsiphonEntry() string  { return env0("GM_PSIPHON_ENTRY", "/etc/psiphon/server-entry.dat") }
func rutaPsiphonConfig() string { return env0("GM_PSIPHON_CFG", "/etc/psiphond/psiphond.config") }
func rutaPsiphonLicense() string {
	return env0("GM_PSIPHON_LIC", "/etc/ghost-license/psiphon-server-entry.dat")
}
func rutaBannerSSH() string { return env0("GM_SSH_BANNER", "/etc/ssh/banner") }

// ─── 3) CREAR USUARIO OPENVPN ───────────────────────────────────────────────
func crearOpenVPN() {
	banner()
	titulo("🛡️  CREAR USUARIO OPENVPN (config armado)")
	if !existeArchivo(rutaCAOpenVPN()) {
		falla("OpenVPN no está instalado en el server.")
		info("Falta " + rutaCAOpenVPN())
		pausar(2)
		return
	}
	usuario := leer("  👤 Nombre de usuario (ej: juan): ")
	if usuario == "" {
		falla("Nombre vacío")
		pausar(2)
		return
	}
	dias := leerInt("  ⏳ ¿Cuántos DÍAS dura? (ej 30 = 1 mes): ", 30)
	clave := claveRandom()
	vence := expDate(dias)

	if existeUsuarioSistema(usuario) || existeUsuarioSSH(usuario) {
		falla("El usuario " + usuario + " ya existe")
		pausar(2)
		return
	}
	crearUsuarioSistemaOpenVPN(usuario, clave, dias)
	if err := crearCuentaSSH(usuario, clave, vence, "solo", 10, 0, "GB"); err != nil {
		falla("no pude guardar en la base: " + err.Error())
		pausar(2)
		return
	}
	instalarWatchdogLimites()

	fmt.Println("")
	oks("¡USUARIO OPENVPN CREADO!")
	dato("👤 Usuario", usuario)
	dato("🔑 Password", clave)
	dato("⏳ Expira", vence)
	fmt.Println("")
	fmt.Printf("  %s[1]%s Ver/descargar el .ovpn armado\n", negrita, nc)
	fmt.Printf("  %s[0]%s Volver al menú\n", negrita, nc)
	if leerDef("  ➤ Opción: ", "0") == "1" {
		verOVPN(usuario, clave)
	}
}

func crearUsuarioSistemaOpenVPN(usuario, clave string, dias int) {
	if noTocarSistema() {
		fmt.Printf("  %s(prueba: no toco usuarios del sistema)%s\n", gris, nc)
		return
	}
	correr("useradd", "-m", "-s", "/bin/false", usuario)
	execChpasswd(usuario, clave).Run()
	correr("chage", "-E", expDia(dias), usuario)
	os.MkdirAll("/etc/ssh/sshd_config.d", 0755)
	os.WriteFile("/etc/ssh/sshd_config.d/"+usuario+".conf", []byte("MaxSessions 10\n"), 0644)
	correr("systemctl", "restart", "ssh")
}

func instalarWatchdogLimites() {
	sh := "/usr/local/bin/ghost-limit.sh"
	if !existeArchivo(sh) {
		if noTocarSistema() {
			fmt.Printf("  %s(prueba: no bajo ghost-limit.sh)%s\n", gris, nc)
			return
		}
		if descargar("https://raw.githubusercontent.com/TU-USUARIO/TU-REPO/main/ghost-limit.sh", sh) {
			os.Chmod(sh, 0755)
		} else {
			aviso("no pude bajar ghost-limit.sh")
			return
		}
	}
	// cron: * * * * * /usr/local/bin/ghost-limit.sh (sin duplicar)
	if noTocarSistema() {
		fmt.Printf("  %s(prueba: no toco el cron)%s\n", gris, nc)
		return
	}
	cron := correr("crontab", "-l")
	if !strings.Contains(cron, "ghost-limit.sh") {
		nuevo := cron
		if strings.TrimSpace(nuevo) != "" && !strings.HasSuffix(nuevo, "\n") {
			nuevo += "\n"
		}
		nuevo += "* * * * * /usr/local/bin/ghost-limit.sh\n"
		execCrontab(nuevo).Run()
		oks("watchdog de límites instalado (cron)")
	}
}

// ─── VER / ARMAR EL .OVPN ───────────────────────────────────────────────────
func verOVPN(usuario, clave string) {
	banner()
	titulo("📄 CONFIG OPENVPN ARMADO")
	caDatos, err := os.ReadFile(rutaCAOpenVPN())
	if err != nil {
		falla("no pude leer el CA: " + err.Error())
		esperar()
		return
	}
	dom := dominio()
	ip := ipPublica()
	plantilla := `client
dev tun
proto tcp-client
remote %s 80
resolv-retry infinite
nobind
persist-key
persist-tun
remote-cert-tls server
verb 3
auth-user-pass
route %s 255.255.255.255 net_gateway
route 104.18.22.183 255.255.255.255 net_gateway
route 104.18.23.183 255.255.255.255 net_gateway
tun-mtu 1200
mssfix 1200
cipher none
auth none
<ca>
%s</ca>
`
	contenido := fmt.Sprintf(plantilla, dom, ip, string(caDatos))
	os.MkdirAll(rutaOVPNDirReal(), 0755)
	ruta := filepath.Join(rutaOVPNDirReal(), usuario+".ovpn")
	if err := os.WriteFile(ruta, []byte(contenido), 0644); err != nil {
		falla("no pude escribir el .ovpn: " + err.Error())
		esperar()
		return
	}
	oks("config escrita en " + ruta)
	fmt.Println("")
	fmt.Printf("  %s[0]%s 📥 DESCARGAR el .ovpn (importar en HTTP Custom)\n", negrita, nc)
	info("     http://" + dom + "/ovpn/" + usuario + ".ovpn")
	fmt.Println("")
	fmt.Printf("  %s[1]%s PAYLOAD:\n", negrita, nc)
	fmt.Println("     ACL // HTTP/1.9[lf]Host: TU-BUGHOST-1[lf]Expect: 911-continue[crlf][crlf][split][crlf][crlf]GET- // HTTP/1.1[crlf]Host: " + dom + "[crlf]Connection: Upgrade[crlf] [crlf]Upgrade: websocket[crlf][crlf]")
	fmt.Println("")
	fmt.Printf("  %s[2]%s PROXY REMOTO:  %s:80\n", negrita, nc, dom)
	fmt.Printf("  %s[3]%s EDITOR DE CONFIGURACIÓN (.ovpn)  →  cat %s\n", negrita, nc, ruta)
	fmt.Printf("  %s[4]%s USUARIO / PASSWORD:  %s / %s\n", negrita, nc, usuario, clave)
	fmt.Println("")
	fmt.Printf("  📁 %s\n", ruta)
	fmt.Println("")
	op := leerDef("  ➤ Opción [1/2/3/4/0]: ", "0")
	switch op {
	case "1":
		verOVPN(usuario, clave) // vuelve a mostrar el resumen
	case "3":
		if datos, err := os.ReadFile(ruta); err == nil {
			fmt.Println(string(datos))
			esperar()
		}
	}
}

// ─── 5) CREAR USUARIO UDP CUSTOM ────────────────────────────────────────────
func crearUDPCustom() {
	banner()
	titulo("🛰️  CREAR USUARIO UDP CUSTOM")
	usuario := leer("  👤 Nombre de usuario: ")
	if usuario == "" {
		falla("Nombre vacío")
		pausar(2)
		return
	}
	operadora := leerDef("  🌐 Operadora (claro/personal): ", "claro")
	dias := leerInt("  ⏳ Duración en DÍAS (ej 30): ", 30)
	clave := claveRandom()
	vence := expDate(dias)

	if existeUsuarioSistema(usuario) {
		falla("El usuario " + usuario + " ya existe")
		pausar(2)
		return
	}
	if noTocarSistema() {
		fmt.Printf("  %s(prueba: no toco usuarios del sistema)%s\n", gris, nc)
	} else {
		correr("useradd", "-m", "-s", "/bin/false", usuario)
		execChpasswd(usuario, clave).Run()
		correr("chage", "-E", expDia(dias), usuario)
	}
	fmt.Println("")
	oks("¡USUARIO UDP CUSTOM CREADO! (valida contra /etc/shadow)")
	dato("👤 Usuario", usuario)
	dato("🔑 Password", clave)
	dato("⏳ Expira", vence)
	dato("🌐 Operadora", operadora)
	fmt.Println("")
	info("🛰️  Host: " + ipPublica() + " · Puertos 1-65535")
	info("📱 App:  UDP Custom en HTTP Custom (solo SIM " + operadora + ")")
	fmt.Println("")
	esperar()
}

// ─── 11) ESTADO WIREGUARD ───────────────────────────────────────────────────
func estadoWG() {
	banner()
	titulo("🔐 ESTADO WIREGUARD")
	if servicioActivo("wg-quick@wg0") {
		oks("wg0: ✅ activo")
	} else {
		falla("wg0: ❌ caído")
	}
	if existeComando("wg") {
		salida := correr("wg", "show")
		for _, l := range strings.Split(salida, "\n") {
			if strings.Contains(l, "interface") || strings.Contains(l, "public key") || strings.Contains(l, "listening port") {
				fmt.Println("  " + l)
			}
		}
		peers := correr("wg", "show", "peers")
		if strings.TrimSpace(peers) == "" {
			info("(sin peers)")
		} else {
			for _, p := range strings.Split(strings.TrimSpace(peers), "\n") {
				fmt.Println("  peer " + p)
			}
		}
	} else {
		aviso("el comando 'wg' no está instalado")
	}
}

// ─── 12) PANEL WEB ──────────────────────────────────────────────────────────
func panelWeb() {
	banner()
	titulo("🌐 PANEL WEB (puerto 8303)")
	switch {
	case servicioActivo("ghost-panel-web"):
		oks("ghost-panel-web: ✅ activo (puerto 8303)")
		info("🔗 Admin:   http://" + ipPublica() + ":8303")
		info("👤 WebView: http://" + ipPublica() + ":8303/webview?user=USUARIO")
	case servicioActivo("ghost-panel"):
		oks("ghost-panel: ✅ activo (puerto 8303)")
		aviso("panel viejo — sin WebView")
	default:
		falla("Panel web: ❌ no instalado")
		if leerSiNo("  ¿Instalarlo ahora?") {
			if noTocarSistema() {
				info("(prueba: no instalo nada)")
			} else {
				info("corriendo el instalador…")
				out := correr("bash", "-c", "bash <(curl -s https://raw.githubusercontent.com/TU-USUARIO/TU-REPO/main/webpanel-node/install-node-panel.sh)")
				fmt.Println(out)
			}
		}
	}
	fmt.Println("")
	esperar()
}

// ─── 10) PSIPHON ────────────────────────────────────────────────────────────
func crearPsiphon() {
	banner()
	titulo("🌐 PSIPHON (credencial única del servidor)")
	if !existeArchivo(rutaPsiphonEntry()) {
		falla("Server Psiphon NO instalado")
		info("falta " + rutaPsiphonEntry())
		pausar(2)
		return
	}
	tam := int64(0)
	if st, err := os.Stat(rutaPsiphonEntry()); err == nil {
		tam = st.Size()
	}
	oks("Server Psiphon instalado")
	dato("📁 Archivo", rutaPsiphonEntry())
	dato("📏 Tamaño", fmt.Sprintf("%d bytes", tam))
	if datos, err := os.ReadFile(rutaPsiphonConfig()); err == nil {
		re := regexp.MustCompile(`"SSHUserName"\s*:\s*"([^"]*)"`)
		if m := re.FindStringSubmatch(string(datos)); m != nil {
			dato("👤 SSHUserName", m[1])
		}
	}
	fmt.Println("")
	fmt.Printf("  %s[1]%s Ver el HEX (Server Entry)\n", negrita, nc)
	fmt.Printf("  %s[2]%s Regenerar (kill-switch: invalida TODOS los configs)\n", negrita, nc)
	fmt.Printf("  %s[0]%s Volver\n", negrita, nc)
	switch leerDef("  ➤ Opción: ", "0") {
	case "1":
		if datos, err := os.ReadFile(rutaPsiphonEntry()); err == nil {
			fmt.Println("")
			fmt.Println(strings.TrimSpace(string(datos)))
			info("HTTP Custom → Psiphon → Server Entry")
		}
		esperar()
	case "2":
		regenerarPsiphon()
	}
}

func regenerarPsiphon() {
	aviso("Esto invalida TODOS los configs de Psiphon (hay que repartirlos de nuevo).")
	if !(strings.EqualFold(leer("  ¿Continuar? (s/N): "), "s")) {
		info("cancelado")
		pausar(1)
		return
	}
	if noTocarSistema() {
		info("(prueba: no toco psiphon)")
		esperar()
		return
	}
	correr("systemctl", "stop", "psiphon")
	pausar(1)
	ip := ipPublica()
	os.Remove(rutaPsiphonConfig())
	os.Remove(rutaPsiphonEntry())
	correr("bash", "-c", "cd /etc/psiphon && /usr/local/bin/psiphon-server -ipaddress "+ip+" -protocol SSH:2223 generate >/dev/null 2>&1")
	if existeArchivo(rutaPsiphonEntry()) {
		os.MkdirAll(filepath.Dir(rutaPsiphonLicense()), 0755)
		if datos, err := os.ReadFile(rutaPsiphonEntry()); err == nil {
			os.WriteFile(rutaPsiphonLicense(), datos, 0600)
		}
		correr("systemctl", "start", "psiphon")
		oks("Psiphon regenerado y arriba")
		if datos, err := os.ReadFile(rutaPsiphonLicense()); err == nil {
			fmt.Println(strings.TrimSpace(string(datos)))
		}
	} else {
		falla("Falló la regeneración")
		correr("systemctl", "start", "psiphon")
	}
	esperar()
}

// ─── 14/16) SHADOWSOCKS ─────────────────────────────────────────────────────
func ssPassword() string {
	if datos, err := os.ReadFile(rutaSSPassword()); err == nil {
		if p := strings.TrimSpace(string(datos)); p != "" {
			return p
		}
	}
	// fallback: el inbound de shadowsocks del config de xray
	if datos, err := os.ReadFile(RutaXrayConfig); err == nil {
		var cfg struct {
			Inbounds []struct {
				Protocol string `json:"protocol"`
				Settings struct {
					Password string `json:"password"`
				} `json:"settings"`
			} `json:"inbounds"`
		}
		if json.Unmarshal(datos, &cfg) == nil {
			for _, in := range cfg.Inbounds {
				if in.Protocol == "shadowsocks" && in.Settings.Password != "" {
					return in.Settings.Password
				}
			}
		}
	}
	return ""
}

func ssPuerto() int {
	cfg := leerConfigJSON()
	if p := int(cfgFloat(cfg, "ss_port", 8388)); p > 0 {
		return p
	}
	return 8388
}

func linkShadowsocks() {
	banner()
	titulo("🕶️ LINK SHADOWSOCKS")
	pass := ssPassword()
	if pass == "" {
		falla("Shadowsocks no configurado (falta " + rutaSSPassword() + ")")
		pausar(2)
		return
	}
	ip := ipPublica()
	puerto := ssPuerto()
	b64 := base64.StdEncoding.EncodeToString([]byte("aes-256-gcm:" + pass))
	oks(fmt.Sprintf("Shadowsocks activo (proxy :80 → interno :%d)", puerto))
	fmt.Println("")
	dato("🔗 Link zero-rating", fmt.Sprintf("ss://%s@%s:80#Ghost-SS", b64, ip))
	dato("🔗 Link directo", fmt.Sprintf("ss://%s@%s:%d#Ghost-SS", b64, ip, puerto))
	fmt.Println("")
	info("📱 Config manual en HTTP Custom (perfil V2Ray):")
	info("   Servidor: Shadowsocks · Server: " + ip + " · Puerto: 80")
	info("   Password: " + pass + " · Método: aes-256-gcm")
	fmt.Println("")
	info("PAYLOAD:")
	fmt.Println("   COPY / HTTP/1.1[crlf]Host: TU-BUGHOST-3[crlf][lf][crlf][split]X / HTTP/1.2[crlf]Host: TU-BUGHOST-3[crlf][lf]")
	info("PROXY REMOTO: TU-BUGHOST-2")
	fmt.Println("")
	esperar()
}

func regenerarShadowsocks() {
	banner()
	titulo("🕶️ REGENERAR PASSWORD SHADOWSOCKS (kill-switch)")
	if !existeArchivo(rutaSSUsersSh()) {
		falla("falta " + rutaSSUsersSh())
		pausar(2)
		return
	}
	aviso("Esto invalida TODOS los links de Shadowsocks (password única).")
	if leer("  ¿Continuar? (s/N): ") != "s" && leer("") != "S" {
		info("cancelado")
		pausar(1)
		return
	}
	if noTocarSistema() {
		info("(prueba: no regenero)")
		esperar()
		return
	}
	out := correr(rutaSSUsersSh(), "new")
	fmt.Println(out)
	oks("password regenerada")
	esperar()
}

func verLinkShadowsocks() {
	banner()
	titulo("🕶️ LINK SHADOWSOCKS ACTUAL")
	if !existeArchivo(rutaSSUsersSh()) {
		falla("falta " + rutaSSUsersSh())
		pausar(2)
		return
	}
	fmt.Println(correr(rutaSSUsersSh(), "show"))
	fmt.Println("")
	esperar()
}

// ─── 17/18) SLOWDNS ─────────────────────────────────────────────────────────
func slowdnsNS() string {
	if datos, err := os.ReadFile(rutaSlowDNSService()); err == nil {
		re := regexp.MustCompile(`sdns\.[^ ]+`)
		if m := re.FindString(string(datos)); m != "" {
			return strings.Trim(m, `"`)
		}
	}
	return "sdns." + dominio()
}

func slowdnsPub() string {
	if datos, err := os.ReadFile(rutaSlowDNSPub()); err == nil {
		return strings.TrimSpace(string(datos))
	}
	return ""
}

func datosSlowDNS() {
	banner()
	titulo("🐢 DATOS SLOWDNS")
	if !existeArchivo(rutaSlowDNSServer()) {
		falla("SlowDNS no instalado (falta " + rutaSlowDNSServer() + ")")
		pausar(2)
		return
	}
	oks("SlowDNS activo (UDP :5300 → SSH, puente :5301)")
	dato("🌐 Nameserver", slowdnsNS())
	dato("🔌 Puerto", "80 (con payload + bughost)")
	dato("🔑 Pubkey", slowdnsPub())
	fmt.Println("")
	aviso(slowdnsNS() + " debe apuntar a " + ipPublica() + " (registro NS o A)")
	info("El nameserver se arma solo: sdns.<tu dominio>")
	fmt.Println("")
	esperar()
}

func crearSlowDNS() {
	banner()
	titulo("🐢 CREAR USUARIO SLOWDNS")
	if !existeArchivo(rutaSlowDNSServer()) {
		falla("SlowDNS no instalado")
		pausar(2)
		return
	}
	usuario := leer("  👤 Nombre (ej: juan): ")
	if usuario == "" {
		return
	}
	dias := leerInt("  ⏳ ¿Cuántos DÍAS dura? (ej 30): ", 30)
	clave := generarHex(6) // el bash usa -hex 6 (12 caracteres)
	vence := expDate(dias)

	if noTocarSistema() {
		fmt.Printf("  %s(prueba: no toco usuarios del sistema)%s\n", gris, nc)
	} else {
		correr("useradd", "-M", "-s", "/bin/false", usuario)
		execChpasswd(usuario, clave).Run()
		correr("chage", "-d", expDia(0), usuario)
		correr("chage", "-E", vence, usuario)
	}
	if err := crearCuentaSSH(usuario, clave, vence, "solo", 1, 0, "GB"); err != nil {
		falla("no pude guardar: " + err.Error())
	} else {
		oks("¡USUARIO SLOWDNS CREADO!")
	}
	fmt.Println("")
	dato("👤 Usuario", usuario)
	dato("🔑 Password", clave)
	dato("⏳ Expira", vence)
	fmt.Println("")
	info("📱 Config HTTP Custom:")
	info("   Nameserver: " + slowdnsNS())
	info("   Puerto: 80 (con payload) o " + ipPublica() + ":5301 directo")
	info("   Pubkey: " + slowdnsPub())
	info("   Usuario/Password: " + usuario + " / " + clave)
	fmt.Println("")
	aviso(slowdnsNS() + " debe resolver a " + ipPublica())
	fmt.Println("")
	esperar()
}

// ─── 13) BANNER SSH ─────────────────────────────────────────────────────────
const plantillaBannerEPRO = `========================================
      ✅  E P R O . H C  ✅
----------------------------------------
   🌐 HTTP Custom · SSH Tunnel
   🔐 Conexión segura establecida
========================================
`

const plantillaBannerGhost = `========================================
      🦇  GHOST VPN  🦇
----------------------------------------
   ⚡ Rápido · Seguro · Estable
   📡 Zero-Rating Ready
   🔐 Tu conexión está protegida
========================================
`

const plantillaBannerMarca = `========================================
      🚀  TU MARCA AQUÍ  🚀
----------------------------------------
   ⚡ Velocidad máxima garantizada
   🌐 Navega sin límites
   🔐 SSL + Tunnel activo
   💬 Soporte: @TU_TELEGRAM
========================================
`

const plantillaBannerVendedor = `========================================
   💰  PLAN PREMIUM  💰
----------------------------------------
   👤 Usuario: TU_USER
   ⏳ Vence: TU_FECHA
   📶 Consumo: TU_CONSUMO
----------------------------------------
   📱 Soporte: @TU_TELEGRAM
   🌐 TuMarca.com
========================================
`

const plantillaBannerANSI = "\033[1;31m========================================\033[0m\n" +
	"\033[1;33m      ✅  E P R O . H C  ✅\033[0m\n" +
	"\033[1;31m----------------------------------------\033[0m\n" +
	"\033[1;36m   🌐 HTTP Custom · SSH Tunnel\033[0m\n" +
	"\033[1;32m   🔐 Conexión segura establecida\033[0m\n" +
	"\033[1;31m========================================\033[0m\n"

func activarBannerSshd() {
	if noTocarSistema() {
		return
	}
	// si no hay banner, lo crea con la plantilla EPRO.HC
	if st, err := os.Stat(rutaBannerSSH()); err != nil || st.Size() == 0 {
		os.MkdirAll(filepath.Dir(rutaBannerSSH()), 0755)
		os.WriteFile(rutaBannerSSH(), []byte(plantillaBannerEPRO), 0644)
	}
	if noTocarSistema() {
		return
	}
	sshdCfg := "/etc/ssh/sshd_config"
	if datos, err := os.ReadFile(sshdCfg); err == nil {
		nuevo := strings.ReplaceAll(string(datos), "Banner none", "Banner /etc/ssh/banner")
		if nuevo != string(datos) {
			os.WriteFile(sshdCfg, []byte(nuevo), 0644)
		}
		if !strings.Contains(nuevo, "Banner /etc/ssh/banner") {
			if st, err := os.Stat("/etc/ssh/sshd_config.d"); err == nil && st.IsDir() {
				os.WriteFile("/etc/ssh/sshd_config.d/99-ghost-banner.conf", []byte("Banner /etc/ssh/banner\n"), 0644)
			} else {
				f, err := os.OpenFile(sshdCfg, os.O_APPEND|os.O_WRONLY, 0644)
				if err == nil {
					f.WriteString("Banner /etc/ssh/banner\n")
					f.Close()
				}
			}
		}
	}
	correr("systemctl", "restart", "ssh")
}

func editarBanner() {
	activarBannerSshd()
	for {
		banner()
		titulo("🎨 EDITAR BANNER SSH")
		dato("📁 Archivo", rutaBannerSSH())
		fmt.Println("")
		fmt.Printf("  %s[1]%s 📝 Editor libre (nano)\n", negrita, nc)
		fmt.Printf("  %s[2]%s 🎨 Plantillas (5 diseños)\n", negrita, nc)
		fmt.Printf("  %s[3]%s 👁️  Ver banner actual\n", negrita, nc)
		fmt.Printf("  %s[4]%s 🌈 Probar con colores ANSI\n", negrita, nc)
		fmt.Printf("  %s[5]%s 🔄 Restaurar banner original (EPRO.HC)\n", negrita, nc)
		fmt.Printf("  %s[0]%s Volver\n", negrita, nc)
		switch leerDef("  ➤ Opción: ", "0") {
		case "0":
			return
		case "1":
			if noTocarSistema() {
				info("(prueba: no abro nano)")
			} else {
				correr("nano", rutaBannerSSH())
				correr("systemctl", "restart", "ssh")
				oks("banner guardado")
			}
		case "2":
			fmt.Printf("  %s[1]%s EPRO.HC clásico\n  %s[2]%s Ghost VPN\n  %s[3]%s Marca (placeholders)\n  %s[4]%s Plan premium (placeholders)\n  %s[0]%s Volver\n",
				negrita, nc, negrita, nc, negrita, nc, negrita, nc, negrita, nc)
			switch leerDef("  ➤ Plantilla: ", "0") {
			case "1":
				escribirBanner(plantillaBannerEPRO)
			case "2":
				escribirBanner(plantillaBannerGhost)
			case "3":
				escribirBanner(plantillaBannerMarca)
			case "4":
				escribirBanner(plantillaBannerVendedor)
			}
		case "3":
			if datos, err := os.ReadFile(rutaBannerSSH()); err == nil {
				fmt.Println(string(datos))
			} else {
				falla("no pude leer el banner")
			}
			esperar()
		case "4":
			escribirBanner(plantillaBannerANSI)
		case "5":
			escribirBanner(plantillaBannerEPRO)
		}
	}
}

func escribirBanner(texto string) {
	os.MkdirAll(filepath.Dir(rutaBannerSSH()), 0755)
	if err := os.WriteFile(rutaBannerSSH(), []byte(texto), 0644); err != nil {
		falla("no pude escribir el banner: " + err.Error())
		esperar()
		return
	}
	if !noTocarSistema() {
		correr("systemctl", "restart", "ssh")
	}
	oks("banner actualizado en " + rutaBannerSSH())
	pausar(1)
}
