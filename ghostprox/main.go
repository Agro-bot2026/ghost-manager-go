// ghostprox — el "pymanager" en Go
// v1.0.0: el "TUBO" (detecta el protocolo y pasa el byte crudo)
// v1.1.0: + CUOTA + CONTADOR DE TRÁFICO + CUENTAS  ("TAXÍMETRO")
//
// Escucha en un puerto (el :80 en producción), lee el payload,
// ADIVINA el protocolo (como el proxy.py de CHARLY_TRICKS) y reenvía.
// Además LIMPIA los encabezados del payload antes de pasarle los datos al backend.
//
// LO QUE AGREGA LA v1.1 (lo que hacía el pymanager y el Go no):
//
//	· Lee /etc/ctmanager/websocket/config.json (quota_gb, quota_dias,
//	  max_connections_per_user, puertos, payload).
//	· _registrar_trafico(): suma rx/tx por protocolo+IP en
//	  /etc/ctmanager/config/trafico.db (tabla "trafico", la misma del panel).
//	· _consumo_ip() + _quota_excedida(): si la IP pasó la cuota → 403 y corta.
//	· max_connections_per_user: tope de conexiones simultáneas por IP.
//	· CUENTAS: lee ssh_users.db (y v2ray_users.db) y las muestra
//	  (`ghostprox cuentas` / `ghostprox top`).
//	· SIGHUP = recarga la config sin reiniciar.
package main

import (
	"bytes"
	"database/sql"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	_ "modernc.org/sqlite"
)

// ─── CONFIG ──────────────────────────────────────────────────────────────────

const (
	RutaConfigJSON = "/etc/ctmanager/websocket/config.json"
	RutaTrafico    = "/etc/ctmanager/config/trafico.db"
	RutaUsuarios   = "/etc/ctmanager/config/ssh_users.db"
	RutaV2RayUsers = "/etc/ctmanager/config/v2ray_users.db"
)

type Config struct {
	Listen      string
	SSHD        int // 22    (sshd del sistema)
	SSHGo       int // 2200  (sshgo: las cuentas de los clientes)
	Dropbear    int // 444
	Psiphon     int // 2223
	PsiphonHost string
	Shadowsocks int // 8388
	VlessRaw    int // 8443
	Vmess       int // 10001
	Brook       int // 18999
	OpenVPN     int // 1194
	SlowDNS     int // 5301
	Payload     string

	// v1.1 — cuota y cuentas
	MaxConn    int     // max_connections_per_user (conexiones por IP)
	QuotaGB    float64 // quota_gb (0 = sin límite, igual que el pymanager)
	QuotaDias  int     // quota_dias (30 por defecto)
	V2rayCheck bool    // v2ray_check_users (apagado por defecto)

	// v1.2 — cuota POR CUENTA en V2Ray
	V2rayQuotaGB   float64 // v2ray_quota_gb: límite por defecto de las cuentas sin límite propio (0 = sin límite)
	V2rayQuotaDias int     // v2ray_quota_dias (30 por defecto)

	DBTrafico string // para pruebas: GHOST_TRAFICO_DB

	Debug bool
}

// archivoConfig: los nombres EXACTOS de las claves de config.json
type archivoConfig struct {
	WSPort         int     `json:"ws_port"`
	TargetHost     string  `json:"target_host"`
	TargetPort     int     `json:"target_port"`
	SSHGoHost      string  `json:"sshgo_host"`
	SSHGoPort      int     `json:"sshgo_port"`
	DropbearHost   string  `json:"dropbear_host"`
	DropbearPort   int     `json:"dropbear_port"`
	PsiphonHost    string  `json:"psiphon_host"`
	PsiphonPort    int     `json:"psiphon_port"`
	V2rayHost      string  `json:"v2ray_host"`
	V2rayPort      int     `json:"v2ray_port"`
	OVPNHost       string  `json:"ovpn_host"`
	OVPNPort       int     `json:"ovpn_port"`
	BrookHost      string  `json:"brook_host"`
	BrookPort      int     `json:"brook_port"`
	SSPort         int     `json:"ss_port"`
	SlowDNSHost    string  `json:"slowdns_host"`
	SlowDNSPort    int     `json:"slowdns_port"`
	WGHost         string  `json:"wg_host"`
	WGPort         int     `json:"wg_port"`
	Payload        string  `json:"payload"`
	MaxConn        int     `json:"max_connections_per_user"`
	QuotaGB        float64 `json:"quota_gb"`
	QuotaDias      int     `json:"quota_dias"`
	V2rayCheck     bool    `json:"v2ray_check_users"`
	V2rayQuotaGB   float64 `json:"v2ray_quota_gb"`
	V2rayQuotaDias int     `json:"v2ray_quota_dias"`
}

func envInt(k string, d int) int {
	if v := os.Getenv(k); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return d
}

// cargarConfig: primero los valores por defecto/entorno, después el archivo
// (el archivo manda, igual que en el pymanager: el python leía config.json).
func cargarConfig() Config {
	cfg := Config{
		Listen:         ":8888",
		SSHD:           envInt("SSHD", 22),
		SSHGo:          envInt("SSHGO", 2200),
		Dropbear:       envInt("DROPBEAR", 444),
		Psiphon:        envInt("PSIPHON", 2223),
		PsiphonHost:    "IP-DE-TU-VPS",
		Shadowsocks:    envInt("SS", 8388),
		VlessRaw:       envInt("VLESS", 8443),
		Vmess:          envInt("VMESS", 10001),
		Brook:          envInt("BROOK", 18999),
		OpenVPN:        envInt("OPENVPN", 1194),
		SlowDNS:        envInt("SLOWDNS", 5301),
		Payload:        "101",
		MaxConn:        envInt("MAXCONN", 50),
		QuotaDias:      30,
		V2rayQuotaDias: 30,
		DBTrafico:      rutaTraficoDB(),
		Debug:          os.Getenv("DEBUG") == "1",
	}
	cfg.SSHGo = envInt("SSHGO", 2200)
	if v := os.Getenv("GHOST_TRAFICO_DB"); v != "" {
		cfg.DBTrafico = v
	}
	if v := os.Getenv("MAXCONN"); v != "" {
		cfg.MaxConn = envInt("MAXCONN", cfg.MaxConn)
	}
	rutaCfg := RutaConfigJSON
	if v := os.Getenv("GHOST_CONFIG_JSON"); v != "" {
		rutaCfg = v
	}
	datos, err := os.ReadFile(rutaCfg)
	if err != nil {
		if cfg.Debug {
			fmt.Printf("[GO] no pude leer %s: %v (uso los defaults)\n", rutaCfg, err)
		}
		return cfg
	}
	var a archivoConfig
	if err := json.Unmarshal(datos, &a); err != nil {
		fmt.Printf("[GO] config.json ilegible (%v): uso los defaults\n", err)
		return cfg
	}
	// puertos y hosts (si vienen en el archivo, mandan)
	if a.WSPort > 0 {
		cfg.Listen = ":" + strconv.Itoa(a.WSPort)
	}
	if a.TargetPort > 0 {
		cfg.SSHD = a.TargetPort
	}
	if a.SSHGoPort > 0 {
		cfg.SSHGo = a.SSHGoPort
	}
	if a.DropbearPort > 0 {
		cfg.Dropbear = a.DropbearPort
	}
	if a.PsiphonPort > 0 {
		cfg.Psiphon = a.PsiphonPort
	}
	if a.PsiphonHost != "" {
		cfg.PsiphonHost = a.PsiphonHost
	}
	if a.V2rayPort > 0 {
		cfg.VlessRaw = a.V2rayPort
	}
	if a.SSPort > 0 {
		cfg.Shadowsocks = a.SSPort
	}
	if a.BrookPort > 0 {
		cfg.Brook = a.BrookPort
	}
	if a.OVPNPort > 0 {
		cfg.OpenVPN = a.OVPNPort
	}
	if a.SlowDNSPort > 0 {
		cfg.SlowDNS = a.SlowDNSPort
	}
	if a.Payload != "" {
		cfg.Payload = a.Payload
	}
	if a.MaxConn > 0 {
		cfg.MaxConn = a.MaxConn
	}
	// cuota: 0 o ausente = SIN límite (como el pymanager)
	cfg.QuotaGB = a.QuotaGB
	if a.QuotaDias > 0 {
		cfg.QuotaDias = a.QuotaDias
	}
	cfg.V2rayCheck = a.V2rayCheck
	// v1.2: cuota por cuenta en V2Ray (0 = sin límite)
	cfg.V2rayQuotaGB = a.V2rayQuotaGB
	if a.V2rayQuotaDias > 0 {
		cfg.V2rayQuotaDias = a.V2rayQuotaDias
	}
	// el puerto de escucha por entorno siempre manda (para probar en :8888)
	if v := os.Getenv("LISTEN"); v != "" {
		cfg.Listen = v
	}
	return cfg
}

