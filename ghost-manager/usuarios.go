// ghost-manager en Go — usuarios: crear / listar / renovar / editar / eliminar
package main

import (
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// ─── 2) CREAR USUARIO SSH ───────────────────────────────────────────────────
func crearSSH() {
	banner()
	titulo("👤 CREAR USUARIO SSH")
	info("Este acceso sirve para SSH y OpenVPN (payload).")
	info("El sistema genera la contraseña SOLO — no la tenés que inventar.")
	fmt.Println("")

	usuario := leer("  👤 Nombre de usuario (ej: juan): ")
	if usuario == "" {
		falla("Nombre vacío")
		pausar(2)
		return
	}
	dias := leerInt("  ⏳ ¿Cuántos DÍAS dura? (ej 30 = 1 mes): ", 30)
	clave := claveRandom()
	vence := expDate(dias)

	// plan: solo (1 conexión) o multi (N)
	fmt.Println("")
	fmt.Printf("  %s👤 Tipo de plan:%s\n", negrita, nc)
	fmt.Printf("  %s[1]%s 👤 Solo  (1 conexión a la vez)\n", negrita, nc)
	fmt.Printf("  %s[2]%s 👥 Multi (varias conexiones — ej: familia/grupo)\n", negrita, nc)
	tipoPlan := leerDef("  Elige [1/2]: ", "1")
	tipoDB := "solo"
	maxConn := 1
	if tipoPlan == "2" {
		maxConn = leerInt("  👥 ¿Cuántas conexiones máx? (ej 5): ", 5)
		tipoDB = "multi"
	}

	// límite de datos
	fmt.Println("")
	fmt.Printf("  %s💾 Límite de DATOS del plan:%s\n", negrita, nc)
	info("(el tráfico que puede gastar este usuario)")
	cant := leerFloat("  💾 ¿Cuánto? (ej: 5, 50, 1, 2 — 0 = ilimitado): ", 0)
	fmt.Printf("  %sUnidad:%s  %s[1]%s MB   %s[2]%s GB   %s[3]%s TB\n", negrita, nc, negrita, nc, negrita, nc, negrita, nc)
	unidad := leerDef("  Elige [1/2/3]: ", "2")
	limiteGB := cant
	unidadNombre := "GB"
	switch unidad {
	case "1":
		limiteGB = cant / 1024
		unidadNombre = "MB"
	case "3":
		limiteGB = cant * 1024
		unidadNombre = "TB"
	}
	if limiteGB <= 0 {
		limiteGB = 0
	}
	limiteGB = float64(int(limiteGB*10000+0.5)) / 10000

	// usuario del sistema
	if existeUsuarioSistema(usuario) || existeUsuarioSSH(usuario) {
		falla("El usuario " + usuario + " ya existe")
		pausar(2)
		return
	}
	crearUsuarioSistema(usuario, clave, dias, maxConn)

	// base del sshgo
	if err := crearCuentaSSH(usuario, clave, vence, tipoDB, maxConn, limiteGB, unidadNombre); err != nil {
		falla("no pude guardar en la base: " + err.Error())
		pausar(2)
		return
	}

	fmt.Println("")
	oks("¡USUARIO CREADO! Guardá estos datos:")
	dato("👤 Usuario", usuario)
	dato("🔑 Password", clave)
	dato("⏳ Expira", vence+"  (se corta solo al vencer)")
	dato("🔌 Host", dominio()+":22")
	if tipoDB == "multi" {
		dato("👥 Plan", fmt.Sprintf("MULTI — hasta %d conexiones", maxConn))
	} else {
		dato("👤 Plan", "SOLO — 1 conexión a la vez")
	}
	if limiteGB > 0 {
		dato("💾 Límite", fmt.Sprintf("%.4g %s de datos", cant, unidadNombre))
	} else {
		dato("💾 Límite", "Ilimitado")
	}
	fmt.Println("")
	aviso("Pasale estos datos al cliente. Cuando venza, renovalo (opción 7).")
	fmt.Println("")
	esperar()
}

// ─── 4) CREAR USUARIO V2RAY ─────────────────────────────────────────────────
func crearV2Ray() {
	banner()
	titulo("🚀 CREAR USUARIO V2RAY")
	info("El sistema genera el UUID SOLO — el cliente solo importa el link.")
	fmt.Println("")
	fmt.Printf("  %s  ¿Para qué método?%s\n", cian, nc)
	fmt.Printf("  %s[1]%s VLESS  (Personal/HTTP Custom, puerto 80)\n", negrita, nc)
	fmt.Printf("  %s[2]%s VMess  (Dtunnel/Frontera, puerto 8080)\n", negrita, nc)
	metodo := leerDef("  👉 Elegí (1/2): ", "1")

	usuario := leer("  👤 Nombre (identificador, ej: juan): ")
	if usuario == "" {
		falla("Nombre vacío")
		pausar(2)
		return
	}
	dias := leerInt("  ⏳ ¿Cuántos DÍAS dura? (ej 30 = 1 mes): ", 30)
	limiteGB := leerFloat("  💾 Límite de GB para esta cuenta (0 = sin límite): ", 0)
	if limiteGB < 0 {
		limiteGB = 0
	}
	uuid := uuidRandom()
	vence := expDate(dias)

	// alta del uuid en xray (mismo helper que el bash)
	if xrayUUID("add", uuid) {
		oks("UUID agregado a Xray")
	} else {
		aviso("no encontré " + xrayUUIDSh() + " — el UUID queda solo en la base")
	}

	if err := crearCuentaV2Ray(usuario, uuid, vence, limiteGB); err != nil {
		falla("no pude guardar la cuenta: " + err.Error())
		pausar(2)
		return
	}

	fmt.Println("")
	oks("¡USUARIO V2RAY CREADO! Guardá estos datos:")
	dato("👤 Nombre", usuario)
	dato("🚀 UUID", uuid)
	dato("⏳ Expira", vence+"  (se corta solo al vencer)")
	if limiteGB > 0 {
		dato("💾 Límite", fmtGB(limiteGB)+" por cuenta (el portero lo corta solo)")
	} else {
		dato("💾 Límite", "sin límite propio (vale la cuota general)")
	}

	var link string
	if metodo == "2" {
		js := fmt.Sprintf(`{"v":"2","ps":"Ghost-%s","add":"%s","port":"8080","id":"%s","aid":"0","net":"tcp","type":"none","host":"","path":"","tls":""}`,
			usuario, ipPublica(), uuid)
		link = "vmess://" + base64.StdEncoding.EncodeToString([]byte(js))
		dato("🔗 Link VMess", link)
		fmt.Println("")
		info("📱 Config para la app Dtunnel/Frontera:")
		info("   Modo: V2Ray · Host: " + ipPublica() + " · Puerto: 8080")
		info("   Protocolo: VMess · UUID: " + uuid + " · Security: none")
	} else {
		link = fmt.Sprintf("vless://%s@%s:80?type=tcp&security=none&encryption=none#Ghost-%s", uuid, dominio(), usuario)
		dato("🔗 Link", link)
		fmt.Println("")
		aviso("El cliente importa el LINK en HTTP Custom (V2Ray).")
	}

	// guardar el link (como el bash)
	if err := appendLinea(rutaLinksV2Ray(), link); err != nil {
		aviso("no pude guardar el link en " + rutaLinksV2Ray())
	} else {
		oks("Link guardado también en: " + rutaLinksV2Ray())
	}
	fmt.Println("")
	esperar()
}

func appendLinea(ruta, linea string) error {
	f, err := os.OpenFile(ruta, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.WriteString(linea + "\n")
	return err
}

// ─── 6) LISTAR ──────────────────────────────────────────────────────────────
func listar() {
	banner()
	titulo("👥 USUARIOS SSH / OPENVPN")
	fmt.Printf("  %s%-14s %-19s %5s %6s %10s %10s %s%s\n", negrita, "USUARIO", "VENCE", "DÍAS", "CONEX", "PLAN", "CONSUMIDO", "LÍMITE", nc)
	ssh := leerCuentasSSH()
	if len(ssh) == 0 {
		info("(sin usuarios)")
	}
	for _, c := range ssh {
		dias := "-"
		if !c.SinFecha {
			dias = fmt.Sprintf("%d", c.Dias)
		}
		fmt.Printf("  %-14s %-19s %5s %6d %10s %10s %10s %s\n",
			recorte(c.Usuario, 14), recorte(c.Vence, 19), dias, c.MaxConn, c.Tipo,
			fmtBytes(c.Usado), fmtGB(c.LimiteGB), estadoCuenta(c.Activo == 1 && !(c.SinFecha == false && c.Dias < 0)))
	}

	fmt.Println("")
	titulo("🚀 USUARIOS V2RAY")
	fmt.Printf("  %s%-14s %-11s %-19s %5s %10s %10s%s\n", negrita, "USUARIO", "UUID", "VENCE", "DÍAS", "CONSUMIDO", "LÍMITE", nc)
	v2 := leerCuentasV2Ray()
	if len(v2) == 0 {
		info("(sin usuarios)")
	}
	for _, c := range v2 {
		dias := "-"
		if !c.SinFecha {
			dias = fmt.Sprintf("%d", c.Dias)
		}
		uuid8 := c.UUID
		if len(uuid8) > 8 {
			uuid8 = uuid8[:8] + "..."
		}
		fmt.Printf("  %-14s %-11s %-19s %5s %10s %10s %s\n",
			recorte(c.Usuario, 14), uuid8, recorte(c.Vence, 19), dias,
			fmtBytes(c.Usado), fmtGB(c.LimiteGB), estadoCuenta(c.Activo == 1 && !(c.SinFecha == false && c.Dias < 0)))
	}

	if datos, err := os.ReadFile(rutaLinksV2Ray()); err == nil && strings.TrimSpace(string(datos)) != "" {
		fmt.Println("")
		titulo("🔗 LINKS V2RAY GUARDADOS (para re-enviar)")
		fmt.Println(strings.TrimSpace(string(datos)))
	}

	fmt.Println("")
	titulo("📄 CONFIGS EN DISCO (" + RutaConfigsApp + ")")
	archivos, _ := filepath.Glob(RutaConfigsApp + "/*.hc")
	fmt.Printf("  🗂️  %d archivos .hc\n", len(archivos))
	fmt.Println("")
	esperar()
}

func estadoCuenta(activa bool) string {
	if activa {
		return verde + "✅" + nc
	}
	return rojo + "❌" + nc
}

func recorte(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n-1]) + "…"
}

