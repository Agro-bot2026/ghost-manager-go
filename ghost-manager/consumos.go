// ghost-manager en Go — consumos: conexiones en vivo, cuota por IP, por cuenta, top
package main

import (
	"fmt"
	"os/exec"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// ─── 19) CONEXIONES EN VIVO (desde el log del portero) ──────────────────────
type conexion struct {
	IP     string
	Proto  string
	Cuenta string
	Veces  int
	Puerto string
}

func verConexiones() {
	banner()
	titulo("📡 CONEXIONES EN VIVO")
	asegurarGeoDB()
	lineas := leerLogPortero()
	if len(lineas) == 0 {
		aviso("No hay conexiones registradas todavía.")
		info("Cuando alguien conecte con un config, aparece acá.")
		fmt.Println("")
		esperar()
		return
	}
	cx := agruparConexiones(lineas)
	fmt.Printf("  %s%3s  %-18s %-18s %-12s %-14s %6s%s\n", negrita, "#", "IP PÚBLICA", "PAÍS", "PROTOCOLO", "CUENTA", "CONEX", nc)
	fmt.Printf("%s  %s%s\n", cian, strings.Repeat("─", 78), nc)
	ips := map[string]bool{}
	for i, c := range cx {
		ips[c.IP] = true
		if i >= 20 {
			break
		}
		fmt.Printf("  %s%3d%s  %-18s %-18s %-12s %-14s %s%6d%s\n",
			fucsia, i+1, nc, c.IP, paisDe(c.IP), c.Proto, recorte(c.Cuenta, 14), naranja, c.Veces, nc)
	}
	fmt.Printf("%s  %s%s\n", cian, strings.Repeat("─", 78), nc)
	total := 0
	for _, c := range cx {
		total += c.Veces
	}
	oks(fmt.Sprintf("Total: %d conexiones · %d IPs únicas (últimas 48h)", total, len(ips)))
	fmt.Println("")
	esperar()
}

var reTunel = regexp.MustCompile(`Tunel ([0-9a-fA-F:.]+) -> ([0-9.]+):([0-9]+) \(([^)]+)\)`)

func leerLogPortero() []conexion {
	out, err := exec.Command("journalctl", "-u", "ghostprox", "--since", "48 hours ago", "--no-pager", "-o", "cat").Output()
	if err != nil && len(out) == 0 {
		return nil
	}
	var cx []conexion
	for _, l := range strings.Split(string(out), "\n") {
		m := reTunel.FindStringSubmatch(l)
		if m == nil {
			continue
		}
		c := conexion{IP: strings.TrimPrefix(m[1], "::ffff:"), Proto: m[4], Puerto: m[3]}
		if i := strings.Index(c.Proto, "· cuenta"); i >= 0 {
			c.Cuenta = strings.TrimSpace(strings.TrimPrefix(c.Proto[i:], "· cuenta"))
			c.Proto = strings.TrimSpace(c.Proto[:i])
		}
		cx = append(cx, c)
	}
	return cx
}

func agruparConexiones(cx []conexion) []conexion {
	mapa := map[string]*conexion{}
	for _, c := range cx {
		k := c.IP + "|" + c.Proto + "|" + c.Cuenta
		if v, ok := mapa[k]; ok {
			v.Veces++
			continue
		}
		cc := c
		cc.Veces = 1
		mapa[k] = &cc
	}
	var out []conexion
	for _, v := range mapa {
		out = append(out, *v)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Veces > out[j].Veces })
	return out
}