func (c Config) String() string {
	return fmt.Sprintf("sshgo:%d sshd:%d dropbear:%d psiphon:%d ss:%d vless:%d openvpn:%d cuotaIP:%.1fGB/%dd cuotaCuenta:%.1fGB/%dd maxconn:%d",
		c.SSHGo, c.SSHD, c.Dropbear, c.Psiphon, c.Shadowsocks, c.VlessRaw, c.OpenVPN,
		c.QuotaGB, c.QuotaDias, c.V2rayQuotaGB, c.V2rayQuotaDias, c.MaxConn)
}

// ─── BASE DE DATOS (tráfico) ─────────────────────────────────────────────────

var (
	dbTrafico *sql.DB
	dbMu      sync.Mutex
)

func dbAbrir(ruta string) {
	db, err := sql.Open("sqlite", ruta)
	if err != nil {
		fmt.Printf("[GO] sin base de tráfico (%v): no cuento GB ni corto por cuota\n", err)
		return
	}
	db.SetMaxOpenConns(1)
	if _, err := db.Exec("PRAGMA busy_timeout=5000"); err != nil {
		fmt.Printf("[GO] aviso: busy_timeout: %v\n", err)
	}
	// la MISMA tabla del pymanager (así el panel sigue leyendo igual)
	if _, err := db.Exec("CREATE TABLE IF NOT EXISTS trafico (id INTEGER PRIMARY KEY AUTOINCREMENT, protocolo TEXT, src_ip TEXT, rx_bytes INTEGER DEFAULT 0, tx_bytes INTEGER DEFAULT 0, conexiones INTEGER DEFAULT 1, fecha TEXT DEFAULT (datetime('now')));"); err != nil {
		fmt.Printf("[GO] no pude crear la tabla trafico: %v\n", err)
	}
	// v1.2: consumo POR CUENTA de V2Ray (espejo de la tabla que usa sshgo para SSH)
	if _, err := db.Exec("CREATE TABLE IF NOT EXISTS v2ray_user_traffic (id INTEGER PRIMARY KEY AUTOINCREMENT, username TEXT, rx_bytes INTEGER, tx_bytes INTEGER, fecha TEXT DEFAULT (datetime('now')));"); err != nil {
		fmt.Printf("[GO] no pude crear la tabla v2ray_user_traffic: %v\n", err)
	}
	dbTrafico = db
}

// prepararV2RayUsers: le agrega la columna limit_gb a v2ray_users.db si no está
// (igual que hizo sshgo con la suya). Es aditivo: xray no lee esta base y las
// filas que ya existen quedan con límite 0 = sin límite, o sea nada cambia.
func prepararV2RayUsers() {
	db, err := sql.Open("sqlite", rutaV2RayDB())
	if err != nil {
		return
	}
	defer db.Close()
	rows, err := db.Query("PRAGMA table_info(users);")
	if err != nil {
		return // no existe la base/tabla: no toco nada
	}
	tiene := false
	for rows.Next() {
		var cid int
		var name, tipo string
		var notnull, pk int
		var def sql.NullString
		if err := rows.Scan(&cid, &name, &tipo, &notnull, &def, &pk); err == nil && name == "limit_gb" {
			tiene = true
		}
	}
	rows.Close()
	if tiene {
		return
	}
	if _, err := db.Exec("ALTER TABLE users ADD COLUMN limit_gb REAL DEFAULT 0;"); err != nil {
		fmt.Printf("[GO] aviso: no pude agregar limit_gb a v2ray_users.db: %v\n", err)
		return
	}
	fmt.Printf("[GO] ➕ agregué la columna limit_gb a v2ray_users.db (0 = sin límite)\n")
}

// registrarTrafico: espejo de _registrar_trafico() del proxy.py
// Una fila por protocolo + IP + día; se van sumando rx/tx y las conexiones.
func registrarTrafico(protocolo, ip string, rx, tx int64) {
	if dbTrafico == nil || ip == "" {
		return
	}
	if rx < 0 {
		rx = 0
	}
	if tx < 0 {
		tx = 0
	}
	hoy := time.Now().Format("2006-01-02")
	dbMu.Lock()
	defer dbMu.Unlock()
	var id int64
	err := dbTrafico.QueryRow("SELECT id FROM trafico WHERE protocolo=? AND src_ip=? AND substr(fecha,1,10)=? LIMIT 1;", protocolo, ip, hoy).Scan(&id)
	if err == nil {
		if _, err := dbTrafico.Exec("UPDATE trafico SET rx_bytes=rx_bytes+?, tx_bytes=tx_bytes+?, conexiones=conexiones+1 WHERE id=?;", rx, tx, id); err != nil {
			fmt.Printf("[GO] aviso: no pude sumar tráfico: %v\n", err)
		}
		return
	}
	if _, err := dbTrafico.Exec("INSERT INTO trafico (protocolo, src_ip, rx_bytes, tx_bytes, conexiones) VALUES (?,?,?,?,1);", protocolo, ip, rx, tx); err != nil {
		fmt.Printf("[GO] aviso: no pude anotar tráfico: %v\n", err)
	}
}

// ─── CONTADOR POR CUENTA DE V2RAY (v1.2) ─────────────────────────────────────

// registrarTraficoV2Ray: suma los bytes de una sesión a la CUENTA (no a la IP).
// Espejo de la tabla que usa sshgo para los usuarios de SSH, así el criterio
// de "cuánto gastó este cliente" es el mismo en los dos protocolos.
func registrarTraficoV2Ray(usuario string, rx, tx int64) {
	if dbTrafico == nil || usuario == "" {
		return
	}
	if rx < 0 {
		rx = 0
	}
	if tx < 0 {
		tx = 0
	}
	hoy := time.Now().Format("2006-01-02")
	dbMu.Lock()
	defer dbMu.Unlock()
	var id int64
	err := dbTrafico.QueryRow("SELECT id FROM v2ray_user_traffic WHERE username=? AND substr(fecha,1,10)=? LIMIT 1;", usuario, hoy).Scan(&id)
	if err == nil {
		if _, err := dbTrafico.Exec("UPDATE v2ray_user_traffic SET rx_bytes=rx_bytes+?, tx_bytes=tx_bytes+? WHERE id=?;", rx, tx, id); err != nil {
			fmt.Printf("[GO] aviso: no pude sumar el tráfico de la cuenta %s: %v\n", usuario, err)
		}
		return
	}
	if _, err := dbTrafico.Exec("INSERT INTO v2ray_user_traffic (username, rx_bytes, tx_bytes) VALUES (?,?,?);", usuario, rx, tx); err != nil {
		fmt.Printf("[GO] aviso: no pude anotar el tráfico de la cuenta %s: %v\n", usuario, err)
	}
	// el consumo va con cachecito de 30s: lo dejo al día con lo que acabo de sumar,
	// así el corte por cuota es inmediato (no espera a que venza el cache)
	consumoMu.Lock()
	if c, ok := consumoMem["cuenta:"+usuario]; ok {
		consumoMem["cuenta:"+usuario] = consumoGuardado{c.bytes + rx + tx, time.Now()}
	}
	consumoMu.Unlock()
}