// ─── 7) RENOVAR ─────────────────────────────────────────────────────────────
func renovar() {
	banner()
	titulo("🔄 RENOVAR USUARIO (agregar más días)")
	usuario := leer("  👤 Nombre del usuario: ")
	if usuario == "" {
		return
	}
	// ¿existe en alguna de las dos bases?
	venceActual := ""
	esSSH, esV2 := false, false
	for _, c := range leerCuentasSSH() {
		if c.Usuario == usuario {
			esSSH = true
			venceActual = c.Vence
		}
	}
	for _, c := range leerCuentasV2Ray() {
		if c.Usuario == usuario {
			esV2 = true
			if venceActual == "" {
				venceActual = c.Vence
			}
		}
	}
	if !esSSH && !esV2 {
		falla("no encontré el usuario " + usuario)
		pausar(2)
		return
	}
	info("Vence actualmente: " + venceActual)
	dias := leerInt("  ⏳ ¿Cuántos DÍAS agregar? (ej 30): ", 30)

	base := time.Now()
	if t, err := time.Parse("2006-01-02 15:04:05", venceActual); err == nil && t.After(base) {
		base = t
	}
	nueva := base.AddDate(0, 0, dias).Format("2006-01-02") + " 23:59:59"

	if esSSH {
		if err := renovarCuentaSSH(usuario, nueva); err != nil {
			falla("SSH: " + err.Error())
		} else {
			oks("SSH renovado hasta " + nueva)
		}
	}
	if esV2 {
		db, err := abrirDB(rutaV2RayDB())
		if err == nil {
			if _, err := db.Exec("UPDATE users SET expires_at=?, activo=1 WHERE username=?;", nueva, usuario); err != nil {
				falla("V2Ray: " + err.Error())
			} else {
				oks("V2Ray renovado hasta " + nueva)
			}
			db.Close()
		}
	}
	fmt.Println("")
	esperar()
}

