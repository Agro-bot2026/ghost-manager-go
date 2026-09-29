// ghost-manager en Go — extras: ayuda IA, bot de Telegram, API key ePro y actualización
package main

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

// ─── 23) AYUDA IA (el binario ghost-ai-help) ────────────────────────────────
func ayudaIA() {
	ia := "/usr/local/bin/ghost-ai-help"
	if !existeArchivo(ia) {
		falla("no está instalado " + ia)
		info("(es el ayudante de IA que viene con el script original)")
		esperar()
		return
	}
	limpiar()
	info("Abriendo la ayuda IA (Ctrl-C para volver)...")
	fmt.Println("")
	c := exec.Command(ia)
	c.Stdin, c.Stdout, c.Stderr = os.Stdin, os.Stdout, os.Stderr
	c.Run()
	fmt.Println("")
	esperar()
}

// ─── 25) BOT DE ADMINISTRACIÓN TELEGRAM ─────────────────────────────────────
func configurarBotTelegram() {
	banner()
	titulo("🤖 BOT DE ADMINISTRACIÓN TELEGRAM")
	info("Administrá este VPS desde Telegram:")
	info("crear usuarios, ver estado, links, banner... todo")
	fmt.Println("")
	tokenPath := rutaConfigDirReal() + "/tg_admin_token"
	idPath := rutaConfigDirReal() + "/tg_admin_id"
	if datos, err := os.ReadFile(tokenPath); err == nil {
		tok := strings.TrimSpace(string(datos))
		cola := tok
		if len(tok) > 8 {
			cola = tok[len(tok)-8:]
		}
		oks("Bot ya configurado: ..." + cola)
		if d2, err := os.ReadFile(idPath); err == nil {
			oks("👑 Admin ID: " + strings.TrimSpace(string(d2)))
		}
		if !strings.EqualFold(leer("  ¿Reconfigurar? (s/N): "), "s") {
			esperar()
			return
		}
	}
	fmt.Println("")
	fmt.Printf("  %s1️⃣ Creá un bot en Telegram: @BotFather → /newbot%s\n", negrita, nc)
	fmt.Printf("  %s2️⃣ Copiá el TOKEN que te da (123456:ABC-DEF...)%s\n", negrita, nc)
	fmt.Println("")
	TOKEN := leer("  🔑 Pegá el token del bot: ")
	if TOKEN == "" {
		falla("Token vacío")
		pausar(2)
		return
	}
	fmt.Println("")
	fmt.Printf("  %s3️⃣ Tu USER ID de Telegram (para que SOLO vos uses el bot)%s\n", negrita, nc)
	info("   Sacalo con @userinfobot o @getidsbot (número de 9-10 dígitos)")
	uid := leer("  👑 Tu USER ID: ")
	if uid == "" || strings.Trim(uid, "0123456789") != "" {
		falla("User ID inválido (debe ser numérico). El bot NO se va a configurar.")
		pausar(2)
		return
	}
	if noTocarSistema() {
		info("(prueba: no guardo token ni instalo el bot)")
		esperar()
		return
	}
	os.MkdirAll(rutaConfigDirReal(), 0755)
	os.WriteFile(tokenPath, []byte(TOKEN), 0600)
	os.WriteFile(idPath, []byte(uid), 0600)

	// archivos del bot (si están al lado del binario, se copian)
	botDir := "/opt/ghost-bot-admin"
	os.MkdirAll(botDir, 0755)
	yo, _ := os.Executable()
	for _, f := range []string{"bot-admin.js", "ghost-api.sh"} {
		origen := dirDe(yo) + "/" + f
		if !existeArchivo(origen) {
			origen = "/root/ghost-go/ghost-manager-go-src/" + f
		}
		if existeArchivo(origen) {
			destino := botDir + "/" + f
			if f == "ghost-api.sh" {
				destino = "/usr/local/bin/ghost-api.sh"
				copiarArchivo(origen, destino)
				os.Chmod(destino, 0755)
			} else {
				copiarArchivo(origen, destino)
			}
		}
	}

	// node/npm + telegraf si faltan
	if !existeComando("node") || !existeComando("npm") {
		info("📦 Instalando nodejs/npm (necesario para el bot)...")
		switch {
		case existeComando("apt-get"):
			correr("apt-get", "install", "-y", "-qq", "nodejs", "npm")
		case existeComando("dnf"):
			correr("dnf", "install", "-y", "nodejs", "npm")
		case existeComando("apk"):
			correr("apk", "add", "nodejs", "npm")
		}
	}
	if !existeArchivo(botDir + "/node_modules/telegraf") {
		info("📦 Instalando telegraf...")
		sh("cd " + botDir + " && npm init -y >/dev/null 2>&1 && npm install telegraf@4 >/dev/null 2>&1")
	}

	unit := "[Unit]\nDescription=Ghost Admin Bot Telegram\nAfter=network.target\n\n[Service]\nWorkingDirectory=" + botDir +
		"\nExecStart=/usr/bin/node " + botDir + "/bot-admin.js\nRestart=always\nRestartSec=10\n\n[Install]\nWantedBy=multi-user.target\n"
	escribirUnidad("ghost-bot-admin", unit)
	correr("systemctl", "daemon-reload")
	correr("systemctl", "enable", "ghost-bot-admin")
	correr("systemctl", "restart", "ghost-bot-admin")
	correr("bash", "-c", "sleep 2")
	if servicioActivo("ghost-bot-admin") {
		oks("Bot de Telegram ACTIVO 🤖")
		info("Mandale /start a tu bot para probarlo")
	} else {
		falla("No arrancó — mirá: journalctl -u ghost-bot-admin -n 20")
	}
	fmt.Println("")
	esperar()
}

