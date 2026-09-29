// ghost-manager en Go — interfaz (colores, banner, entradas, tablas, barras)
package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
)

// ─── COLORES (los mismos del bash) ───────────────────────────────────────────
// Son variables para poder apagarlos en pruebas/pipes con GM_SIN_COLOR=1
var (
	rojo    = "\033[0;31m"
	verde   = "\033[0;32m"
	amar    = "\033[1;33m"
	azul    = "\033[0;34m"
	magenta = "\033[0;35m"
	fucsia  = "\033[1;35m"
	naranja = "\033[38;5;208m"
	cian    = "\033[0;36m"
	gris    = "\033[0;90m"
	negrita = "\033[1m"
	nc      = "\033[0m"
)

func init() {
	if os.Getenv("GM_SIN_COLOR") == "1" {
		rojo, verde, amar, azul, magenta, fucsia = "", "", "", "", "", ""
		naranja, cian, gris, negrita, nc = "", "", "", "", ""
	}
}

var entrada = bufio.NewReader(os.Stdin)

// noTocarSistema: para PRUEBAS (GM_NO_SYSTEM=1) no crea/borra usuarios del sistema
func noTocarSistema() bool { return os.Getenv("GM_NO_SYSTEM") == "1" }

func limpiar() {
	if os.Getenv("GM_NO_CLEAR") == "1" {
		return
	}
	fmt.Print("\033[2J\033[H")
}

// ─── BANNER (el mismo dibujo del bash) ──────────────────────────────────────
func banner() {
	limpiar()
	fmt.Println(naranja)
	fmt.Println("███████╗██████╗ ██████╗  ██████╗    ██╗  ██╗ ██████╗")
	fmt.Println("██╔════╝██╔══██╗██╔══██╗██╔═══██╗   ██║  ██║██╔════╝")
	fmt.Println("█████╗  ██████╔╝██████╔╝██║   ██║   ███████║██║     ")
	fmt.Println("██╔══╝  ██╔═══╝ ██╔══██╗██║   ██║   ██╔══██║██║     ")
	fmt.Println("███████╗██║     ██║  ██║╚██████╔╝██╗██║  ██║╚██████╗")
	fmt.Println("╚══════╝╚═╝     ╚═╝  ╚═╝ ╚═════╝ ╚═╝╚═╝  ╚═╝ ╚═════╝")
	fmt.Print(nc)
	fmt.Printf("%s%s  🦇 Ghost VPN - Administrador de Usuarios%s\n", negrita, azul, nc)
	fmt.Printf("%s  ──────────────────────────────────────────%s\n", cian, nc)
	if actualizacionDisponible {
		fmt.Printf("  %s⚠️  Versión %s — ¡hay versión NUEVA disponible! (opción 29)%s\n", amar, Version, nc)
	} else {
		fmt.Printf("  %s✅ Versión %s (%s)%s\n", verde, Version, VersionNombre, nc)
	}
	fmt.Printf("%s  ──────────────────────────────────────────%s\n", cian, nc)
	fmt.Printf("  %sBienvenido! Acá creás y gestionás los accesos VPN.%s\n", negrita, nc)
	fmt.Println("  Cada usuario recibe una CONTRASEÑA y una FECHA de vencimiento.")
	fmt.Println("  Cuando vence, el acceso se corta solo. Podés renovar cuando quieras.")
	fmt.Println("")
}

// ─── ENTRADAS ───────────────────────────────────────────────────────────────
// entradaCerrada: se pone en true cuando se termina el stdin (Ctrl-D o prueba con pipe)
var entradaCerrada bool

func leer(prompt string) string {
	fmt.Printf("  %s", prompt)
	linea, err := entrada.ReadString('\n')
	if err != nil {
		if strings.TrimSpace(linea) == "" {
			entradaCerrada = true
		}
	}
	if err == io.EOF && strings.TrimSpace(linea) == "" {
		return ""
	}
	return strings.TrimSpace(linea)
}

