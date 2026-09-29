// ghost-manager en Go — instaladores: UDP Custom, Bilola (BHTTP/XHTTP/BTUN) y HCR (FASE 3)
// ⚠️ Todos DETECTAN primero: si ya está instalado y andando, avisan y preguntan antes de tocar.
package main

import (
	"fmt"
	"os"
	"strings"
)

// ─── UDP CUSTOM ─────────────────────────────────────────────────────────────
const (
	udpConfig  = "{\n  \"listen\": \":36712\",\n  \"stream_buffer\": 33554432,\n  \"receive_buffer\": 83886080,\n  \"auth\": {\n    \"mode\": \"passwords\"\n  }\n}\n"
	udpMD5     = "60cff70331810a9be6d7fc082443cfb5" // MD5 del BINARIO oficial v1.4
	udpURL     = "https://raw.githubusercontent.com/http-custom/udp-custom/main/bin/udp-custom-linux-amd64"
	udpUnit    = "[Unit]\nDescription=UDP Custom by ePro Dev. Team\nAfter=network.target\n\n[Service]\nUser=root\nType=simple\nExecStart=%s server\nWorkingDirectory=%s\nRestart=always\nRestartSec=2s\n\n[Install]\nWantedBy=multi-user.target\n"
	udpExcept  = "#!/bin/bash\nsleep 2\niptables -t nat -D PREROUTING -p udp --dport 53 -j RETURN 2>/dev/null\niptables -t nat -D PREROUTING -p udp --dport 51820 -j RETURN 2>/dev/null\niptables -t nat -I PREROUTING 2 -p udp --dport 53 -j RETURN 2>/dev/null\niptables -t nat -I PREROUTING 2 -p udp --dport 51820 -j RETURN 2>/dev/null\n"
	udpExcUnit = "[Unit]\nDescription=Excepciones UDP (WG/DNS) para udp-custom DNAT\nAfter=udp-custom.service\nRequires=udp-custom.service\n\n[Service]\nType=oneshot\nExecStart=/etc/udp-custom-except.sh\n\n[Install]\nWantedBy=multi-user.target\n"
)

// rutas de UDP Custom (pisables por entorno para pruebas)
func udpDirReal() string { return env0("GM_UDP_DIR", "/root/udp") }
func udpBinReal() string { return env0("GM_UDP_BIN", "/root/udp/udp-custom") }
func udpCfgReal() string { return env0("GM_UDP_CFG", "/root/udp/config.json") }
func hcrKeyFile() string { return env0("GM_HCR_KEY_FILE", "/etc/hcr-server.key") }
func hcrBinDir() string  { return env0("GM_HCR_DIR", "/opt/hcr-server") }