func rutaConfigDirReal() string { return env0("GM_CONFIG_DIR", RutaConfigDir) }

// ─── 26) API KEY ePRO ───────────────────────────────────────────────────────
func configurarAPIEpro() {
	banner()
	titulo("🔑 API KEY ePRO")
	info("Configurá tu API key de ePro para generar configs .hc/.oc desde el bot.")
	fmt.Println("")
	keyPath := "/etc/ghost-license/epro-api.key"
	if datos, err := os.ReadFile(keyPath); err == nil {
		k := strings.TrimSpace(string(datos))
		cola := k
		if len(k) > 8 {
			cola = k[len(k)-8:]
		}
		oks("Key actual: ..." + cola)
		if !strings.EqualFold(leer("  ¿Reemplazarla? (s/N): "), "s") {
			esperar()
			return
		}
	}
	fmt.Println("")
	nueva := leer("  🔑 Pegá tu API key de ePro: ")
	if nueva == "" {
		falla("Key vacía")
		pausar(2)
		return
	}
	if noTocarSistema() {
		info("(prueba: no guardo la key)")
		esperar()
		return
	}
	os.MkdirAll("/etc/ghost-license", 0755)
	os.WriteFile(keyPath, []byte(nueva), 0600)

	// dominio (o IP)
	domPath := rutaConfigDirReal() + "/domain.txt"
	if dom := strings.TrimSpace(leerArchivo(domPath)); dom != "" {
		oks("🌐 Dominio detectado: " + dom)
		oks("✅ Los enlaces van a usar: https://" + dom + "/configs/...")
	} else {
		aviso("No hay dominio configurado (no instalaste SSL/SlowDNS).")
		if strings.EqualFold(leer("  ¿Querés configurar un dominio ahora? (s/N): "), "s") {
			dom := leer("  🌐 Tu dominio (ej: mivps.com): ")
			if dom != "" {
				os.MkdirAll(rutaConfigDirReal(), 0755)
				os.WriteFile(domPath, []byte(dom), 0644)
				oks("Dominio guardado: " + dom)
			}
		} else {
			aviso("📦 Usaré IP directa: http://" + ipPublica() + ":8080/configs/...")
			info("(el archivo también se te envía por Telegram igual)")
		}
	}

	// verificar la key con la cuota (usa el mismo script del ecosistema)
	fmt.Println("")
	info("🔍 Verificando key con la API...")
	out := correr("bash", "/usr/local/bin/ghost-epro.sh", "cuota")
	if out == "" {
		aviso("no pude verificarla (¿falta /usr/local/bin/ghost-epro.sh?)")
	} else {
		primeras := strings.Split(out, "\n")
		if len(primeras) > 5 {
			primeras = primeras[:5]
		}
		res := strings.ToLower(strings.Join(primeras, " "))
		if strings.Contains(res, "error") || strings.Contains(res, "no se pudo") || strings.Contains(res, "inválida") || strings.Contains(res, "401") {
			aviso("No pude verificar la cuota (la API puede estar migrando o la key es inválida).")
			aviso("La key quedó guardada igual — probá generando un config.")
		} else {
			oks("Key verificada:")
			fmt.Println(strings.Join(primeras, "\n"))
		}
	}
	fmt.Println("")
	oks("API key guardada! Generá configs desde el bot de Telegram:")
	info("📦 Crear Config (ePro) → elegí protocolo → formato")
	fmt.Println("")
	esperar()
}