// consumoV2Ray: bytes de esa cuenta en los últimos N días (con cachecito de 30s)
func consumoV2Ray(usuario string, dias int) int64 {
	if dbTrafico == nil || usuario == "" {
		return 0
	}
	if dias <= 0 {
		dias = 30
	}
	clave := "cuenta:" + usuario
	consumoMu.Lock()
	if c, ok := consumoMem[clave]; ok && time.Since(c.cuando) < consumoTTL {
		consumoMu.Unlock()
		return c.bytes
	}
	consumoMu.Unlock()

	var total int64
	dbMu.Lock()
	err := dbTrafico.QueryRow(
		"SELECT COALESCE(SUM(rx_bytes+tx_bytes),0) FROM v2ray_user_traffic WHERE username=? AND fecha >= datetime('now', ?);",
		usuario, fmt.Sprintf("-%d days", dias)).Scan(&total)
	dbMu.Unlock()
	if err != nil {
		total = 0
	}
	consumoMu.Lock()
	consumoMem[clave] = consumoGuardado{total, time.Now()}
	consumoMu.Unlock()
	return total
}

// borrarConsumoV2Ray: pone el contador de una cuenta en cero (para cuando renueva)
func borrarConsumoV2Ray(usuario string) (int64, int64, int) {
	if dbTrafico == nil {
		return 0, 0, 0
	}
	dbMu.Lock()
	defer dbMu.Unlock()
	var rx, tx int64
	var n int
	if err := dbTrafico.QueryRow("SELECT COALESCE(SUM(rx_bytes),0), COALESCE(SUM(tx_bytes),0), COUNT(*) FROM v2ray_user_traffic WHERE username=?;", usuario).Scan(&rx, &tx, &n); err != nil {
		return 0, 0, 0
	}
	if n == 0 {
		return 0, 0, 0
	}
	if _, err := dbTrafico.Exec("DELETE FROM v2ray_user_traffic WHERE username=?;", usuario); err != nil {
		fmt.Printf("[GO] no pude borrar el consumo de %s: %v\n", usuario, err)
		return 0, 0, 0
	}
	consumoMu.Lock()
	delete(consumoMem, "cuenta:"+usuario)
	consumoMu.Unlock()
	return rx, tx, n
}

// ─── CUOTA POR IP ────────────────────────────────────────────────────────────
type consumoGuardado struct {
	bytes  int64
	cuando time.Time
}

var (
	consumoMu  sync.Mutex
	consumoMem = map[string]consumoGuardado{}
)

const consumoTTL = 30 * time.Second

// ipAlterna: el pymanager (python, socket IPv6) guardaba "::ffff:1.2.3.4".
// Escribimos en ese formato para no cortar los totales que ya había, pero al
// sumar miramos las dos formas (así no se pierde nada viejo ni nuevo).
func ipAlterna(ip string) string {
	if strings.HasPrefix(ip, "::ffff:") {
		return strings.TrimPrefix(ip, "::ffff:")
	}
	if ip == "0.0.0.0" || ip == "::" || ip == "" {
		return ""
	}
	if strings.Contains(ip, ":") {
		return "" // IPv6 puro: no tiene forma alterna
	}
	return "::ffff:" + ip
}

// consumoIP: espejo de _consumo_ip() — bytes subidos+bajados de esa IP
// en los últimos N días.
func consumoIP(ip string, dias int) int64 {
	if dbTrafico == nil {
		return 0
	}
	clave := ip
	consumoMu.Lock()
	if c, ok := consumoMem[clave]; ok && time.Since(c.cuando) < consumoTTL {
		consumoMu.Unlock()
		return c.bytes
	}
	consumoMu.Unlock()

	ips := []string{ip}
	if alt := ipAlterna(ip); alt != "" {
		ips = append(ips, alt)
	}
	var total int64
	dbMu.Lock()
	err := dbTrafico.QueryRow(
		"SELECT COALESCE(SUM(rx_bytes+tx_bytes),0) FROM trafico WHERE src_ip IN (?,?) AND fecha >= datetime('now', ?);",
		ips[0], segundoO(ips), fmt.Sprintf("-%d days", dias)).Scan(&total)
	dbMu.Unlock()
	if err != nil {
		total = 0
	}
	consumoMu.Lock()
	consumoMem[clave] = consumoGuardado{total, time.Now()}
	consumoMu.Unlock()
	return total
}

func segundoO(ips []string) string {
	if len(ips) > 1 {
		return ips[1]
	}
	return ips[0]
}

// quotaExcedida: espejo de _quota_excedida() — 0 GB = sin límite.
func quotaExcedida(ip string, cfg Config) bool {
	if cfg.QuotaGB <= 0 {
		return false
	}
	dias := cfg.QuotaDias
	if dias <= 0 {
		dias = 30
	}
	usado := consumoIP(ip, dias)
	return float64(usado) >= cfg.QuotaGB*1024*1024*1024
}

// ─── LÍMITE DE CONEXIONES POR IP ─────────────────────────────────────────────

var (
	connMu  sync.Mutex
	connIPs = map[string]int{}
)

func sumarConexion(ip string, max int) bool {
	connMu.Lock()
	defer connMu.Unlock()
	if actual := connIPs[ip]; actual >= max {
		return false
	}
	connIPs[ip]++
	return true
}

func restarConexion(ip string) {
	connMu.Lock()
	defer connMu.Unlock()
	if connIPs[ip] <= 1 {
		delete(connIPs, ip)
		return
	}
	connIPs[ip]--
}

// ─── DETECCIÓN DEL PROTOCOLO (fiel al pymanager) ─────────────────────────────

func primerasBytes(c net.Conn, cfg Config) ([]byte, error) {
	buf := make([]byte, 0, 4096)
	tmp := make([]byte, 2048)
	c.SetReadDeadline(time.Now().Add(10 * time.Second))
	for len(buf) < 2048 {
		n, err := c.Read(tmp)
		if n > 0 {
			buf = append(buf, tmp[:n]...)
			if bytes.Contains(buf, []byte("\r\n\r\n")) || len(buf) >= 64 {
				break
			}
		}
		if err != nil {
			if len(buf) > 0 {
				break
			}
			return nil, err
		}
	}
	c.SetReadDeadline(time.Time{})
	return buf, nil
}

// puertoDelPayload: saca el puerto de "X-Online-Host: ip:22" o "CONNECT ip:22"
func puertoDelPayload(p []byte) int {
	if len(p) == 0 {
		return 0
	}
	pl := bytes.ToLower(p)
	i := bytes.Index(pl, []byte("x-online-host:"))
	if i < 0 {
		i = bytes.Index(pl, []byte("connect "))
		if i < 0 {
			return 0
		}
	}
	seg := p[i:]
	j := bytes.LastIndexByte(seg, 58) // ':'
	if j < 0 {
		return 0
	}
	n := 0
	for x := j + 1; x < len(seg) && seg[x] >= 48 && seg[x] <= 57; x++ {
		n = n*10 + int(seg[x]-48)
	}
	return n
}

// detectTarget: PORT FIEL del detect_target del pymanager.
func detectTarget(dato []byte, payload []byte, cfg Config) (host string, puerto int, proto string) {
	host = "127.0.0.1"
	sshPuerto := cfg.SSHGo
	if cfg.SSHGo == 0 {
		sshPuerto = cfg.SSHD
	}
	if p := puertoDelPayload(payload); p == cfg.SSHD {
		sshPuerto = cfg.SSHD
	}
	if len(dato) == 0 {
		return host, sshPuerto, "ssh(default)"
	}
	if bytes.HasPrefix(dato, []byte("GET /ws")) &&
		bytes.Contains(dato, []byte("Upgrade: websocket")) &&
		bytes.Contains(dato, []byte("Sec-WebSocket-Key")) {
		return host, cfg.Brook, "brook"
	}
	if len(dato) >= 6 && dato[2] == 0x01 && dato[3] == 0x00 && dato[4] == 0x00 && dato[5] == 0x01 {
		return host, cfg.SlowDNS, "slowdns"
	}
	if bytes.HasPrefix(dato, []byte("SSH-2.0-Go")) || bytes.HasPrefix(dato, []byte("SSH-2.0-Psiphon")) {
		return cfg.PsiphonHost, cfg.Psiphon, "psiphon"
	}
	if bytes.HasPrefix(dato, []byte("SSH-")) {
		return host, sshPuerto, "ssh"
	}
	if dato[0] == 0x16 && len(dato) >= 2 && dato[1] == 0x03 {
		return host, cfg.VlessRaw, "v2ray(tls)"
	}
	if dato[0] == 0x38 || dato[0] == 0x08 || dato[0] == 0x28 {
		return host, cfg.OpenVPN, "openvpn"
	}
	if len(dato) >= 3 && (dato[2] == 0x38 || dato[2] == 0x08 || dato[2] == 0x28) {
		return host, cfg.OpenVPN, "openvpn"
	}
	nn := len(dato)
	if nn > 8 {
		nn = 8
	}
	if bytes.IndexByte(dato[:nn], 0x38) >= 0 || bytes.IndexByte(dato[:nn], 0x08) >= 0 {
		return host, cfg.OpenVPN, "openvpn"
	}
	if dato[0] == 0x00 || dato[0] == 0x01 {
		return host, cfg.VlessRaw, "v2ray(raw)"
	}
	if cfg.Shadowsocks != 0 {
		return host, cfg.Shadowsocks, "shadowsocks"
	}
	return host, sshPuerto, "ssh(default)"
}

