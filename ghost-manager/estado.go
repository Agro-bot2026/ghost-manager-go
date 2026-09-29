// ghost-manager en Go — estado del sistema y dashboard
package main

import (
	"fmt"
	"os"
	"strings"
	"time"
)

func estado() {
	banner()
	titulo("📊 ESTADO DEL SISTEMA")
	fmt.Printf("  🌐 IP pública:     %s%s%s\n", verde, ipPublica(), nc)
	fmt.Printf("  🟢 Puerto 80:      %s\n", estadoServicioLindo("ghostprox", "✅ Portero WS activo (ghostprox)"))
	fmt.Printf("  👤 Cuentas SSH:    %s\n", estadoServicioLindo("sshgo", "✅ sshgo activo :2200"))
	fmt.Printf("  🛡️  OpenVPN 1194:  %s\n", estadoServicioLindo("openvpn@server", "✅ activo"))
	fmt.Printf("  🚀 Xray 8443:      %s\n", estadoServicioLindo("xray", "✅ activo"))
	fmt.Printf("  🌐 Psiphon 2223:   %s\n", estadoServicioLindo("psiphon", "✅ activo"))
	fmt.Printf("  🦇 HCR :8080:      %s\n", estadoServicioLindo("hcr-server-8080", "✅ activo (método HTTP Custom)"))
	fmt.Println("")
	fmt.Printf("  👥 Usuarios SSH:   %s%d%s   🚀 V2Ray: %s%d%s\n", verde, len(leerCuentasSSH()), nc, verde, len(leerCuentasV2Ray()), nc)
	cfg := leerConfigJSON()
	fmt.Printf("  📏 Cuota por IP:   %s\n", limiteTexto(cfgFloat(cfg, "quota_gb", 0)))
	fmt.Printf("  📡 Cuota V2Ray:    %s por cuenta cada %.0f días\n", limiteTexto(cfgFloat(cfg, "v2ray_quota_gb", 0)), cfgFloat(cfg, "v2ray_quota_dias", 30))
	fmt.Printf("  🧩 Versión portero:%s %s\n", "", versionPortero())
	fmt.Println("")
}

func estadoServicioLindo(unidad, textoActivo string) string {
	if servicioActivo(unidad) {
		return verde + textoActivo + nc
	}
	if correr("systemctl", "is-active", unidad) == "failed" {
		return rojo + "❌ error" + nc
	}
	return rojo + "❌ caído" + nc
}

func limiteTexto(gb float64) string {
	if gb <= 0 {
		return amar + "SIN LÍMITE" + nc
	}
	return verde + fmtGB(gb) + nc
}

func versionPortero() string {
	out := correr(RutaGhostprox, "--version")
	if out == "" {
		return "(no encontrado)"
	}
	partes := strings.Fields(out)
	if len(partes) >= 2 {
		return partes[1]
	}
	return out
}

// ─── DASHBOARD (el panel de arriba del menú) ────────────────────────────────
func dashboard() {
	totalMB, usadoMB, ramPct := memInfo()
	totalGB, usadoGB, discoPct := discoInfo()
	carga := cargaCPU()
	osNombre := nombreOS()
	hora := time.Now().Format("15:04:05")

	titulo("🖥️  SISTEMA")
	fmt.Printf("  %s%-16s%s %s%s%s   %s⚡ Carga:%s %s%s%s\n", negrita, "Sistema:", nc, negrita, osNombre, nc, cian, nc, negrita, carga, nc)
	fmt.Printf("  %s%-16s%s %s   %s🐏 RAM:%s %s\n", negrita, "Hora:", nc, hora, cian, nc, barraUso(ramPct))
	fmt.Printf("  %s%-16s%s %d/%d MB\n", negrita, "", nc, usadoMB, totalMB)
	fmt.Printf("  %s%-16s%s %s   %s💾 Disco:%s %s\n", negrita, "Dominio:", nc, dominio(), cian, nc, barraUso(discoPct))
	fmt.Printf("  %s%-16s%s %d/%d GB\n", negrita, "", nc, usadoGB, totalGB)
	fmt.Println("")

	// servicios
	activos, apagados := 0, 0
	var apagadosNombres []string
	for _, s := range listaServicios() {
		if servicioActivo(s.Unidad) {
			activos++
		} else if correr("systemctl", "list-unit-files", s.Unidad+".service") != "" {
			apagados++
			if len(apagadosNombres) < 4 {
				apagadosNombres = append(apagadosNombres, s.Unidad)
			}
		}
	}

	// usuarios
	ssh := leerCuentasSSH()
	v2 := leerCuentasV2Ray()
	actSSH, vencSSH := 0, 0
	for _, c := range ssh {
		if c.Activo == 1 && (c.SinFecha || c.Dias >= 0) {
			actSSH++
		} else {
			vencSSH++
		}
	}
	actV2, vencV2 := 0, 0
	for _, c := range v2 {
		if c.Activo == 1 && (c.SinFecha || c.Dias >= 0) {
			actV2++
		} else {
			vencV2++
		}
	}
	cfg := leerConfigJSON()
	fmt.Printf("  %s%s%s\n", negrita, "👥 RESUMEN", nc)
	fmt.Printf("  %s%s%s  SSH: %s\n", negrita, fucsia, "  Usuarios",
		verde+fmt.Sprintf("%d activos", actSSH)+nc+" · "+amar+fmt.Sprintf("%d vencidos", vencSSH)+nc)
	fmt.Printf("  %s%s%s  V2Ray: %s\n", negrita, fucsia, "  Cuentas ",
		verde+fmt.Sprintf("%d activos", actV2)+nc+" · "+amar+fmt.Sprintf("%d vencidos", vencV2)+nc)
	fmt.Printf("  %s%s%s  Servicios: %s", negrita, fucsia, "  Stack   ", verde+fmt.Sprintf("%d arriba", activos)+nc)
	if apagados > 0 {
		fmt.Printf(" · %s (%s)", amar+fmt.Sprintf("%d abajo", apagados)+nc, strings.Join(apagadosNombres, ", "))
	}
	fmt.Println("")
	fmt.Printf("  %s%s%s  Portero: %s · cuota IP %s · cuota V2Ray %s\n", negrita, fucsia, "  Datos   ",
		versionPortero(), limiteTexto(cfgFloat(cfg, "quota_gb", 0)), limiteTexto(cfgFloat(cfg, "v2ray_quota_gb", 0)))
	fmt.Println("")
	if os.Getenv("GM_NO_CLEAR") != "1" {
		fmt.Printf("%s  ──────────────────────────────────────────────────────────%s\n", cian, nc)
	}
}
