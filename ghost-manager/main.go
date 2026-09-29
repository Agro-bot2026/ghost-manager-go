// ghost-manager v29.0-go "ESPELHO" — el ghost-manager del bash, portado a Go
// Uso: ghost-manager   (menú interactivo) · ghost-manager --version / --firma / estado / listar
package main

import (
	"fmt"
	"os"
	"strings"
)

const (
	Version       = "v29.0-go"
	VersionNombre = "ESPELHO"
	Autor         = "CHARLY_TRICKS"
)

var actualizacionDisponible = false

func firma() string {
	return "ghost-manager " + Version + " \"" + VersionNombre + "\" by " + Autor
}

// anchoVisible: cuenta los emojis como 2 columnas (para que el recuadro quede derecho)
func anchoVisible(s string) int {
	n := 0
	for _, r := range s {
		if r >= 0x1F300 || (r >= 0x2600 && r <= 0x27BF) {
			n += 2
		} else {
			n++
		}
	}
	return n
}

func centrar(texto string, ancho int) string {
	relleno := ancho - anchoVisible(texto)
	if relleno < 0 {
		relleno = 0
	}
	return texto + strings.Repeat(" ", relleno)
}

func mostrarFirma() {
	ancho := 50
	linea := func(t string) {
		fmt.Printf("  ║  %s║\n", centrar(t, ancho-4))
	}
	fmt.Println("")
	fmt.Printf("  ╔%s╗\n", strings.Repeat("═", ancho-2))
	linea("🦇 GHOST-MANAGER " + Version)
	linea("Todo en Go · " + VersionNombre)
	linea("by " + Autor)
	fmt.Printf("  ╚%s╝\n", strings.Repeat("═", ancho-2))
	fmt.Println("")
}