func leerDef(prompt, porDefecto string) string {
	v := leer(prompt)
	if v == "" {
		return porDefecto
	}
	return v
}

func leerInt(prompt string, porDefecto int) int {
	v := leer(prompt)
	if v == "" {
		return porDefecto
	}
	n, err := strconv.Atoi(strings.TrimSpace(v))
	if err != nil {
		return porDefecto
	}
	return n
}

func leerFloat(prompt string, porDefecto float64) float64 {
	v := strings.ReplaceAll(leer(prompt), ",", ".")
	if v == "" {
		return porDefecto
	}
	f, err := strconv.ParseFloat(v, 64)
	if err != nil {
		return porDefecto
	}
	return f
}

func leerSiNo(prompt string) bool {
	v := strings.ToLower(leer(prompt + " (s/n): "))
	return v == "s" || v == "si" || v == "sí" || v == "y" || v == "yes"
}

func esperar() {
	leer("  ⏎ Enter para continuar...")
}

func pausar(seg int) {
	fmt.Printf("  %s(volviendo en %d segundos)%s\n", gris, seg, nc)
}

// ─── PRESENTACIÓN ───────────────────────────────────────────────────────────
func titulo(texto string) {
	fmt.Printf("%s%s  %s%s\n", negrita, amar, texto, nc)
	fmt.Printf("%s  %s%s\n", cian, strings.Repeat("─", min(58, len([]rune(texto))+18)), nc)
}

func oks(msgs ...string)   { fmt.Printf("  %s✅ %s%s\n", verde, strings.Join(msgs, " "), nc) }
func falla(msgs ...string) { fmt.Printf("  %s❌ %s%s\n", rojo, strings.Join(msgs, " "), nc) }
func aviso(msgs ...string) { fmt.Printf("  %s⚠️  %s%s\n", amar, strings.Join(msgs, " "), nc) }
func info(msgs ...string)  { fmt.Printf("  %s%s%s\n", cian, strings.Join(msgs, " "), nc) }
func dato(k, v string)     { fmt.Printf("  %s%-12s%s %s%s%s\n", negrita, k+":", nc, negrita, v, nc) }

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// barraUso: barra de 10 bloques ▰▱ con color según el porcentaje
func barraUso(pct int) string {
	if pct < 0 {
		pct = 0
	}
	if pct > 100 {
		pct = 100
	}
	llenos := pct / 10
	color := verde
	if pct >= 80 {
		color = rojo
	} else if pct >= 60 {
		color = amar
	}
	barra := strings.Repeat("▰", llenos) + strings.Repeat("▱", 10-llenos)
	return fmt.Sprintf("%s%s%s %3d%%", color, barra, nc, pct)
}

// celda: texto con color y relleno hasta el ancho pedido
func celda(color, texto string, ancho int) string {
	largos := len([]rune(texto))
	pad := ancho - largos
	if pad < 1 {
		pad = 1
	}
	return color + texto + nc + strings.Repeat(" ", pad)
}

// fmtBytes: bytes → B / KB / MB / GB
func fmtBytes(b int64) string {
	switch {
	case b >= 1024*1024*1024:
		return fmt.Sprintf("%.2f GB", float64(b)/(1024*1024*1024))
	case b >= 1024*1024:
		return fmt.Sprintf("%.1f MB", float64(b)/(1024*1024))
	case b >= 1024:
		return fmt.Sprintf("%.0f KB", float64(b)/1024)
	}
	return fmt.Sprintf("%d B", b)
}

// fmtGB: GB con nombre humano (para límites chicos muestra MB)
func fmtGB(gb float64) string {
	if gb <= 0 {
		return "ilimitado"
	}
	if gb >= 1024 {
		return fmt.Sprintf("%.2f TB", gb/1024)
	}
	if gb >= 1 {
		return fmt.Sprintf("%.2f GB", gb)
	}
	return fmt.Sprintf("%.0f MB", gb*1024)
}

func siNo(cond bool) string {
	if cond {
		return "✅"
	}
	return "❌"
}