// rutaBashOriginal: el script bash del menú (después del pase queda como ghost-manager.bash;
// si todavía no se hizo el pase, el comando ghost-manager ES el bash)
func rutaBashOriginal() string {
	if v := os.Getenv("GM_BASH_PATH"); v != "" {
		return v
	}
	if existeArchivo("/usr/local/bin/ghost-manager.bash") {
		return "/usr/local/bin/ghost-manager.bash"
	}
	return "/usr/local/bin/ghost-manager"
}

// ─── 29) ACTUALIZAR GHOST-MANAGER ───────────────────────────────────────────
func actualizarGhostManager() {
	banner()
	titulo("🔄 ACTUALIZAR GHOST-MANAGER")
	info("Versión local: " + firma())
	info("El original (bash) se actualiza desde GitHub: TU-USUARIO/TU-REPO")
	fmt.Println("")

	// ¿hay algo nuevo en el repo del bash?
	sha := ""
	if txt := descargarTexto("https://api.github.com/repos/TU-USUARIO/TU-REPO/commits/main"); txt != "" {
		if i := strings.Index(txt, `"sha": "`); i >= 0 {
			resto := txt[i+len(`"sha": "`):]
			if j := strings.Index(resto, `"`); j > 0 {
				sha = resto[:j]
			}
		}
	}
	if sha == "" {
		aviso("no pude consultar GitHub (¿sin internet?)")
	} else {
		remoto := descargarTexto("https://raw.githubusercontent.com/TU-USUARIO/TU-REPO/" + sha + "/ghost-manager")
		if remoto == "" {
			aviso("no pude bajar el ghost-manager del repo")
		} else {
			local := leerArchivo(rutaBashOriginal())
			if len(local) == len(remoto) {
				oks("el bash del repo está igual que el local (nada nuevo)")
			} else {
				aviso("hay una versión NUEVA del bash en el repo (distinta a la local)")
				info("OJO: esa versión es la que mira al pymanager; si la instalás, perdés")
				info("     las mejoras del port a Go (ghostprox, cuentas, cuotas).")
				if leerSiNo("  ¿Bajar igual el bash nuevo (hace backup antes)?") && !noTocarSistema() {
					destino := rutaBashOriginal()
					copiarArchivo(destino, destino+".bak-"+time.Now().Format("20060102-150405"))
					if os.WriteFile(destino, []byte(remoto), 0755) == nil {
						oks("bash actualizado (backup guardado)")
					} else {
						falla("no pude escribir el bash")
					}
				}
			}
		}
	}
	fmt.Println("")
	info("📌 El ghost-manager en Go (este programa) se actualiza copiando el binario nuevo.")
	info("   El bash viejo queda en /usr/local/bin/ghost-manager.bash por las dudas.")
	fmt.Println("")
	esperar()
}

// ─── estado de actualización (para el cartel del banner) ────────────────────
func chequearActualizacion() {
	if os.Getenv("GM_SIN_UPDATE") == "1" {
		return
	}
	// comparación liviana: solo el largo del archivo del repo (evita bajar todo al abrir)
	txt := descargarTexto("https://raw.githubusercontent.com/TU-USUARIO/TU-REPO/main/ghost-manager")
	if txt == "" {
		return
	}
	local := leerArchivo(rutaBashOriginal())
	if len(local) > 0 && len(txt) != len(local) {
		actualizacionDisponible = true
	}
}