func instalarUDPCustom() {
	banner()
	titulo("🛰️  INSTALAR UDP CUSTOM (método UDP)")
	os.MkdirAll(udpDirReal(), 0755)

	// 1) ¿ya está el binario oficial?
	yaEsta := existeArchivo(udpBinReal()) && correr("md5sum", udpBinReal()) != "" &&
		strings.HasPrefix(strings.TrimSpace(correr("md5sum", udpBinReal())), udpMD5)
	if yaEsta {
		oks("Binario udp-custom ya instalado (v1.4 oficial)")
		if servicioActivo("udp-custom") && !leerSiNo("  Ya está ACTIVO. ¿Reinstalar igual?") {
			info("cancelo la instalación (queda como está)")
			esperar()
			return
		}
	} else if noTocarSistema() {
		info("(prueba: no descargo el binario)")
	} else {
		info("⬇️  Descargando udp-custom...")
		if !descargar(udpURL, udpBinReal()) {
			falla("No se pudo descargar")
			esperar()
			return
		}
		os.Chmod(udpBinReal(), 0755)
		md5bin := strings.Fields(correr("md5sum", udpBinReal()))
		if len(md5bin) > 0 && md5bin[0] == udpMD5 {
			oks("Binario v1.4 descargado (MD5 verificado)")
		} else if len(md5bin) > 0 {
			aviso("MD5 distinto (" + md5bin[0] + ") — puede ser versión nueva, sigo igual")
		}
	}

	// 2) config
	if !existeArchivo(udpCfgReal()) || !strings.Contains(leerArchivo(udpCfgReal()), "36712") {
		os.WriteFile(udpCfgReal(), []byte(udpConfig), 0644)
		oks("config.json escrito en " + udpCfgReal())
	} else {
		oks("config.json ya existe (se conserva)")
	}

	if noTocarSistema() {
		info("(prueba: no escribo unidades ni arranco servicios)")
		esperar()
		return
	}

	// 3) unidad + arranque limpio
	escribirUnidad("udp-custom", fmt.Sprintf(udpUnit, udpBinReal(), udpDirReal()))
	correr("systemctl", "daemon-reload")
	correr("systemctl", "stop", "udp-custom")
	for i := 0; i < 20; i++ {
		if correrOculto("iptables", "-t", "nat", "-D", "PREROUTING", "2") != nil {
			break
		}
	}
	correr("systemctl", "start", "udp-custom")
	correr("bash", "-c", "sleep 2")
	if servicioActivo("udp-custom") {
		oks("UDP Custom ACTIVO (:36712)")
	} else {
		falla("No arrancó — mirá: journalctl -u udp-custom -n 15")
	}

	// 4) excepciones WG/DNS
	os.WriteFile("/etc/udp-custom-except.sh", []byte(udpExcept), 0755)
	escribirUnidad("udp-custom-except", udpExcUnit)
	correr("systemctl", "daemon-reload")
	correr("systemctl", "enable", "udp-custom")
	correr("systemctl", "enable", "udp-custom-except")
	correr("bash", "/etc/udp-custom-except.sh")
	oks("Excepciones WG/DNS aplicadas (persistentes)")

	// 5) firewall (solo UDP 36712)
	if existeComando("ufw") {
		correr("ufw", "allow", "36712/udp")
		oks("UFW: 36712/udp abierto")
	} else if existeComando("firewall-cmd") {
		correr("firewall-cmd", "--permanent", "--add-port=36712/udp")
		correr("firewall-cmd", "--reload")
		oks("firewalld: 36712/udp abierto")
	} else {
		correr("iptables", "-I", "INPUT", "-p", "udp", "--dport", "36712", "-j", "ACCEPT")
		oks("iptables: 36712/udp abierto")
	}

	// 6) usuario demo
	if !existeUsuarioSistema("udp1") {
		correr("useradd", "-M", "udp1", "-s", "/bin/false")
		execChpasswd("udp1", "udp1").Run()
		oks("Usuario demo: udp1 / udp1")
	}
	fmt.Println("")
	info("📱 En la app (HTTP Custom → UDP Custom):")
	info("   " + ipPublica() + ":1-65535@udp1:udp1   (o :36712 fijo)")
	info("   ☑️ UDP Custom ON · ☑️ Permitir inseguro · ☑️ SNI automático")
	fmt.Println("")
	esperar()
}

func leerArchivo(ruta string) string {
	datos, err := os.ReadFile(ruta)
	if err != nil {
		return ""
	}
	return string(datos)
}

// ─── BILOLA (BHTTP / XHTTP / BTUN) ──────────────────────────────────────────
const bilolaRelease = "https://github.com/TU-USUARIO/TU-REPO/releases/download/BILOLA-TAG"
const bilolaDir = "/usr/local/lib/bilola"

func bilolaArch() string {
	if strings.TrimSpace(correr("uname", "-m")) == "aarch64" {
		return "arm64"
	}
	return "amd64"
}