// ─── 20) CONSUMO DE DATOS / CUOTA GENERAL ───────────────────────────────────
func verConsumo() {
	banner()
	titulo("📊 CONSUMO DE DATOS (cuota)")
	cfg := leerConfigJSON()
	cuotaIP := cfgFloat(cfg, "quota_gb", 0)
	dias := int(cfgFloat(cfg, "quota_dias", 30))
	cuotaV2 := cfgFloat(cfg, "v2ray_quota_gb", 0)

	fmt.Printf("  📏 Cuota POR IP:        %s (cada %d días)\n", limiteTexto(cuotaIP), dias)
	fmt.Printf("  📡 Cuota por CUENTA V2Ray: %s (cada %.0f días)\n", limiteTexto(cuotaV2), cfgFloat(cfg, "v2ray_quota_dias", 30))
	fmt.Printf("  👥 Conexiones por IP:   %d (max_connections_per_user)\n", int(cfgFloat(cfg, "max_connections_per_user", 50)))
	fmt.Println("")
	topIPs(12)
	fmt.Println("")
	if leerSiNo("  ¿Querés cambiar la cuota POR IP (la que corta por IP)?") {
		nueva := leerFloat(fmt.Sprintf("  📏 GB por IP (0 = sin límite, actual %.1f): ", cuotaIP), cuotaIP)
		cfg["quota_gb"] = nueva
		cfg["quota_dias"] = leerInt(fmt.Sprintf("  📅 Cada cuántos días (actual %d): ", dias), dias)
		if err := guardarConfigJSON(cfg); err != nil {
			falla(err.Error())
		} else {
			oks("guardado en " + rutaConfigJSON())
			if recargarPortero() {
				oks("el portero lo tomó al toque (SIGHUP)")
			} else {
				aviso("el portero no está corriendo: se aplica cuando arranque")
			}
		}
	}
	fmt.Println("")
	if leerSiNo("  ¿Querés cambiar la cuota POR CUENTA de V2Ray?") {
		nueva := leerFloat(fmt.Sprintf("  📡 GB por cuenta (0 = sin límite, actual %.1f): ", cuotaV2), cuotaV2)
		cfg["v2ray_quota_gb"] = nueva
		cfg["v2ray_quota_dias"] = leerInt("  📅 Cada cuántos días (actual 30): ", 30)
		if err := guardarConfigJSON(cfg); err != nil {
			falla(err.Error())
		} else {
			oks("guardado en " + rutaConfigJSON())
			if recargarPortero() {
				oks("el portero lo tomó al toque (SIGHUP)")
			}
		}
	}
	fmt.Println("")
	esperar()
}

func topIPs(cantidad int) {
	titulo("🏆 TOP IPs POR CONSUMO (últimos 30 días)")
	asegurarGeoDB()
	db, err := abrirDB(rutaTraficoDB())
	if err != nil {
		falla("no pude abrir " + rutaTraficoDB())
		return
	}
	defer db.Close()
	rows, err := db.Query("SELECT src_ip, SUM(rx_bytes+tx_bytes) AS t, SUM(conexiones) FROM trafico WHERE fecha >= datetime('now','-30 days') GROUP BY src_ip ORDER BY t DESC LIMIT ?;", cantidad)
	if err != nil {
		falla(err.Error())
		return
	}
	defer rows.Close()
	fmt.Printf("  %s%-20s %12s %8s  %s%s\n", negrita, "IP", "CONSUMIDO", "CONEX", "PAÍS", nc)
	n := 0
	for rows.Next() {
		var ip string
		var t, cx int64
		if rows.Scan(&ip, &t, &cx) != nil {
			continue
		}
		n++
		fmt.Printf("  %-20s %12s %8d  %s\n", strings.TrimPrefix(ip, "::ffff:"), fmtBytes(t), cx, paisDe(strings.TrimPrefix(ip, "::ffff:")))
	}
	if n == 0 {
		info("(todavía no hay tráfico anotado)")
	}
}