func menu() {
	activarBannerSshd()     // deja el banner del sshd listo (como el bash al entrar)
	chequearActualizacion() // avisa en el cartel si hay versión nueva
	for {
		banner()
		dashboard()
		fmt.Printf("%s%s  📋 MENÚ PRINCIPAL%s\n", negrita, amar, nc)
		fmt.Printf("%s  ───────────────%s\n", cian, nc)
		fmt.Printf("  %s%s  👤 USUARIOS%s\n", negrita, cian, nc)
		fmt.Printf("  %s%s1%s) 📊 Estado del sistema\n", negrita, fucsia, nc)
		fmt.Printf("  %s%s2%s) 👤 Crear usuario SSH\n", negrita, fucsia, nc)
		fmt.Printf("  %s%s3%s) 🛡️  Crear usuario OpenVPN (config armado)\n", negrita, fucsia, nc)
		fmt.Printf("  %s%s4%s) 🚀 Crear usuario V2Ray\n", negrita, fucsia, nc)
		fmt.Printf("  %s%s5%s) 🛰️  Crear usuario UDP Custom\n", negrita, fucsia, nc)
		fmt.Printf("  %s%s6%s) 👥 Listar usuarios (ver todos)\n", negrita, fucsia, nc)
		fmt.Printf("  %s%s7%s) 🔄 Renovar usuario (agregar más días)\n", negrita, fucsia, nc)
		fmt.Printf("  %s%s8%s) ✏️  Editar usuario (plan, clave, límite)\n", negrita, fucsia, nc)
		fmt.Printf("  %s%s9%s) 🗑️  Eliminar usuario (cortar acceso)\n", negrita, fucsia, nc)
		fmt.Printf("  %s%s  🌐 SERVICIOS%s\n", negrita, cian, nc)
		fmt.Printf("  %s%s10%s) 🌐 Estado Psiphon (server entry)\n", negrita, fucsia, nc)
		fmt.Printf("  %s%s11%s) 🔐 Estado WireGuard\n", negrita, fucsia, nc)
		fmt.Printf("  %s%s12%s) 🌐 Panel web (http://IP:8303)\n", negrita, fucsia, nc)
		fmt.Printf("  %s%s13%s) 🎨 Editar banner SSH\n", negrita, fucsia, nc)
		fmt.Printf("  %s%s14%s) 🕶️ Link Shadowsocks (ss://)\n", negrita, fucsia, nc)
		fmt.Printf("  %s%s15%s) 🕶️ Regenerar password Shadowsocks (kill-switch)\n", negrita, fucsia, nc)
		fmt.Printf("  %s%s16%s) 🕶️ Ver link Shadowsocks actual\n", negrita, fucsia, nc)
		fmt.Printf("  %s%s17%s) 🐢 Datos SlowDNS (nameserver + pubkey)\n", negrita, fucsia, nc)
		fmt.Printf("  %s%s18%s) 🐢 Crear usuario SlowDNS\n", negrita, fucsia, nc)
		fmt.Printf("  %s%s  📊 CONSUMOS%s\n", negrita, cian, nc)
		fmt.Printf("  %s%s19%s) 📡 Ver conexiones en vivo\n", negrita, fucsia, nc)
		fmt.Printf("  %s%s20%s) 📊 Ver consumo de datos (cuota)\n", negrita, fucsia, nc)
		fmt.Printf("  %s%s21%s) 📊 Consumo por CUENTA (SSH + V2Ray)\n", negrita, fucsia, nc)
		fmt.Printf("  %s%s22%s) 📊 Consumo Psiphon por IP\n", negrita, fucsia, nc)
		fmt.Printf("  %s%s31%s) 📏 Poner/quitar el límite de GB de una cuenta\n", negrita, fucsia, nc)
		fmt.Printf("  %s%s32%s) 🧹 Resetear el contador de GB (al renovar)\n", negrita, fucsia, nc)
		fmt.Printf("  %s%s  🛠️  HERRAMIENTAS%s\n", negrita, cian, nc)
		fmt.Printf("  %s%s23%s) 🤖 Ayuda IA (gratis, DeepSeek)\n", negrita, fucsia, nc)
		fmt.Printf("  %s%s24%s) 🗑️  Eliminar usuario VLESS\n", negrita, fucsia, nc)
		fmt.Printf("  %s%s  🤖 TELEGRAM%s\n", negrita, cian, nc)
		fmt.Printf("  %s%s25%s) 🤖 Bot de administración Telegram\n", negrita, fucsia, nc)
		fmt.Printf("  %s%s26%s) 🔑 Configurar API Key ePro\n", negrita, fucsia, nc)
		fmt.Printf("  %s%s  🚀 MÉTODO NUEVO (HCR/HTTP Custom)%s\n", negrita, cian, nc)
		fmt.Printf("  %s%s27%s) 🚀 Instalar HCR Server (método HTTP Custom)\n", negrita, fucsia, nc)
		fmt.Printf("  %s%s28%s) 🦇 Protocolos Bilola (BHTTP/XHTTP/BTUN)\n", negrita, fucsia, nc)
		fmt.Printf("  %s%s29%s) 🔄 Actualizar ghost-manager%s\n", negrita, fucsia, nc, mapaActualizacion())
		fmt.Printf("  %s%s30%s) 🛰️  Instalar UDP Custom (Movistar / método UDP)\n", negrita, fucsia, nc)
		fmt.Printf("  %s%s0%s) Salir\n", negrita, fucsia, nc)
		fmt.Println("")

		op := leer("  ➤ Opción: ")
		if entradaCerrada {
			fmt.Println("")
			return
		}
		switch strings.TrimSpace(op) {
		case "1":
			estado()
			esperar()
		case "2":
			crearSSH()
		case "3":
			crearOpenVPN()
		case "4":
			crearV2Ray()
		case "5":
			crearUDPCustom()
		case "6":
			listar()
		case "7":
			renovar()
		case "8":
			editar()
		case "9":
			eliminar()
		case "10":
			crearPsiphon()
		case "11":
			estadoWG()
			esperar()
		case "12":
			panelWeb()
		case "13":
			editarBanner()
		case "14":
			linkShadowsocks()
		case "15":
			regenerarShadowsocks()
		case "16":
			verLinkShadowsocks()
		case "17":
			datosSlowDNS()
		case "18":
			crearSlowDNS()
		case "19":
			verConexiones()
		case "20":
			verConsumo()
		case "21":
			verConsumoCuentas()
		case "22":
			verConsumoPsiphon()
		case "23":
			ayudaIA()
		case "24":
			eliminarVLESS()
		case "25":
			configurarBotTelegram()
		case "26":
			configurarAPIEpro()
		case "27":
			instalarHCR()
		case "28":
			menuBilola()
		case "29":
			actualizarGhostManager()
		case "30":
			instalarUDPCustom()
		case "31":
			cmdLimiteGB()
		case "32":
			cmdResetGB()
		case "0", "q", "salir":
			fmt.Printf("  %s👋 Hasta luego!%s\n", verde, nc)
			return
		default:
			aviso("opción no válida")
			pausar(1)
		}
	}
}

func mapaActualizacion() string {
	if actualizacionDisponible {
		return " " + verde + "(hay versión nueva!)" + nc
	}
	return ""
}

func ayuda() {
	fmt.Printf("\n%s\n\n", firma())
	fmt.Println("USO")
	fmt.Println("  ghost-manager                 menú interactivo (lo de siempre)")
	fmt.Println("  ghost-manager estado          estado del sistema")
	fmt.Println("  ghost-manager listar          usuarios SSH y V2Ray")
	fmt.Println("  ghost-manager consumo         consumo por IP y por cuenta")
	fmt.Println("  ghost-manager --version       la firma")
	fmt.Println("  ghost-manager --firma         el cartel")
	fmt.Println("")
	fmt.Println("PRUEBAS (no toca nada):  GM_NO_SYSTEM=1 GM_NO_CLEAR=1 GM_SSH_DB=/tmp/x.db …")
	fmt.Println()
}

func main() {
	args := os.Args[1:]
	if len(args) > 0 {
		switch args[0] {
		case "--version", "-v":
			fmt.Println(firma())
			return
		case "--firma":
			mostrarFirma()
			return
		case "--ayuda", "-h", "--help", "ayuda":
			ayuda()
			return
		case "estado":
			estado()
			return
		case "listar", "usuarios":
			listar()
			return
		case "consumo":
			verConsumoCuentas()
			return
		case "dashboard":
			dashboard()
			return
		}
	}
	menu()
}