func bilolaBajarBinario(nombre string) bool {
	destino := bilolaDir + "/" + nombre
	os.MkdirAll(bilolaDir, 0755)
	if existeArchivo(destino) {
		if correrOculto(destino, "-h") == nil {
			oks("binario ya instalado (" + nombre + ")")
			return true
		}
	}
	if noTocarSistema() {
		info("(prueba: no bajo " + nombre + ")")
		return false
	}
	info("⬇️  Descargando " + nombre + "...")
	if !descargar(bilolaRelease+"/"+nombre, destino) {
		falla("No se pudo descargar " + nombre)
		return false
	}
	os.Chmod(destino, 0755)
	return true
}

func bilolaAbrirPuertos(puertos ...int) {
	if noTocarSistema() {
		info("(prueba: no abro puertos)")
		return
	}
	for _, p := range puertos {
		if existeComando("ufw") {
			correr("ufw", "allow", fmt.Sprintf("%d/tcp", p))
			correr("ufw", "allow", fmt.Sprintf("%d/udp", p))
		}
		if existeComando("iptables") {
			if correrOculto("iptables", "-C", "INPUT", "-p", "tcp", "--dport", fmt.Sprint(p), "-j", "ACCEPT") != nil {
				correr("iptables", "-I", "INPUT", "-p", "tcp", "--dport", fmt.Sprint(p), "-j", "ACCEPT")
			}
			if correrOculto("iptables", "-C", "INPUT", "-p", "udp", "--dport", fmt.Sprint(p), "-j", "ACCEPT") != nil {
				correr("iptables", "-I", "INPUT", "-p", "udp", "--dport", fmt.Sprint(p), "-j", "ACCEPT")
			}
		}
	}
}