// ─── 21) CONSUMO POR CUENTA (SSH + V2Ray) ───────────────────────────────────
func verConsumoCuentas() {
	banner()
	titulo("📊 CONSUMO POR CUENTA")
	cfg := leerConfigJSON()
	cuotaV2 := cfgFloat(cfg, "v2ray_quota_gb", 0)

	fmt.Printf("%s  🚀 V2RAY%s  (cuota general: %s)\n", negrita, nc, limiteTexto(cuotaV2))
	fmt.Printf("  %s%-14s %-8s %-19s %5s %12s %10s%s\n", negrita, "USUARIO", "ESTADO", "VENCE", "DÍAS", "CONSUMIDO", "LÍMITE", nc)
	v2 := leerCuentasV2Ray()
	if len(v2) == 0 {
		info("(sin cuentas)")
	}
	for _, c := range v2 {
		limite := c.LimiteGB
		if limite <= 0 {
			limite = cuotaV2
		}
		estado := "activo"
		usado := consumoCuentaV2Ray(c.Usuario, 30)
		if c.Usado > usado {
			usado = c.Usado
		}
		switch {
		case c.Activo != 1:
			estado = "inactivo"
		case !c.SinFecha && c.Dias < 0:
			estado = "vencido"
		case limite > 0 && float64(usado) >= limite*1024*1024*1024:
			estado = "PASADO"
		}
		dias := "-"
		if !c.SinFecha {
			dias = strconv.Itoa(c.Dias)
		}
		color := verde
		if estado == "PASADO" {
			color = rojo
		} else if estado != "activo" {
			color = amar
		}
		fmt.Printf("  %-14s %s%-8s%s %-19s %5s %12s %10s\n", recorte(c.Usuario, 14), color, estado, nc,
			recorte(c.Vence, 19), dias, fmtBytes(usado), fmtGB(limite))
	}

	fmt.Println("")
	fmt.Printf("%s  👤 SSH%s  (el corte por usuario lo hace sshgo)\n", negrita, nc)
	fmt.Printf("  %s%-14s %-8s %-19s %5s %12s %10s %6s%s\n", negrita, "USUARIO", "ESTADO", "VENCE", "DÍAS", "CONSUMIDO", "LÍMITE", "CONEX", nc)
	ssh := leerCuentasSSH()
	if len(ssh) == 0 {
		info("(sin cuentas)")
	}
	for _, c := range ssh {
		estado := "activo"
		color := verde
		switch {
		case c.Activo != 1:
			estado, color = "inactivo", amar
		case !c.SinFecha && c.Dias < 0:
			estado, color = "vencido", amar
		case c.LimiteGB > 0 && float64(c.Usado) >= c.LimiteGB*1024*1024*1024:
			estado, color = "PASADO", rojo
		}
		dias := "-"
		if !c.SinFecha {
			dias = strconv.Itoa(c.Dias)
		}
		fmt.Printf("  %-14s %s%-8s%s %-19s %5s %12s %10s %6d\n", recorte(c.Usuario, 14), color, estado, nc,
			recorte(c.Vence, 19), dias, fmtBytes(c.Usado), fmtGB(c.LimiteGB), c.MaxConn)
	}

	fmt.Println("")
	topIPs(8)
	fmt.Println("")
	esperar()
}

// ─── 22) CONSUMO PSIPHON ────────────────────────────────────────────────────
func verConsumoPsiphon() {
	banner()
	titulo("📊 CONSUMO PSIPHON POR IP (últimos 30 días)")
	db, err := abrirDB(rutaTraficoDB())
	if err != nil {
		falla("no pude abrir " + rutaTraficoDB())
		esperar()
		return
	}
	defer db.Close()
	rows, err := db.Query("SELECT src_ip, SUM(rx_bytes+tx_bytes) AS t, SUM(conexiones) FROM trafico WHERE protocolo='psiphon' AND fecha >= datetime('now','-30 days') GROUP BY src_ip ORDER BY t DESC LIMIT 20;")
	if err != nil {
		falla(err.Error())
		esperar()
		return
	}
	defer rows.Close()
	var total int64
	n := 0
	fmt.Printf("  %s%3s  %-20s %12s %8s  %s%s\n", negrita, "#", "IP", "CONSUMIDO", "CONEX", "PAÍS", nc)
	for rows.Next() {
		var ip string
		var t, cx int64
		if rows.Scan(&ip, &t, &cx) != nil {
			continue
		}
		total += t
		n++
		fmt.Printf("  %3d  %-20s %12s %8d  %s\n", n, strings.TrimPrefix(ip, "::ffff:"), fmtBytes(t), cx, paisDe(strings.TrimPrefix(ip, "::ffff:")))
	}
	if n == 0 {
		info("(sin sesiones de Psiphon en los últimos 30 días)")
	} else {
		fmt.Println("")
		oks("Total Psiphon: " + fmtBytes(total))
	}
	fmt.Println("")
	esperar()
}