// nombreProto: el MISMO nombre que usaba el pymanager en la tabla (por puerto),
// así el panel sigue agrupando los protocolos como antes.
func nombreProto(puerto int, proto string) string {
	switch {
	case strings.HasPrefix(proto, "v2ray"):
		return "v2ray"
	case strings.HasPrefix(proto, "ssh"):
		if puerto == 22 {
			return "ssh"
		}
		return "sshgo"
	case proto == "brook", proto == "slowdns", proto == "psiphon", proto == "openvpn":
		return proto
	case strings.HasPrefix(proto, "shadowsocks"):
		return "shadowsocks"
	}
	switch puerto {
	case 5301:
		return "slowdns"
	case 2200:
		return "sshgo"
	case 22:
		return "ssh"
	case 1194:
		return "openvpn"
	case 8443, 10001:
		return "v2ray"
	case 51821:
		return "wireguard"
	case 2223:
		return "psiphon"
	case 18999:
		return "brook"
	case 8388:
		return "shadowsocks"
	}
	return "ssh"
}

func completarDato(ya []byte, c net.Conn, cfg Config) []byte {
	if !bytes.Contains(ya, []byte("HTTP/1.")) && !bytes.HasPrefix(ya, []byte("ACL")) &&
		!bytes.HasPrefix(ya, []byte("GET")) && !bytes.HasPrefix(ya, []byte("COPY")) &&
		!bytes.HasPrefix(ya, []byte("CONNECT")) && !bytes.HasPrefix(ya, []byte("POST")) {
		return ya
	}
	// igual que el pymanager: SOLO el 101 (el banner lo manda el backend)
	c.Write([]byte("HTTP/1.1 101 Switching Protocols\r\nUpgrade: websocket\r\nConnection: Upgrade\r\n\r\n"))
	if cfg.Debug {
		fmt.Printf("[GO] mande el 101 (sin banner, como el pymanager)\n")
	}
	buf := append([]byte{}, ya...)
	tmp := make([]byte, 4096)
	c.SetReadDeadline(time.Now().Add(6 * time.Second))
	for i := 0; i < 12; i++ {
		if hayDato(buf) {
			break
		}
		n, err := c.Read(tmp)
		if n > 0 {
			buf = append(buf, tmp[:n]...)
		}
		if err != nil {
			break
		}
	}
	c.SetReadDeadline(time.Time{})
	if cfg.Debug && len(buf) != len(ya) {
		fmt.Printf("[GO] complete el payload: %dB -> %dB\n", len(ya), len(buf))
	}
	return buf
}

func esInicioProtocolo(b []byte) bool {
	if len(b) == 0 {
		return false
	}
	if b[0] == 0x00 || b[0] == 0x01 || b[0] == 0x16 || b[0] == 0x38 || b[0] == 0x08 {
		return true
	}
	return bytes.HasPrefix(b, []byte("SSH-"))
}

func datosUtiles(b []byte) []byte {
	i := bytes.LastIndex(b, []byte("\r\n\r\n"))
	if i < 0 {
		return b
	}
	cand := b[i+4:]
	if esInicioProtocolo(cand) {
		return cand
	}
	for {
		prev := bytes.LastIndex(b[:i], []byte("\r\n\r\n"))
		if prev < 0 {
			break
		}
		seg := b[prev+4:]
		if esInicioProtocolo(seg) {
			return seg
		}
		i = prev
	}
	return cand
}

func hayDato(b []byte) bool {
	if i := bytes.LastIndex(b, []byte("\r\n\r\n")); i >= 0 {
		return len(b) > i+4
	}
	return true
}

// limpiarPayload: al backend le va SOLO el dato (nunca el payload del ZR).
func limpiarPayload(b []byte, proto string) ([]byte, bool) {
	if i := bytes.LastIndex(b, []byte("\r\n\r\n")); i >= 0 {
		if resto := b[i+4:]; len(resto) > 0 {
			return resto, false
		}
		return nil, true
	}
	if bytes.HasPrefix(b, []byte("GET")) || bytes.HasPrefix(b, []byte("ACL")) ||
		bytes.HasPrefix(b, []byte("POST")) || bytes.HasPrefix(b, []byte("CONNECT")) ||
		bytes.HasPrefix(b, []byte("COPY")) {
		return nil, true
	}
	return b, false
}

// ─── CUENTAS (v1.1) ─────────────────────────────────────────────────────────

type cuenta struct {
	Usuario  string
	Activo   int
	Vence    string
	SinVence bool // no tiene fecha de vencimiento
	Dias     int  // solo vale si !SinVence
	Usado    int64
	Limite   float64
	MaxConn  int
	Tipo     string
}

// vencida: la cuenta venció (sin fecha = no vence)
func (c cuenta) vencida() bool {
	return !c.SinVence && c.Dias <= 0
}

// utilizable: activa y sin vencer
func (c cuenta) utilizable() bool { return c.Activo == 1 && !c.vencida() }

// usuariosActivos: lee ssh_users.db (la MISMA fuente que sshgo)
func leerCuentas() []cuenta {
	db, err := sql.Open("sqlite", rutaUsuariosDB())
	if err != nil {
		return nil
	}
	defer db.Close()
	rows, err := db.Query("SELECT username, COALESCE(activo,1), COALESCE(expires_at,''), COALESCE(limit_gb,0), COALESCE(max_conn,1), COALESCE(tipo,'solo') FROM users ORDER BY username;")
	if err != nil {
		return nil
	}
	defer rows.Close()
	var out []cuenta
	for rows.Next() {
		var c cuenta
		if err := rows.Scan(&c.Usuario, &c.Activo, &c.Vence, &c.Limite, &c.MaxConn, &c.Tipo); err != nil {
			continue
		}
		c.Dias, c.SinVence = diasRestantes(c.Vence)
		out = append(out, c)
	}
	// tráfico por usuario (tabla del sshgo, en trafico.db)
	if dbTrafico != nil {
		dbMu.Lock()
		for i := range out {
			var t int64
			if err := dbTrafico.QueryRow("SELECT COALESCE(SUM(rx_bytes+tx_bytes),0) FROM ssh_user_traffic WHERE username=?;", out[i].Usuario).Scan(&t); err == nil {
				out[i].Usado = t
			}
		}
		dbMu.Unlock()
	}
	return out
}

// diasRestantes devuelve (días, sinFecha)
func diasRestantes(vence string) (int, bool) {
	if strings.TrimSpace(vence) == "" {
		return 0, true
	}
	t, err := time.Parse("2006-01-02 15:04:05", vence)
	if err != nil {
		t, err = time.Parse("2006-01-02", vence)
		if err != nil {
			return 0, true // fecha ilegible: no la trato como vencida
		}
	}
	return int(time.Until(t).Hours() / 24), false
}

func aGB(b int64) float64 { return float64(b) / (1024 * 1024 * 1024) }

// fmtBytes: si es menos de 1 GB lo muestra en MB (si no, todo parece "0.00 GB")
func fmtBytes(b int64) string {
	if b >= 1024*1024*1024 {
		return fmt.Sprintf("%.2f GB", aGB(b))
	}
	if b >= 1024*1024 {
		return fmt.Sprintf("%.1f MB", float64(b)/(1024*1024))
	}
	if b >= 1024 {
		return fmt.Sprintf("%.0f KB", float64(b)/1024)
	}
	return fmt.Sprintf("%d B", b)
}