// ─── 8) EDITAR ──────────────────────────────────────────────────────────────
func editar() {
	banner()
	titulo("✏️  EDITAR USUARIO (plan, clave, límite)")
	usuario := leer("  👤 Nombre del usuario: ")
	if usuario == "" {
		return
	}
	var cuentaSSH *CuentaSSH
	for _, c := range leerCuentasSSH() {
		if c.Usuario == usuario {
			cc := c
			cuentaSSH = &cc
		}
	}
	var cuentaV2 *CuentaV2Ray
	for _, c := range leerCuentasV2Ray() {
		if c.Usuario == usuario {
			cc := c
			cuentaV2 = &cc
		}
	}
	if cuentaSSH == nil && cuentaV2 == nil {
		falla("no encontré el usuario " + usuario)
		pausar(2)
		return
	}
	if cuentaSSH != nil {
		info(fmt.Sprintf("SSH: plan %s · conexiones %d · clave %s · límite %s",
			cuentaSSH.Tipo, cuentaSSH.MaxConn, cuentaSSH.Clave, fmtGB(cuentaSSH.LimiteGB)))
	} else {
		info("(es una cuenta de V2Ray)")
	}

	fmt.Println("")
	fmt.Printf("  %s¿Qué querés cambiar?%s\n", negrita, nc)
	fmt.Printf("  %s[1]%s Plan y conexiones (solo/multi)\n", negrita, nc)
	fmt.Printf("  %s[2]%s Límite de datos (MB/GB/TB)\n", negrita, nc)
	fmt.Printf("  %s[3]%s Cambiar la clave\n", negrita, nc)
	fmt.Printf("  %s[4]%s Límite de GB de V2Ray (por cuenta)\n", negrita, nc)
	op := leerDef("  Elige [1/2/3/4]: ", "1")

	switch op {
	case "1":
		fmt.Printf("  %s[1]%s 👤 Solo (1 conexión)\n  %s[2]%s 👥 Multi (varias)\n", negrita, nc, negrita, nc)
		tipoPlan := leerDef("  Elige [1/2]: ", "1")
		tipoDB, maxConn := "solo", 1
		if tipoPlan == "2" {
			maxConn = leerInt("  👥 ¿Cuántas conexiones máx? (ej 5): ", 5)
			tipoDB = "multi"
		}
		if err := actualizarCuentaSSH(usuario, map[string]interface{}{"tipo": tipoDB, "max_conn": maxConn, "activo": 1}); err != nil {
			falla(err.Error())
		} else {
			oks(fmt.Sprintf("plan %s con %d conexiones", tipoDB, maxConn))
			if !noTocarSistema() {
				os.WriteFile("/etc/ssh/sshd_config.d/"+usuario+".conf", []byte(fmt.Sprintf("MaxSessions %d\n", maxConn)), 0644)
				correr("systemctl", "restart", "ssh")
			}
		}
	case "2":
		cant := leerFloat("  💾 ¿Cuánto? (0 = ilimitado): ", 0)
		fmt.Printf("  %sUnidad:%s  [1] MB  [2] GB  [3] TB\n", negrita, nc)
		unidad := leerDef("  Elige [1/2/3]: ", "2")
		limiteGB, unidadNombre := cant, "GB"
		switch unidad {
		case "1":
			limiteGB, unidadNombre = cant/1024, "MB"
		case "3":
			limiteGB, unidadNombre = cant*1024, "TB"
		}
		if limiteGB < 0 {
			limiteGB = 0
		}
		if err := actualizarCuentaSSH(usuario, map[string]interface{}{"limit_gb": limiteGB, "limit_unidad": unidadNombre, "limit_unit": unidadNombre, "activo": 1}); err != nil {
			falla(err.Error())
		} else {
			oks("límite " + fmtGB(limiteGB) + " (" + unidadNombre + ")")
		}
	case "3":
		nueva := claveRandom()
		if err := actualizarCuentaSSH(usuario, map[string]interface{}{"password": nueva}); err != nil {
			falla(err.Error())
		}
		if !noTocarSistema() && existeUsuarioSistema(usuario) {
			correr("chpasswd", usuario+":"+nueva)
		}
		oks("clave nueva: " + nueva)
	case "4":
		if cuentaV2 == nil {
			falla("no hay una cuenta V2Ray con ese nombre")
			break
		}
		gb := leerFloat("  💾 Límite de GB para V2Ray (0 = sin tope propio): ", 0)
		db, err := abrirDB(rutaV2RayDB())
		if err == nil {
			db.Exec("UPDATE users SET limit_gb=? WHERE username=?;", gb, usuario)
			db.Close()
			oks("V2Ray: límite " + fmtGB(gb))
		}
	}
	fmt.Println("")
	esperar()
}