// ─── 31) LÍMITE DE GB POR CUENTA ────────────────────────────────────────────
func cmdLimiteGB() {
	banner()
	titulo("📏 LÍMITE DE GB DE UNA CUENTA")
	usuario := leer("  👤 Nombre de la cuenta: ")
	if usuario == "" {
		return
	}
	// ¿existe en V2Ray, en SSH o en las dos?
	var v2 *CuentaV2Ray
	for _, c := range leerCuentasV2Ray() {
		if c.Usuario == usuario {
			cc := c
			v2 = &cc
		}
	}
	var ss *CuentaSSH
	for _, c := range leerCuentasSSH() {
		if c.Usuario == usuario {
			cc := c
			ss = &cc
		}
	}
	if v2 == nil && ss == nil {
		falla("no encontré la cuenta " + usuario)
		pausar(2)
		return
	}
	if v2 != nil {
		info(fmt.Sprintf("V2Ray: consumido %s · límite propio %s", fmtBytes(v2.Usado), fmtGB(v2.LimiteGB)))
	}
	if ss != nil {
		info(fmt.Sprintf("SSH: consumido %s · límite %s", fmtBytes(ss.Usado), fmtGB(ss.LimiteGB)))
	}
	gb := leerFloat("  💾 Nuevo límite en GB (0 = sin tope; ej 50): ", -1)
	if gb < 0 {
		info("no cambié nada")
		pausar(1)
		return
	}
	if v2 != nil {
		db, err := abrirDB(rutaV2RayDB())
		if err == nil {
			if _, err := db.Exec("UPDATE users SET limit_gb=? WHERE username=?;", gb, usuario); err != nil {
				falla(err.Error())
			} else {
				oks(fmt.Sprintf("V2Ray '%s': límite %s", usuario, fmtGB(gb)))
			}
			db.Close()
		}
	}
	if ss != nil {
		if err := actualizarCuentaSSH(usuario, map[string]interface{}{"limit_gb": gb}); err != nil {
			falla(err.Error())
		} else {
			oks(fmt.Sprintf("SSH '%s': límite %s (lo corta sshgo)", usuario, fmtGB(gb)))
		}
	}
	fmt.Println("")
	esperar()
}

// ─── 32) RESETEAR EL CONTADOR DE GB ─────────────────────────────────────────
func cmdResetGB() {
	banner()
	titulo("🧹 RESETEAR EL CONTADOR DE GB")
	usuario := leer("  👤 Nombre de la cuenta: ")
	if usuario == "" {
		return
	}
	var usadoV2, usadoSSH int64
	db, err := abrirDB(rutaTraficoDB())
	if err != nil {
		falla("no pude abrir " + rutaTraficoDB())
		pausar(2)
		return
	}
	db.QueryRow("SELECT COALESCE(SUM(rx_bytes+tx_bytes),0) FROM v2ray_user_traffic WHERE username=?;", usuario).Scan(&usadoV2)
	db.QueryRow("SELECT COALESCE(SUM(rx_bytes+tx_bytes),0) FROM ssh_user_traffic WHERE username=?;", usuario).Scan(&usadoSSH)
	db.Close()
	fmt.Printf("  📡 V2Ray (lo cuenta el portero): %s\n", fmtBytes(usadoV2))
	fmt.Printf("  👤 SSH   (lo cuenta sshgo):      %s\n", fmtBytes(usadoSSH))
	fmt.Println("")
	if usadoV2 == 0 && usadoSSH == 0 {
		info("no hay consumo anotado para esa cuenta")
		pausar(2)
		return
	}
	if !leerSiNo("  ⚠️  ¿Poner en cero el contador de " + usuario + "?") {
		info("cancelado")
		pausar(1)
		return
	}
	db, err = abrirDB(rutaTraficoDB())
	if err == nil {
		if usadoV2 > 0 {
			db.Exec("DELETE FROM v2ray_user_traffic WHERE username=?;", usuario)
			oks("V2Ray: contador en cero (" + fmtBytes(usadoV2) + " liberados)")
		}
		if usadoSSH > 0 && leerSiNo("  ¿También el de SSH (sshgo)?") {
			db.Exec("DELETE FROM ssh_user_traffic WHERE username=?;", usuario)
			oks("SSH: contador en cero (" + fmtBytes(usadoSSH) + " liberados)")
		}
		db.Close()
	}
	fmt.Println("")
	esperar()
}