// fmtLimite: el límite de GB de una cuenta (si es chico lo muestra en MB)
func fmtLimite(gb float64) string {
	if gb <= 0 {
		return "sin tope"
	}
	if gb >= 1 {
		return fmt.Sprintf("%.1f GB", gb)
	}
	return fmt.Sprintf("%.0f MB", gb*1024)
}

// ─── CUENTAS DE V2RAY (v1.1: vencimiento · v1.2: + consumo por cuenta) ───────

// v2rayCuenta: el proxy SÍ puede saber quién es la cuenta en V2Ray:
// el UUID viaja en el primer paquete (VLESS: versión 1B + UUID 16B).
var (
	v2rayMu    sync.Mutex
	v2rayCache = map[string]v2rayEntrada{}
)

type v2rayEntrada struct {
	Usuario  string
	Vence    string
	Activo   int
	SinVence bool
	Dias     int
	Limite   float64
	cuando   time.Time
}

// cuentaDeV2Ray devuelve (cuenta, encontrada): busca el UUID en v2ray_users.db
func cuentaDeV2Ray(dato []byte) (v2rayEntrada, bool) {
	if len(dato) < 17 {
		return v2rayEntrada{}, false
	}
	if dato[0] != 0x00 && dato[0] != 0x01 {
		return v2rayEntrada{}, false // no es VLESS raw: no se puede saber el uuid
	}
	uuid := strings.ToLower(hexDe(dato[1:17]))
	v2rayMu.Lock()
	if e, ok := v2rayCache[uuid]; ok && time.Since(e.cuando) < 20*time.Second {
		v2rayMu.Unlock()
		return e, true
	}
	v2rayMu.Unlock()

	db, err := sql.Open("sqlite", rutaV2RayDB())
	if err != nil {
		return v2rayEntrada{}, false
	}
	defer db.Close()
	var e v2rayEntrada
	var vence string
	err = db.QueryRow("SELECT username, COALESCE(activo,1), COALESCE(expires_at,''), COALESCE(limit_gb,0) FROM users WHERE REPLACE(LOWER(uuid),'-','')=?;", uuid).Scan(&e.Usuario, &e.Activo, &vence, &e.Limite)
	if err != nil {
		// base con el esquema VIEJO (sin la columna limit_gb): pregunto otra vez
		// sin esa columna — si no, el corte por cuenta no se haría nunca
		err = db.QueryRow("SELECT username, COALESCE(activo,1), COALESCE(expires_at,'') FROM users WHERE REPLACE(LOWER(uuid),'-','')=?;", uuid).Scan(&e.Usuario, &e.Activo, &vence)
		e.Limite = 0
	}
	if err != nil {
		return v2rayEntrada{}, false // UUID desconocido: NO se corta (nunca romper a un cliente)
	}
	e.Dias, e.SinVence = diasRestantes(vence)
	e.Vence = vence
	e.cuando = time.Now()
	v2rayMu.Lock()
	v2rayCache[uuid] = e
	v2rayMu.Unlock()
	return e, true
}

// revisarCuentaV2Ray: ¿esta conexión de V2Ray hay que cortarla?
// Devuelve (usuario, motivo) — motivo vacío = pasa.
func revisarCuentaV2Ray(dato []byte, cfg Config) (string, string) {
	e, encontrada := cuentaDeV2Ray(dato)
	if !encontrada {
		return "", "" // no se pudo identificar: se deja pasar y se cuenta por protocolo
	}
	if e.Activo != 1 {
		return e.Usuario, "la cuenta está dada de baja"
	}
	if !e.SinVence && e.Dias < 0 {
		return e.Usuario, "la cuenta está vencida"
	}
	// v1.2: cuota POR CUENTA (limit_gb de la cuenta; si es 0, el valor general de config.json)
	limite := e.Limite
	if limite <= 0 {
		limite = cfg.V2rayQuotaGB
	}
	if limite > 0 {
		dias := cfg.V2rayQuotaDias
		if dias <= 0 {
			dias = 30
		}
		usado := consumoV2Ray(e.Usuario, dias)
		if float64(usado) >= limite*1024*1024*1024 {
			return e.Usuario, fmt.Sprintf("se pasó de la cuota (%.2f GB de %.1f GB)", aGB(usado), limite)
		}
	}
	return e.Usuario, ""
}

// limiteV2Ray: el límite que le corresponde a esa cuenta (para mostrarlo)
func limiteV2Ray(e v2rayEntrada, cfg Config) float64 {
	if e.Limite > 0 {
		return e.Limite
	}
	return cfg.V2rayQuotaGB
}

// leerCuentasV2Ray: las cuentas de v2ray_users.db con su consumo
func leerCuentasV2Ray() []v2rayEntrada {
	db, err := sql.Open("sqlite", rutaV2RayDB())
	if err != nil {
		return nil
	}
	defer db.Close()
	rows, err := db.Query("SELECT username, COALESCE(activo,1), COALESCE(expires_at,''), COALESCE(limit_gb,0) FROM users ORDER BY username;")
	if err != nil {
		// esquema viejo (sin limit_gb): listo igual, sin límite
		rows, err = db.Query("SELECT username, COALESCE(activo,1), COALESCE(expires_at,''), 0 FROM users ORDER BY username;")
		if err != nil {
			return nil
		}
	}
	defer rows.Close()
	var out []v2rayEntrada
	for rows.Next() {
		var e v2rayEntrada
		var vence string
		if err := rows.Scan(&e.Usuario, &e.Activo, &vence, &e.Limite); err != nil {
			continue
		}
		e.Dias, e.SinVence = diasRestantes(vence)
		e.Vence = vence
		out = append(out, e)
	}
	return out
}

func hexDe(b []byte) string {
	const hex = "0123456789abcdef"
	out := make([]byte, 0, len(b)*2)
	for _, x := range b {
		out = append(out, hex[x>>4], hex[x&0x0f])
	}
	return string(out)
}

// rutas de las DB (se pueden pisar por entorno para probar sin tocar producción)
func rutaTraficoDB() string {
	if v := os.Getenv("GHOST_TRAFICO_DB"); v != "" {
		return v
	}
	return RutaTrafico
}

func rutaUsuariosDB() string {
	if v := os.Getenv("GHOST_USERS_DB"); v != "" {
		return v
	}
	return RutaUsuarios
}

func rutaV2RayDB() string {
	if v := os.Getenv("GHOST_V2RAY_DB"); v != "" {
		return v
	}
	return RutaV2RayUsers
}

// ─── BOMBEO ──────────────────────────────────────────────────────────────────
func forward(src, dst net.Conn, wg *sync.WaitGroup, contador *int64) {
	defer wg.Done()
	n, _ := io.Copy(dst, src)
	if contador != nil {
		*contador += n
	}
	if t, ok := dst.(*net.TCPConn); ok {
		t.CloseWrite()
	}
}

func ipDeCliente(addr net.Addr) string {
	if addr == nil {
		return ""
	}
	host, _, err := net.SplitHostPort(addr.String())
	if err != nil {
		return addr.String()
	}
	return host
}

// ipParaGuardar: el pymanager (python, socket IPv6) guardaba la IP como
// "::ffff:1.2.3.4". Escribimos igual, así los totales por IP siguen sumando
// los GB que ya venían contados (y el panel los agrupa igual que antes).
func ipParaGuardar(ip string) string {
	if ip == "" {
		return ""
	}
	if strings.HasPrefix(ip, "::ffff:") {
		return ip
	}
	if strings.Contains(ip, ":") {
		return ip // IPv6 puro: se deja como está
	}
	return "::ffff:" + ip
}