// ─── 9) ELIMINAR ────────────────────────────────────────────────────────────
func eliminar() {
	banner()
	titulo("🗑️  ELIMINAR USUARIO")
	usuario := leer("  👤 Nombre del usuario a eliminar: ")
	if usuario == "" {
		return
	}
	if !leerSiNo("  ⚠️  ¿Seguro que querés cortarle el acceso a " + usuario + "?") {
		info("cancelado")
		pausar(1)
		return
	}
	// sistema
	if borrarUsuarioSistema(usuario) {
		oks("usuario de sistema eliminado")
	} else {
		aviso("no era usuario de sistema")
	}
	os.Remove("/etc/ssh/sshd_config.d/" + usuario + ".conf")

	// bases
	borrarCuentaSSH(usuario)
	oks("quitado de ssh_users.db")
	uuid, _ := borrarCuentaV2Ray(usuario)
	if uuid != "" {
		if xrayUUID("remove", uuid) {
			oks("uuid quitado de Xray")
		}
		oks("quitado de v2ray_users.db")
	}

	// configs del usuario
	borrados := 0
	if archivos, err := filepath.Glob(RutaConfigsApp + "/*" + usuario + "*.hc"); err == nil {
		for _, a := range archivos {
			os.Remove(a)
			borrados++
		}
	}
	oks(fmt.Sprintf("configs del usuario borrados (%d)", borrados))
	fmt.Println("")
	esperar()
}

// ─── 24) ELIMINAR USUARIO VLESS ─────────────────────────────────────────────
func eliminarVLESS() {
	banner()
	titulo("🗑️  ELIMINAR USUARIO VLESS (V2Ray)")
	usuario := leer("  👤 Nombre del usuario VLESS: ")
	if usuario == "" {
		return
	}
	uuid, err := borrarCuentaV2Ray(usuario)
	if err != nil || uuid == "" {
		falla("no encontré el usuario en v2ray_users.db")
		pausar(2)
		return
	}
	oks("quitado de v2ray_users.db (uuid " + uuid[:8] + "...)")
	if xrayUUID("remove", uuid) {
		oks("uuid quitado de Xray")
	} else {
		aviso("no pude sacarlo de Xray (falta " + xrayUUIDSh() + ")")
	}
	fmt.Println("")
	esperar()
}