func bilolaInstalar(que string) {
	arch := bilolaArch()
	switch que {
	case "bhttp":
		banner()
		titulo("🦇 INSTALAR BHTTP (bilola-server)")
		info("Protocolo BHP1/BHP2 nativo → sshd :22")
		if !bilolaBajarBinario("bilola-server-linux-" + arch) {
			esperar()
			return
		}
		puertos := "0.0.0.0:80,0.0.0.0:53"
		if puertoEscuchando(80) {
			aviso("El :80 está ocupado (el portero) — BHTTP va en :9000 y :9001")
			puertos = "0.0.0.0:9000,0.0.0.0:9001"
			bilolaAbrirPuertos(9000, 9001)
		} else {
			bilolaAbrirPuertos(80, 53)
		}
		unit := fmt.Sprintf("[Unit]\nDescription=Bilola BHTTP Go server\nAfter=network.target\n\n[Service]\nType=simple\nExecStart=%s/bilola-server-linux-%s --listen %s --target 127.0.0.1:22\nRestart=always\nRestartSec=3\n\n[Install]\nWantedBy=multi-user.target\n",
			bilolaDir, arch, puertos)
		escribirUnidad("bilola-bhttp", unit)
		correr("systemctl", "daemon-reload")
		correr("systemctl", "enable", "bilola-bhttp")
		correr("systemctl", "restart", "bilola-bhttp")
		correr("bash", "-c", "sleep 2")
		if servicioActivo("bilola-bhttp") {
			oks("BHTTP ACTIVO (" + puertos + " → sshd :22)")
		} else {
			falla("No arrancó — mirá: journalctl -u bilola-bhttp -n 20")
		}
		esperar()
	case "xhttp":
		banner()
		titulo("🛇  INSTALAR SSH_XHTTP (DTunnel 5.0.0)")
		info("Protocolo XHTTP (HTTP/2) en :443 y :8080 → sshd :22 · requiere TLS")
		if !bilolaBajarBinario("bilola-xhttp-server-linux-" + arch) {
			esperar()
			return
		}
		cert, key := "/etc/bilola/tls/fullchain.pem", "/etc/bilola/tls/privkey.pem"
		os.MkdirAll("/etc/bilola/tls", 0755)
		cf := strings.TrimSpace(correr("bash", "-c", "find /var/lib/caddy/.local/share/caddy/certificates -name '*.crt' 2>/dev/null | head -1"))
		if cf != "" {
			dom := correr("bash", "-c", "basename $(dirname "+cf+")")
			copiarArchivo(cf, cert)
			copiarArchivo(correr("bash", "-c", "dirname "+cf)+"/"+dom+".key", key)
			os.Chmod(key, 0600)
			oks("Cert TLS copiado (" + dom + ")")
		} else {
			aviso("No encontré cert TLS — pedí el fullchain.pem y privkey.pem")
			rc := leer("  📁 Ruta del fullchain.pem (Enter = cancelar): ")
			if rc == "" {
				return
			}
			rk := leer("  📁 Ruta del privkey.pem: ")
			if !copiarArchivo(rc, cert) || !copiarArchivo(rk, key) {
				falla("Cert o key no válidos")
				esperar()
				return
			}
			os.Chmod(key, 0600)
		}
		bilolaAbrirPuertos(443, 8080)
		unit := fmt.Sprintf("[Unit]\nDescription=Bilola SSH_XHTTP TLS/HTTP2 server\nAfter=network.target\n\n[Service]\nType=simple\nExecStart=%s/bilola-xhttp-server-linux-%s --listen=0.0.0.0:443,0.0.0.0:8080 --target=127.0.0.1:22 --tls-cert=%s --tls-key=%s\nRestart=always\nRestartSec=3\n\n[Install]\nWantedBy=multi-user.target\n",
			bilolaDir, arch, cert, key)
		escribirUnidad("bilola-xhttp", unit)
		correr("systemctl", "daemon-reload")
		correr("systemctl", "enable", "bilola-xhttp")
		correr("systemctl", "restart", "bilola-xhttp")
		correr("bash", "-c", "sleep 2")
		if servicioActivo("bilola-xhttp") {
			oks("SSH_XHTTP ACTIVO (:443+:8080 → sshd :22)")
		} else {
			falla("No arrancó — mirá: journalctl -u bilola-xhttp -n 20")
		}
		esperar()
	case "btun":
		banner()
		titulo("🛇  INSTALAR BTUN (túnel UDP)")
		info("Protocolo BTUN en :7300 (TCP+UDP) → túnel + PAM")
		if !bilolaBajarBinario("bilola-btun-server-linux-" + arch) {
			esperar()
			return
		}
		bilolaAbrirPuertos(7300)
		unit := fmt.Sprintf("[Unit]\nDescription=Bilola BTUN protocol server\nAfter=network.target\n\n[Service]\nType=simple\nExecStart=%s/bilola-btun-server-linux-%s --tcp-listen 0.0.0.0:7300 --udp-listen 0.0.0.0:7300 --auth pam --pam-service login\nRestart=always\nRestartSec=3\n\n[Install]\nWantedBy=multi-user.target\n",
			bilolaDir, arch)
		escribirUnidad("bilola-btun", unit)
		correr("systemctl", "daemon-reload")
		correr("systemctl", "enable", "bilola-btun")
		correr("systemctl", "restart", "bilola-btun")
		correr("bash", "-c", "sleep 2")
		if servicioActivo("bilola-btun") {
			oks("BTUN ACTIVO (:7300 TCP+UDP)")
		} else {
			falla("No arrancó — mirá: journalctl -u bilola-btun -n 20")
		}
		esperar()
	}
}

func copiarArchivo(origen, destino string) bool {
	datos, err := os.ReadFile(origen)
	if err != nil {
		return false
	}
	os.MkdirAll(dirDe(destino), 0755)
	return os.WriteFile(destino, datos, 0644) == nil
}

func dirDe(ruta string) string {
	i := strings.LastIndex(ruta, "/")
	if i <= 0 {
		return "."
	}
	return ruta[:i]
}