func manejar(src net.Conn, cfg Config) {
	defer src.Close()
	ip := ipDeCliente(src.RemoteAddr())

	// 1) CUOTA DE DATOS (como el pymanager: primero la cuota)
	if quotaExcedida(ip, cfg) {
		src.Write([]byte("HTTP/1.1 403 Forbidden\r\nContent-Length: 0\r\n\r\n"))
		fmt.Printf("[GO] ⛔ %s rechazada: superó la cuota de datos (%.1f GB en %d días · usado %.2f GB)\n",
			ip, cfg.QuotaGB, cfg.QuotaDias, aGB(consumoIP(ip, cfg.QuotaDias)))
		return
	}

	first, err := primerasBytes(src, cfg)
	if err != nil || len(first) == 0 {
		return
	}
	first = completarDato(first, src, cfg)
	host, puerto, proto := detectTarget(datosUtiles(first), first, cfg)
	limpio, _ := limpiarPayload(first, proto)

	// 1b) CUENTAS de V2Ray (opcional: v2ray_check_users)
	//     v1.1: corta las dadas de baja / vencidas.
	//     v1.2: además corta las que se pasaron de su cuota de GB, y el consumo
	//     de cada sesión se le anota a la CUENTA (no solo a la IP).
	cuenta := ""
	if cfg.V2rayCheck && puerto == cfg.VlessRaw {
		if user, motivo := revisarCuentaV2Ray(datosUtiles(first), cfg); user != "" {
			cuenta = user
			if motivo != "" {
				// ojo: el 101 ya salió (igual que en el pymanager), así que acá
				// solo se corta la conexión y queda el aviso en el log
				fmt.Printf("[GO] ⛔ %s rechazada: cuenta V2Ray '%s' — %s\n", ip, user, motivo)
				return
			}
		}
	}

	if cfg.Debug {
		fmt.Printf("[GO] %s -> %s:%d (%s) recibido=%dB envio=%dB\n",
			src.RemoteAddr(), host, puerto, proto, len(first), len(limpio))
	}

	// 2) LÍMITE DE CONEXIONES POR IP
	if !sumarConexion(ip, cfg.MaxConn) {
		if cfg.Debug {
			fmt.Printf("[GO] ⛔ %s ya tiene %d conexiones (max_connections_per_user)\n", ip, cfg.MaxConn)
		}
		return
	}
	defer restarConexion(ip)

	dst, err := net.DialTimeout("tcp", net.JoinHostPort(host, strconv.Itoa(puerto)), 10*time.Second)
	if err != nil {
		if cfg.Debug {
			fmt.Printf("[GO] error conectando a %d: %v\n", puerto, err)
		}
		return
	}
	defer dst.Close()

	// 3) BOMBEO + CONTADOR DE TRÁFICO
	nombre := nombreProto(puerto, proto)
	// el primer pedazo (lo que se le manda al backend) también cuenta como bajada
	var rx, tx int64
	// se anota SIEMPRE por protocolo+IP (la tabla del panel/pymanager) y, si se
	// pudo identificar la cuenta de V2Ray, ADEMÁS por cuenta (para la cuota por GB)
	anotar := func() {
		registrarTrafico(nombre, ipParaGuardar(ip), rx, tx)
		if cuenta != "" {
			registrarTraficoV2Ray(cuenta, rx, tx)
		}
	}
	if len(limpio) > 0 {
		n, err := dst.Write(limpio)
		rx += int64(n)
		if err != nil {
			anotar()
			return
		}
	}

	if cuenta != "" {
		fmt.Printf("[GO] 🚇 Tunel %s -> %s:%d (%s · cuenta %s)\n", ip, host, puerto, strings.ToUpper(nombre), cuenta)
	} else {
		fmt.Printf("[GO] 🚇 Tunel %s -> %s:%d (%s)\n", ip, host, puerto, strings.ToUpper(nombre))
	}

	var wg sync.WaitGroup
	wg.Add(2)
	go forward(src, dst, &wg, &rx)
	go forward(dst, src, &wg, &tx)
	wg.Wait()
	anotar()
	if cuenta != "" {
		fmt.Printf("[GO] 📊 %s · cuenta %s ↓%.2f MB ↑%.2f MB (acumulado %s)\n", ip, cuenta,
			float64(rx)/(1024*1024), float64(tx)/(1024*1024), fmtBytes(consumoV2Ray(cuenta, cfg.V2rayQuotaDias)))
	} else {
		fmt.Printf("[GO] 📊 %s %s ↓%.2f MB ↑%.2f MB\n", ip, strings.ToUpper(nombre), float64(rx)/(1024*1024), float64(tx)/(1024*1024))
	}
}

// ─── FIRMA Y VERSION ─────────────────────────────────────────────
const (
	Version  = "v1.2.0"
	Codename = "BALANZA" // ahora pesa a cada cuenta de V2Ray
	Autor    = "CHARLY_TRICKS"
	Firma    = "ghostprox " + Version + " \"" + Codename + "\" by " + Autor
)

func mostrarFirma() {
	fmt.Println("")
	fmt.Println("  ╔══════════════════════════════════════════════════╗")
	fmt.Println("  ║  🚀 GHOSTPROX  " + Version + strings.Repeat(" ", 28) + "║")
	fmt.Println("  ║  Ghost manager en Go · " + Codename + strings.Repeat(" ", 19) + "║")
	fmt.Println("  ║  by " + Autor + strings.Repeat(" ", 32) + "║")
	fmt.Println("  ╚══════════════════════════════════════════════════╝")
	fmt.Println("")
}

// ─── COMANDOS DE CONSULTA (cuentas / top / reset) ────────────────────────────

func cmdCuentas(filtro string) {
	if filtro != "v2ray" {
		cmdCuentasSSH()
	}
	if filtro != "ssh" {
		cmdCuentasV2Ray()
	}
}

func cmdCuentasSSH() {
	cuentas := leerCuentas()
	fmt.Printf("\n👥 CUENTAS sshgo — %s\n", rutaUsuariosDB())
	if len(cuentas) == 0 {
		fmt.Println("   (no hay cuentas todavía)")
		return
	}
	fmt.Printf("%-14s %-8s %-19s %5s %12s %10s %5s\n", "USUARIO", "ESTADO", "VENCE", "DÍAS", "USADO", "LÍMITE", "CONEX")
	activas, vencidas := 0, 0
	for _, c := range cuentas {
		estado := "activo"
		switch {
		case c.Activo != 1:
			estado = "inactivo"
			vencidas++
		case c.vencida():
			estado = "vencido"
			vencidas++
		default:
			activas++
		}
		dias := "sin fecha"
		if !c.SinVence {
			dias = strconv.Itoa(c.Dias)
		}
		vence := c.Vence
		if vence == "" {
			vence = "-"
		}
		limite := fmtLimite(c.Limite)
		fmt.Printf("%-14s %-8s %-19s %5s %12s %10s %5d\n", c.Usuario, estado, vence, dias, fmtBytes(c.Usado), limite, c.MaxConn)
	}
	fmt.Printf("\n   ✅ activas: %d   ⚠️ inactivas/vencidas: %d\n", activas, vencidas)
	fmt.Printf("   (el SSH lo corta sshgo por usuario: días y limit_gb)\n")
}

func cmdCuentasV2Ray() {
	cfg := cargarConfig()
	cuentas := leerCuentasV2Ray()
	fmt.Printf("\n📡 CUENTAS V2Ray — %s\n", rutaV2RayDB())
	if len(cuentas) == 0 {
		fmt.Println("   (no hay cuentas todavía)")
		return
	}
	fmt.Printf("%-14s %-8s %-19s %5s %12s %10s\n", "USUARIO", "ESTADO", "VENCE", "DÍAS", "USADO", "LÍMITE")
	activas, vencidas, pasadas := 0, 0, 0
	for _, e := range cuentas {
		limite := limiteV2Ray(e, cfg)
		usado := consumoV2Ray(e.Usuario, cfg.V2rayQuotaDias)
		estado := "activo"
		switch {
		case e.Activo != 1:
			estado = "inactivo"
			vencidas++
		case !e.SinVence && e.Dias < 0:
			estado = "vencido"
			vencidas++
		case limite > 0 && float64(usado) >= limite*1024*1024*1024:
			estado = "PASADO"
			pasadas++
		default:
			activas++
		}
		dias := "sin fecha"
		if !e.SinVence {
			dias = strconv.Itoa(e.Dias)
		}
		lim := fmtLimite(limite)
		vence := e.Vence
		if vence == "" {
			vence = "-"
		}
		fmt.Printf("%-14s %-8s %-19s %5s %12s %10s\n", e.Usuario, estado, vence, dias, fmtBytes(usado), lim)
	}
	fmt.Printf("\n   ✅ activas: %d   ⚠️ vencidas/inactivas: %d   📏 pasadas de cuota: %d\n", activas, vencidas, pasadas)
	if cfg.V2rayQuotaGB <= 0 {
		fmt.Printf("   (cuota general de V2Ray: SIN LÍMITE — se pone en config.json: v2ray_quota_gb)\n")
	} else {
		fmt.Printf("   (cuota general: %.1f GB cada %d días · el 'limit_gb' de cada cuenta manda)\n", cfg.V2rayQuotaGB, cfg.V2rayQuotaDias)
	}
}

