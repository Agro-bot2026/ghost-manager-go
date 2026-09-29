// ghost-manager en Go — datos: rutas, config.json, bases SQLite, cuentas, consumo
package main

import (
	"crypto/rand"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	_ "modernc.org/sqlite"
)

// ─── RUTAS (se pueden pisar por entorno para PRUEBAS) ────────────────────────
func env0(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

const (
	RutaConfigDir   = "/etc/ctmanager/config"
	RutaCTManager   = "/etc/ctmanager"
	RutaConfigJSON  = "/etc/ctmanager/websocket/config.json"
	RutaDominio     = "/etc/ctmanager/websocket/dominio"
	RutaXrayConfig  = "/usr/local/etc/xray/config.json"
	RutaXrayUUIDSh  = "/usr/local/bin/xray_uuid.sh"
	RutaXrayBin     = "/usr/local/bin/xray"
	RutaOVPNDir     = "/etc/ctmanager/ovpn"
	RutaConfigsApp  = "/opt/ghost-configs"
	RutaGhostprox   = "/root/ghost-go/ghostprox"
	RutaPymanagerSh = "/root/ghost-go/volver-al-pymanager.sh"
)

func rutaSSHDB() string       { return env0("GM_SSH_DB", RutaConfigDir+"/ssh_users.db") }
func rutaV2RayDB() string     { return env0("GM_V2RAY_DB", RutaConfigDir+"/v2ray_users.db") }
func rutaTraficoDB() string   { return env0("GM_TRAFICO_DB", RutaConfigDir+"/trafico.db") }
func rutaConfigJSON() string  { return env0("GM_CONFIG_JSON", RutaConfigJSON) }
func rutaDominioFile() string { return env0("GM_DOMINIO_FILE", RutaDominio) }
func rutaBannerFile() string  { return env0("GM_BANNER_FILE", RutaConfigDir+"/sshgo_banner.txt") }
func rutaLinksV2Ray() string  { return env0("GM_LINKS_FILE", RutaConfigDir+"/v2ray_links.txt") }
func rutaOVPNDirReal() string { return env0("GM_OVPN_DIR", RutaOVPNDir) }
func xrayUUIDSh() string      { return env0("GM_XRAY_UUID_SH", RutaXrayUUIDSh) }

// ─── SQLITE ─────────────────────────────────────────────────────────────────
var dbMu sync.Mutex

func abrirDB(ruta string) (*sql.DB, error) {
	db, err := sql.Open("sqlite", ruta)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	db.Exec("PRAGMA busy_timeout=5000")
	return db, nil
}

// ─── FECHAS / GENERADORES ───────────────────────────────────────────────────
func expDate(dias int) string {
	return time.Now().AddDate(0, 0, dias).Format("2006-01-02 15:04:05")
}

func diasRestantes(vence string) (int, bool) {
	if strings.TrimSpace(vence) == "" {
		return 0, true
	}
	for _, layout := range []string{"2006-01-02 15:04:05", "2006-01-02"} {
		if t, err := time.Parse(layout, vence); err == nil {
			return int(time.Until(t).Hours() / 24), false
		}
	}
	return 0, true
}

func claveRandom() string {
	b := make([]byte, 4)
	rand.Read(b)
	return fmt.Sprintf("%x", b)
}

func uuidRandom() string {
	b := make([]byte, 16)
	rand.Read(b)
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

// ─── MIGRACIONES (igual que el bash: agregar columnas si faltan) ────────────
func migrarTablaSSH(db *sql.DB) {
	db.Exec("CREATE TABLE IF NOT EXISTS users (id INTEGER PRIMARY KEY AUTOINCREMENT, username TEXT UNIQUE NOT NULL, password TEXT NOT NULL, created_at TEXT DEFAULT (datetime('now')), expires_at TEXT, activo INTEGER DEFAULT 1, tipo TEXT DEFAULT 'solo', max_conn INTEGER DEFAULT 1, limit_gb REAL DEFAULT 0);")
	// ⚠️ el bash escribe 'limit_unidad' y sshgo lee 'limit_unit': dejo LAS DOS
	for _, alter := range []string{
		"ALTER TABLE users ADD COLUMN tipo TEXT DEFAULT 'solo';",
		"ALTER TABLE users ADD COLUMN max_conn INTEGER DEFAULT 1;",
		"ALTER TABLE users ADD COLUMN limit_gb REAL DEFAULT 0;",
		"ALTER TABLE users ADD COLUMN limit_unidad TEXT DEFAULT 'GB';",
		"ALTER TABLE users ADD COLUMN limit_unit TEXT DEFAULT 'GB';",
	} {
		db.Exec(alter)
	}
}

func migrarTablaV2Ray(db *sql.DB) {
	db.Exec("CREATE TABLE IF NOT EXISTS users (username TEXT PRIMARY KEY, uuid TEXT, expires_at TEXT, activo INTEGER DEFAULT 1);")
	db.Exec("ALTER TABLE users ADD COLUMN limit_gb REAL DEFAULT 0;")
}

// ─── COLUMNAS REALES DE LA TABLA (cada instalación tiene las que tiene) ─────
// El bash y el sshgo toleran esquemas viejos; acá hago lo mismo: leo qué columnas
// existen de verdad y armo la consulta con esas (si falta una, uso un valor por defecto).
func columnasTabla(db *sql.DB, tabla string) map[string]bool {
	cols := map[string]bool{}
	rows, err := db.Query("PRAGMA table_info(" + tabla + ");")
	if err != nil {
		return cols
	}
	defer rows.Close()
	for rows.Next() {
		var cid int
		var nombre, tipo string
		var notnull, pk int
		var dflt interface{}
		if rows.Scan(&cid, &nombre, &tipo, &notnull, &dflt, &pk) == nil {
			cols[strings.ToLower(nombre)] = true
		}
	}
	return cols
}

// campo devuelve "COALESCE(col, def)" si la columna existe, o el valor por defecto
func campo(cols map[string]bool, col, def string) string {
	if cols[col] {
		return "COALESCE(" + col + "," + def + ")"
	}
	return def
}

// ─── CUENTAS SSH ────────────────────────────────────────────────────────────
type CuentaSSH struct {
	Usuario  string
	Clave    string
	Vence    string
	Activo   int
	Tipo     string
	MaxConn  int
	LimiteGB float64
	Unidad   string
	Usado    int64
	Dias     int
	SinFecha bool
}

func leerCuentasSSH() []CuentaSSH {
	db, err := abrirDB(rutaSSHDB())
	if err != nil {
		return nil
	}
	defer db.Close()
	cols := columnasTabla(db, "users")
	if len(cols) == 0 {
		return nil
	}
	unidad := campo(cols, "limit_unidad", campo(cols, "limit_unit", "'GB'"))
	q := "SELECT username, " + campo(cols, "password", "''") + ", " + campo(cols, "expires_at", "''") + ", " +
		campo(cols, "activo", "1") + ", " + campo(cols, "tipo", "'solo'") + ", " + campo(cols, "max_conn", "1") + ", " +
		campo(cols, "limit_gb", "0") + ", " + unidad + " FROM users ORDER BY expires_at DESC;"
	rows, err := db.Query(q)
	if err != nil {
		return nil
	}
	defer rows.Close()
	var out []CuentaSSH
	for rows.Next() {
		var c CuentaSSH
		if err := rows.Scan(&c.Usuario, &c.Clave, &c.Vence, &c.Activo, &c.Tipo, &c.MaxConn, &c.LimiteGB, &c.Unidad); err != nil {
			continue
		}
		c.Dias, c.SinFecha = diasRestantes(c.Vence)
		out = append(out, c)
	}
	// consumo por usuario (tabla del sshgo)
	if dbT, err := abrirDB(rutaTraficoDB()); err == nil {
		for i := range out {
			var t int64
			if err := dbT.QueryRow("SELECT COALESCE(SUM(rx_bytes+tx_bytes),0) FROM ssh_user_traffic WHERE username=?;", out[i].Usuario).Scan(&t); err == nil {
				out[i].Usado = t
			}
		}
		dbT.Close()
	}
	return out
}

func existeUsuarioSSH(usuario string) bool {
	for _, c := range leerCuentasSSH() {
		if c.Usuario == usuario {
			return true
		}
	}
	return false
}

func crearCuentaSSH(usuario, clave, vence, tipo string, maxConn int, limiteGB float64, unidad string) error {
	db, err := abrirDB(rutaSSHDB())
	if err != nil {
		return err
	}
	defer db.Close()
	migrarTablaSSH(db)
	_, err = db.Exec("INSERT OR REPLACE INTO users (username, password, expires_at, activo, tipo, max_conn, limit_gb, limit_unidad, limit_unit) VALUES (?,?,?,1,?,?,?,?,?);",
		usuario, clave, vence, tipo, maxConn, limiteGB, unidad, unidad)
	return err
}

func renovarCuentaSSH(usuario string, nuevaFecha string) error {
	db, err := abrirDB(rutaSSHDB())
	if err != nil {
		return err
	}
	defer db.Close()
	_, err = db.Exec("UPDATE users SET expires_at=?, activo=1 WHERE username=?;", nuevaFecha, usuario)
	return err
}

func actualizarCuentaSSH(usuario string, campos map[string]interface{}) error {
	db, err := abrirDB(rutaSSHDB())
	if err != nil {
		return err
	}
	defer db.Close()
	var sets []string
	var args []interface{}
	for k, v := range campos {
		sets = append(sets, k+"=?")
		args = append(args, v)
	}
	args = append(args, usuario)
	_, err = db.Exec("UPDATE users SET "+strings.Join(sets, ", ")+" WHERE username=?;", args...)
	return err
}

func borrarCuentaSSH(usuario string) error {
	db, err := abrirDB(rutaSSHDB())
	if err != nil {
		return err
	}
	defer db.Close()
	_, err = db.Exec("DELETE FROM users WHERE username=?;", usuario)
	return err
}

// ─── CUENTAS V2RAY ──────────────────────────────────────────────────────────
type CuentaV2Ray struct {
	Usuario  string
	UUID     string
	Vence    string
	Activo   int
	LimiteGB float64
	Usado    int64
	Dias     int
	SinFecha bool
}

func leerCuentasV2Ray() []CuentaV2Ray {
	db, err := abrirDB(rutaV2RayDB())
	if err != nil {
		return nil
	}
	defer db.Close()
	cols := columnasTabla(db, "users")
	if len(cols) == 0 {
		return nil
	}
	q := "SELECT username, " + campo(cols, "uuid", "''") + ", " + campo(cols, "expires_at", "''") + ", " +
		campo(cols, "activo", "1") + ", " + campo(cols, "limit_gb", "0") + " FROM users ORDER BY expires_at DESC;"
	rows, err := db.Query(q)
	if err != nil {
		return nil
	}
	defer rows.Close()
	var out []CuentaV2Ray
	for rows.Next() {
		var c CuentaV2Ray
		if err := rows.Scan(&c.Usuario, &c.UUID, &c.Vence, &c.Activo, &c.LimiteGB); err != nil {
			continue
		}
		c.Dias, c.SinFecha = diasRestantes(c.Vence)
		out = append(out, c)
	}
	if dbT, err := abrirDB(rutaTraficoDB()); err == nil {
		for i := range out {
			var t int64
			if err := dbT.QueryRow("SELECT COALESCE(SUM(rx_bytes+tx_bytes),0) FROM v2ray_user_traffic WHERE username=?;", out[i].Usuario).Scan(&t); err == nil {
				out[i].Usado = t
			}
		}
		dbT.Close()
	}
	return out
}

func crearCuentaV2Ray(usuario, uuid, vence string, limiteGB float64) error {
	db, err := abrirDB(rutaV2RayDB())
	if err != nil {
		return err
	}
	defer db.Close()
	migrarTablaV2Ray(db)
	_, err = db.Exec("INSERT OR REPLACE INTO users (username, uuid, expires_at, activo, limit_gb) VALUES (?,?,?,1,?);", usuario, uuid, vence, limiteGB)
	return err
}

func borrarCuentaV2Ray(usuario string) (string, error) {
	db, err := abrirDB(rutaV2RayDB())
	if err != nil {
		return "", err
	}
	defer db.Close()
	var uuid string
	db.QueryRow("SELECT COALESCE(uuid,'') FROM users WHERE username=?;", usuario).Scan(&uuid)
	_, err = db.Exec("DELETE FROM users WHERE username=?;", usuario)
	return uuid, err
}

// xrayUUID: alta/baja del UUID en el xray (usa el mismo helper que el bash)
func xrayUUID(accion, uuid string) bool {
	sh := xrayUUIDSh()
	if _, err := os.Stat(sh); err != nil {
		return false
	}
	out, _ := exec.Command(sh, accion, uuid).CombinedOutput()
	if strings.TrimSpace(string(out)) != "" {
		fmt.Printf("  %s%s%s\n", gris, strings.TrimSpace(string(out)), nc)
	}
	exec.Command(sh, "reload").Run()
	return true
}

// ─── CONSUMO (tabla trafico del portero) ────────────────────────────────────
func ipAlterna(ip string) string {
	if strings.HasPrefix(ip, "::ffff:") {
		return strings.TrimPrefix(ip, "::ffff:")
	}
	if ip == "" || ip == "0.0.0.0" || ip == "::" || strings.Contains(ip, ":") {
		return ""
	}
	return "::ffff:" + ip
}

func consumoIP(ip string, dias int) int64 {
	db, err := abrirDB(rutaTraficoDB())
	if err != nil {
		return 0
	}
	defer db.Close()
	if dias <= 0 {
		dias = 30
	}
	alt := ipAlterna(ip)
	var t int64
	db.QueryRow("SELECT COALESCE(SUM(rx_bytes+tx_bytes),0) FROM trafico WHERE src_ip IN (?,?) AND fecha >= datetime('now', ?);",
		ip, alt, fmt.Sprintf("-%d days", dias)).Scan(&t)
	return t
}

func consumoCuentaV2Ray(usuario string, dias int) int64 {
	db, err := abrirDB(rutaTraficoDB())
	if err != nil {
		return 0
	}
	defer db.Close()
	if dias <= 0 {
		dias = 30
	}
	var t int64
	db.QueryRow("SELECT COALESCE(SUM(rx_bytes+tx_bytes),0) FROM v2ray_user_traffic WHERE username=? AND fecha >= datetime('now', ?);",
		usuario, fmt.Sprintf("-%d days", dias)).Scan(&t)
	return t
}

// ─── CONFIG.JSON (cuotas y límites) ─────────────────────────────────────────
func leerConfigJSON() map[string]interface{} {
	m := map[string]interface{}{}
	datos, err := os.ReadFile(rutaConfigJSON())
	if err != nil {
		return m
	}
	json.Unmarshal(datos, &m)
	return m
}

func cfgFloat(m map[string]interface{}, clave string, def float64) float64 {
	if v, ok := m[clave]; ok {
		switch t := v.(type) {
		case float64:
			return t
		case int:
			return float64(t)
		}
	}
	return def
}

func guardarConfigJSON(m map[string]interface{}) error {
	datos, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	tmp := rutaConfigJSON() + ".tmp"
	if err := os.WriteFile(tmp, datos, 0644); err != nil {
		return err
	}
	return os.Rename(tmp, rutaConfigJSON())
}

// recargarPortero: le manda SIGHUP a ghostprox para que tome la config nueva
func recargarPortero() bool {
	out, _ := exec.Command("pgrep", "-f", "[g]hostprox").Output()
	pid := strings.TrimSpace(strings.Split(string(out), "\n")[0])
	if pid == "" {
		return false
	}
	if err := exec.Command("kill", "-HUP", pid).Run(); err != nil {
		return false
	}
	return true
}