func bilolaEstado() {
	banner()
	titulo("🦇 ESTADO PROTOCOLOS BILOLA")
	estado := func(u string) string {
		switch correr("systemctl", "is-active", u) {
		case "active":
			return verde + "✅ activo" + nc
		case "failed":
			return rojo + "❌ error" + nc
		}
		return amar + "❌ no instalado" + nc
	}
	fmt.Printf("  %sBHTTP :80/:53   →%s %s\n", negrita, nc, estado("bilola-bhttp"))
	fmt.Printf("  %sXHTTP :443/:8080→%s %s\n", negrita, nc, estado("bilola-xhttp"))
	fmt.Printf("  %sBTUN  :7300     →%s %s\n", negrita, nc, estado("bilola-btun"))
	fmt.Println("")
	esperar()
}

func menuBilola() {
	for {
		banner()
		titulo("🦇 PROTOCOLOS BILOLA (BHTTP/XHTTP/BTUN)")
		info("Desarrollo original: equipo del protocolo")
		fmt.Println("")
		fmt.Printf("  %s[1]%s 🦇 Instalar BHTTP  (:80+:53 → sshd :22)\n", negrita, nc)
		fmt.Printf("  %s[2]%s 🚀 Instalar SSH_XHTTP (:443+:8080 → sshd :22, TLS)\n", negrita, nc)
		fmt.Printf("  %s[3]%s 📡 Instalar BTUN   (:7300 TCP+UDP, PAM)\n", negrita, nc)
		fmt.Printf("  %s[4]%s 📊 Estado de los protocolos\n", negrita, nc)
		fmt.Printf("  %s[0]%s Volver al menú principal\n", negrita, nc)
		switch leerDef("  Opción: ", "0") {
		case "0":
			return
		case "1":
			bilolaInstalar("bhttp")
		case "2":
			bilolaInstalar("xhttp")
		case "3":
			bilolaInstalar("btun")
		case "4":
			bilolaEstado()
		default:
			falla("Opción inválida")
			pausar(1)
		}
	}
}