// cmdLimite: muestra o cambia el límite de GB de una cuenta de V2Ray.
// (el SSH lo maneja sshgo; esto es para las cuentas de V2Ray, que no tenían con qué)
func cmdLimite(usuario string, gb float64, tieneValor bool) {
	prepararV2RayUsers()
	dbAbrir(rutaTraficoDB()) // para poder leer lo consumido
	cfg := cargarConfig()
	db, err := sql.Open("sqlite", rutaV2RayDB())
	if err != nil {
		fmt.Printf("\n❌ no pude abrir %s: %v\n\n", rutaV2RayDB(), err)
		return
	}
	defer db.Close()
	var limite float64
	var activo int
	var vence string
	// esquema nuevo y, si falla, el viejo (sin limit_gb)
	err = db.QueryRow("SELECT COALESCE(limit_gb,0), COALESCE(activo,1), COALESCE(expires_at,'') FROM users WHERE username=?;", usuario).Scan(&limite, &activo, &vence)
	if err != nil {
		err = db.QueryRow("SELECT 0, COALESCE(activo,1), COALESCE(expires_at,'') FROM users WHERE username=?;", usuario).Scan(&limite, &activo, &vence)
	}
	if err != nil {
		fmt.Printf("\n❌ no encontré la cuenta V2Ray '%s' en %s\n\n", usuario, rutaV2RayDB())
		return
	}
	usado := consumoV2Ray(usuario, 30)
	dias := "sin fecha"
	if d, sinFecha := diasRestantes(vence); !sinFecha {
		dias = fmt.Sprintf("%d días", d)
	}
	if !tieneValor {
		efectivo := limite
		origen := "propio de la cuenta"
		if efectivo <= 0 {
			efectivo = cfg.V2rayQuotaGB
			origen = fmt.Sprintf("general (config.json, %d días)", cfg.V2rayQuotaDias)
		}
		fmt.Printf("\n📡 Cuenta V2Ray '%s'\n", usuario)
		fmt.Printf("   estado: %s · vence: %s (%s)\n", map[int]string{1: "activo", 0: "inactivo"}[activo], vence, dias)
		fmt.Printf("   consumido: %s\n", fmtBytes(usado))
		if efectivo > 0 {
			fmt.Printf("   límite: %s  ← %s\n", fmtLimite(efectivo), origen)
		} else {
			fmt.Printf("   límite: SIN TOPE (no hay cuota general y la cuenta no tiene propia)\n")
		}
		fmt.Printf("   (para cambiarlo: ghostprox limite %s <GB>   ·   0 = sin tope propio)\n\n", usuario)
		return
	}
	if _, err := db.Exec("UPDATE users SET limit_gb=? WHERE username=?;", gb, usuario); err != nil {
		fmt.Printf("\n❌ no pude guardar el límite: %v\n\n", err)
		return
	}
	// que el proxy en marcha lo tome ya (tiene la cuenta cacheada 20s)
	v2rayMu.Lock()
	v2rayCache = map[string]v2rayEntrada{}
	v2rayMu.Unlock()
	consumoMu.Lock()
	delete(consumoMem, "cuenta:"+usuario)
	consumoMu.Unlock()
	fmt.Printf("\n✅ Cuenta V2Ray '%s': límite %s → %s (consumido hasta ahora: %s)\n", usuario, fmtLimite(limite), fmtLimite(gb), fmtBytes(usado))
	if gb > 0 && float64(usado) >= gb*1024*1024*1024 {
		fmt.Printf("   ⚠️ OJO: ya está por encima de ese límite → la próxima conexión se corta.\n")
	}
	fmt.Printf("   (el proxy lo toma en ≤20 segundos · para que sea YA: kill -HUP <pid de ghostprox>)\n\n")
}

// cmdReset: pone en cero el contador de GB de una cuenta de V2Ray (al renovar)
func cmdReset(usuario string, confirmado bool) {
	dbAbrir(rutaTraficoDB())
	if dbTrafico == nil {
		fmt.Println("no hay base de tráfico")
		return
	}
	var rx, tx int64
	var n int
	dbMu.Lock()
	dbTrafico.QueryRow("SELECT COALESCE(SUM(rx_bytes),0), COALESCE(SUM(tx_bytes),0), COUNT(*) FROM v2ray_user_traffic WHERE username=?;", usuario).Scan(&rx, &tx, &n)
	dbMu.Unlock()
	if n == 0 {
		fmt.Printf("\nℹ️  '%s' no tiene consumo anotado en V2Ray (nada que borrar)\n\n", usuario)
		return
	}
	fmt.Printf("\n🧹 Cuenta V2Ray '%s': %d filas · bajada %s · subida %s (total %s)\n", usuario, n, fmtBytes(rx), fmtBytes(tx), fmtBytes(rx+tx))
	if !confirmado {
		fmt.Printf("   (para borrarlo de verdad, agregá --confirmo)\n\n")
		return
	}
	brx, btx, bn := borrarConsumoV2Ray(usuario)
	fmt.Printf("   ✅ borrado: %d filas · %s liberados. La cuenta vuelve a contar de cero.\n\n", bn, fmtBytes(brx+btx))
}

func cmdTop() {
	if dbTrafico == nil {
		fmt.Println("no hay base de tráfico")
		return
	}
	fmt.Printf("\n📊 TRÁFICO POR PROTOCOLO — %s\n", rutaTraficoDB())
	rows, err := dbTrafico.Query("SELECT protocolo, COUNT(*), SUM(rx_bytes), SUM(tx_bytes), COUNT(DISTINCT src_ip) FROM trafico GROUP BY protocolo ORDER BY SUM(rx_bytes+tx_bytes) DESC;")
	if err == nil {
		fmt.Printf("%-14s %8s %12s %12s %6s\n", "PROTOCOLO", "SESIONES", "BAJADA", "SUBIDA", "IPs")
		for rows.Next() {
			var proto string
			var ses, ips int
			var rx, tx sql.NullInt64
			if err := rows.Scan(&proto, &ses, &rx, &tx, &ips); err == nil {
				fmt.Printf("%-14s %8d %12s %12s %6d\n", proto, ses, fmtBytes(rx.Int64), fmtBytes(tx.Int64), ips)
			}
		}
		rows.Close()
	}
	fmt.Println("\n🏆 TOP 10 IPs (bajado + subido)")
	rows2, err := dbTrafico.Query("SELECT src_ip, SUM(rx_bytes+tx_bytes) AS t, SUM(conexiones) FROM trafico GROUP BY src_ip ORDER BY t DESC LIMIT 10;")
	if err == nil {
		fmt.Printf("%-26s %12s %8s\n", "IP", "TOTAL", "CONEX")
		for rows2.Next() {
			var ip string
			var t, cx sql.NullInt64
			if err := rows2.Scan(&ip, &t, &cx); err == nil {
				fmt.Printf("%-26s %9s %8d\n", ip, fmtBytes(t.Int64), cx.Int64)
			}
		}
		rows2.Close()
	}
	fmt.Println()
}