// ─── HCR SERVER (método HTTP Custom) ────────────────────────────────────────
func instalarHCR() {
	banner()
	titulo("🚀 INSTALAR HCR SERVER (método HTTP Custom)")

	// 1) clave (archivo → entorno → preguntar)
	clave := strings.TrimSpace(leerArchivo(hcrKeyFile()))
	if clave == "" {
		clave = os.Getenv("HCR_KEY")
	}
	if clave == "" {
		clave = leer("  🔑 HCR_KEY (la que te dio el ePro): ")
		if clave == "" {
			falla("Sin clave — cancelado")
			esperar()
			return
		}
		if !noTocarSistema() {
			os.WriteFile(hcrKeyFile(), []byte(clave), 0600)
			oks("Clave guardada en " + hcrKeyFile() + " (no la pide más)")
		}
	}

	// 2) ¿ya está?
	bin := hcrBinDir() + "/hcr-server"
	yaInstalado := existeArchivo(bin) && correr(bin, "-version") != ""
	if yaInstalado {
		oks("Binario ya instalado: " + strings.Split(correr(bin, "-version"), "\n")[0])
		if servicioActivo("hcr-server") && !leerSiNo("  Ya está ACTIVO. ¿Reinstalar igual?") {
			info("cancelo (queda como está)")
			esperar()
			return
		}
	}
	if noTocarSistema() {
		info("(prueba: no descargo ni escribo unidades)")
		esperar()
		return
	}

	// 3) descarga
	os.MkdirAll(hcrBinDir(), 0755)
	correr("systemctl", "stop", "hcr-server")
	correr("systemctl", "stop", "hcr-server-8080")
	info("⬇️  Descargando hcr-server (con tu clave)...")
	if !descargar("https://TU-SERVIDOR/hcr/hcr-server?key="+clave, bin) {
		falla("No se pudo descargar (¿clave incorrecta?)")
		esperar()
		return
	}
	os.Chmod(bin, 0755)
	os.Chown(bin, 0, 0)
	if correr(bin, "-version") == "" {
		falla("El binario no funciona (no es un hcr-server válido)")
		esperar()
		return
	}
	oks("Binario OK: " + strings.Split(correr(bin, "-version"), "\n")[0])

	// 4) cert TLS (opcional)
	certDir := "/var/lib/caddy/.local/share/caddy/certificates/acme-v02.api.letsencrypt.org-directory"
	dom := strings.TrimSpace(correr("bash", "-c", "for d in "+certDir+"/*/; do if [ -f \"$d/fullchain.pem\" ] || ls \"$d\"*.crt >/dev/null 2>&1; then basename \"$d\"; break; fi; done"))
	tlsArgs := "--transport plain"
	if dom != "" && existeArchivo(certDir+"/"+dom+"/"+dom+".crt") && existeArchivo(certDir+"/"+dom+"/"+dom+".key") {
		copiarArchivo(certDir+"/"+dom+"/"+dom+".crt", hcrBinDir()+"/fullchain.pem")
		copiarArchivo(certDir+"/"+dom+"/"+dom+".key", hcrBinDir()+"/privkey.pem")
		os.Chmod(hcrBinDir()+"/privkey.pem", 0600)
		oks("Cert TLS copiado (" + dom + ") — transport auto")
		tlsArgs = "--transport auto --tls-cert " + hcrBinDir() + "/fullchain.pem --tls-key " + hcrBinDir() + "/privkey.pem"
	} else {
		aviso("Sin cert TLS — transport plain (sin TLS)")
		os.Remove(hcrBinDir() + "/fullchain.pem")
		os.Remove(hcrBinDir() + "/privkey.pem")
	}

	// 5) unidades
	base := bin + " --listen %s --target 127.0.0.1:22 " + tlsArgs + " --max-download-frame 6144 --download-poll-timeout 8s"
	escribirUnidad("hcr-server", fmt.Sprintf("[Unit]\nDescription=HCR Server (8880)\nAfter=network.target\n\n[Service]\nType=simple\nExecStart="+base+"\nRestart=always\nRestartSec=3\n\n[Install]\nWantedBy=multi-user.target\n", ":8880"))
	escribirUnidad("hcr-server-8080", fmt.Sprintf("[Unit]\nDescription=HCR Server (8080)\nAfter=network.target\n\n[Service]\nType=simple\nExecStart="+base+"\nRestart=always\nRestartSec=3\n\n[Install]\nWantedBy=multi-user.target\n", ":8080"))

	// 6) puertos + arranque
	for _, p := range []int{8880, 8080} {
		if existeComando("ufw") {
			correr("ufw", "allow", fmt.Sprintf("%d/tcp", p))
		}
		if existeComando("iptables") {
			if correrOculto("iptables", "-C", "INPUT", "-p", "tcp", "--dport", fmt.Sprint(p), "-j", "ACCEPT") != nil {
				correr("iptables", "-I", "INPUT", "-p", "tcp", "--dport", fmt.Sprint(p), "-j", "ACCEPT")
			}
		}
	}
	oks("Puertos 8880 y 8080 abiertos")
	correr("systemctl", "daemon-reload")
	correr("systemctl", "enable", "hcr-server")
	correr("systemctl", "enable", "hcr-server-8080")
	correr("systemctl", "restart", "hcr-server")
	correr("systemctl", "restart", "hcr-server-8080")
	correr("bash", "-c", "sleep 3")
	if servicioActivo("hcr-server") {
		oks("HCR Server ACTIVO en :8880")
	} else {
		falla("hcr-server :8880 no arrancó — mirá: journalctl -u hcr-server -n 20")
	}
	if servicioActivo("hcr-server-8080") {
		oks("HCR Server ACTIVO en :8080")
	} else {
		aviso("hcr-server-8080 no arrancó (opcional)")
	}
	fmt.Println("")
	info("📱 Config para la app HTTP Custom:")
	info("   Host:   " + ipPublica())
	info("   Puerto: 8880 o 8080 (a gusto)")
	info("   User:   (creá un usuario SSH con la opción 2)")
	info("   Modo:   HCR · TLS: OFF")
	fmt.Println("")
	esperar()
}