func ayuda() {
	fmt.Printf("\n%s\n\n", Firma)
	fmt.Println("USO")
	fmt.Println("  ghostprox                    arranca el proxy (el :80 en producción)")
	fmt.Println("  ghostprox cuentas            cuentas sshgo + cuentas V2Ray (vence, GB, límite)")
	fmt.Println("  ghostprox cuentas ssh        solo las de SSH")
	fmt.Println("  ghostprox cuentas v2ray      solo las de V2Ray (con lo consumido por cuenta)")
	fmt.Println("  ghostprox limite <usuario> [GB]   cuánto gastó / ponerle el tope de GB (0 = sin tope)")
	fmt.Println("  ghostprox top                tráfico por protocolo y top 10 de IPs")
	fmt.Println("  ghostprox reset <usuario>    cuánto consumió esa cuenta de V2Ray (no borra nada)")
	fmt.Println("  ghostprox reset <usuario> --confirmo   lo pone en cero (al renovar la cuenta)")
	fmt.Println("  ghostprox --version          la firma en una línea")
	fmt.Println("  ghostprox --firma            el cartel completo")
	fmt.Println("")
	fmt.Println("QUÉ MIRA (v1.2)")
	fmt.Printf("  config : %s\n", RutaConfigJSON)
	fmt.Println("           quota_gb / quota_dias            → cuota POR IP (0 = sin límite)")
	fmt.Println("           v2ray_quota_gb / v2ray_quota_dias → cuota por CUENTA en V2Ray (0 = sin límite)")
	fmt.Println("           v2ray_check_users                → corta cuentas V2Ray vencidas/bajas/pasadas")
	fmt.Println("           max_connections_per_user         → conexiones por IP")
	fmt.Printf("  cuentas: %s  ·  %s\n", RutaUsuarios, RutaV2RayUsers)
	fmt.Printf("  tráfico: %s  (trafico · ssh_user_traffic · v2ray_user_traffic)\n", RutaTrafico)
	fmt.Println("")
	fmt.Println("  SIGHUP → recarga la config (cuotas y límites) sin cortar a nadie.")
	fmt.Println("  El límite de cada cuenta de V2Ray sale de su columna limit_gb; si es 0,")
	fmt.Println("  vale el v2ray_quota_gb general. Sin ninguno de los dos → sin cuota.")
	fmt.Println("")
	fmt.Println("PRUEBA EN OTRO PUERTO (sin tocar el :80)")
	fmt.Println("  LISTEN=:8888 DEBUG=1 GHOST_TRAFICO_DB=/tmp/trafico-prueba.db ./ghostprox")
	fmt.Println("")
	fmt.Println("VUELTA ATRÁS")
	fmt.Println("  bash /root/ghost-go/volver-al-pymanager.sh")
	fmt.Println()
}

// ─── MAIN ────────────────────────────────────────────────────────────────────

func main() {
	args := os.Args[1:]
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--version", "-v":
			fmt.Println(Firma)
			return
		case "--firma":
			mostrarFirma()
			return
		case "--ayuda", "-h", "--help", "ayuda":
			ayuda()
			return
		case "cuentas", "usuarios", "users":
			filtro := ""
			if i+1 < len(args) {
				switch args[i+1] {
				case "ssh":
					filtro = "ssh"
				case "v2ray", "v2":
					filtro = "v2ray"
				}
			}
			dbAbrir(rutaTraficoDB())
			prepararV2RayUsers()
			cmdCuentas(filtro)
			return
		case "top", "trafico", "tráfico":
			dbAbrir(rutaTraficoDB())
			cmdTop()
			return
		case "reset", "reseteo", "renovar":
			usuario := ""
			confirmado := false
			for _, x := range args[i+1:] {
				if x == "--confirmo" || x == "--si" || x == "-y" {
					confirmado = true
				} else if usuario == "" {
					usuario = x
				}
			}
			if usuario == "" {
				fmt.Println("uso: ghostprox reset <usuario> [--confirmo]   (pone en cero el contador de V2Ray)")
				return
			}
			cmdReset(usuario, confirmado)
			return
		case "limite", "límite", "cuota":
			usuario := ""
			var gb float64
			tieneValor := false
			for _, x := range args[i+1:] {
				if v, err := strconv.ParseFloat(strings.ReplaceAll(x, ",", "."), 64); err == nil && usuario != "" {
					gb = v
					tieneValor = true
				} else if usuario == "" && !strings.HasPrefix(x, "-") {
					usuario = x
				}
			}
			if usuario == "" {
				fmt.Println("uso: ghostprox limite <usuario> [GB]   (sin GB solo muestra cómo está; 0 = sin límite)")
				return
			}
			cmdLimite(usuario, gb, tieneValor)
			return
		}
	}

	if os.Getenv("FIRMA") != "0" {
		mostrarFirma()
	}
	fmt.Printf("[GO] %s\n", Firma)

	cfg := cargarConfig()
	dbAbrir(cfg.DBTrafico)
	prepararV2RayUsers()
	if dbTrafico == nil {
		fmt.Printf("[GO] ⚠️  sin base de tráfico: no cuento GB ni corto por cuota\n")
	} else {
		fmt.Printf("[GO] 🧮 contando tráfico en %s\n", cfg.DBTrafico)
	}
	if cfg.QuotaGB > 0 {
		fmt.Printf("[GO] 📏 cuota por IP: %.1f GB cada %d días\n", cfg.QuotaGB, cfg.QuotaDias)
	} else {
		fmt.Printf("[GO] 📏 cuota por IP: SIN LÍMITE (quota_gb=0)\n")
	}
	if cfg.V2rayCheck {
		if cfg.V2rayQuotaGB > 0 {
			fmt.Printf("[GO] 📡 cuota por CUENTA V2Ray: %.1f GB cada %d días (el limit_gb de cada cuenta manda)\n", cfg.V2rayQuotaGB, cfg.V2rayQuotaDias)
		} else {
			fmt.Printf("[GO] 📡 cuentas V2Ray: corta vencidas/bajas (sin cuota de GB: v2ray_quota_gb=0)\n")
		}
	} else {
		fmt.Printf("[GO] 📡 cuentas V2Ray: SIN control (v2ray_check_users=false)\n")
	}
	if cuentas := leerCuentas(); len(cuentas) > 0 {
		act := 0
		for _, c := range cuentas {
			if c.utilizable() {
				act++
			}
		}
		fmt.Printf("[GO] 👥 cuentas sshgo: %d activas de %d\n", act, len(cuentas))
	}
	if v2 := leerCuentasV2Ray(); len(v2) > 0 {
		act := 0
		for _, e := range v2 {
			if e.Activo == 1 && (e.SinVence || e.Dias >= 0) {
				act++
			}
		}
		fmt.Printf("[GO] 📡 cuentas V2Ray: %d activas de %d\n", act, len(v2))
	}

	// SIGHUP = recargar config (cuota, límites) sin cortar a nadie
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGHUP)
	go func() {
		for range sig {
			nueva := cargarConfig()
			cfg.QuotaGB = nueva.QuotaGB
			cfg.QuotaDias = nueva.QuotaDias
			cfg.MaxConn = nueva.MaxConn
			cfg.V2rayCheck = nueva.V2rayCheck
			cfg.V2rayQuotaGB = nueva.V2rayQuotaGB
			cfg.V2rayQuotaDias = nueva.V2rayQuotaDias
			cfg.Debug = nueva.Debug
			consumoMu.Lock()
			consumoMem = map[string]consumoGuardado{}
			consumoMu.Unlock()
			// también olvido las cuentas cacheadas: así un cambio de límite
			// (ghostprox limite …) se aplica YA con el HUP
			v2rayMu.Lock()
			v2rayCache = map[string]v2rayEntrada{}
			v2rayMu.Unlock()
			fmt.Printf("[GO] 🔄 config recargada (cuota IP %.1fGB/%dd · cuota cuenta V2Ray %.1fGB/%dd · maxconn %d)\n",
				cfg.QuotaGB, cfg.QuotaDias, cfg.V2rayQuotaGB, cfg.V2rayQuotaDias, cfg.MaxConn)
		}
	}()

	ln, err := net.Listen("tcp", cfg.Listen)
	if err != nil {
		log.Fatalf("no pude escuchar en %s: %v", cfg.Listen, err)
	}
	fmt.Printf("[GO] ghostprox escuchando en %s\n[GO] backends: %s\n", cfg.Listen, cfg)
	fmt.Printf("[GO] (probar en el :8888 antes de tocar el :80 ✅)\n")
	for {
		c, err := ln.Accept()
		if err != nil {
			continue
		}
		go manejar(c, cfg)
	}
}

var _ = binary.BigEndian // (por si después hace falta)
