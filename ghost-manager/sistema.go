// ghost-manager en Go — sistema: comandos, servicios, estado, usuarios del sistema
package main

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

// ─── COMANDOS ───────────────────────────────────────────────────────────────
func correr(nombre string, args ...string) string {
	out, _ := exec.Command(nombre, args...).CombinedOutput()
	return strings.TrimSpace(string(out))
}

func correrOculto(nombre string, args ...string) error {
	return exec.Command(nombre, args...).Run()
}

func existeComando(nombre string) bool {
	_, err := exec.LookPath(nombre)
	return err == nil
}

func servicioActivo(svc string) bool {
	return correr("systemctl", "is-active", svc) == "active"
}

func existeArchivo(ruta string) bool {
	_, err := os.Stat(ruta)
	return err == nil
}

// ─── SERVICIOS (la misma lista del bash, mirando a ghostprox) ───────────────
type Servicio struct {
	Unidad string
	Nombre string
}

func listaServicios() []Servicio {
	return []Servicio{
		{"ghostprox", "PORTERO WS :80"},
		{"sshgo", "CUENTAS SSH :2200"},
		{"psiphon", "PSIPHON :2223"},
		{"xray", "XRAY :8443/:8388/:10001"},
		{"openvpn@server", "OPENVPN :1194"},
		{"wg-quick@wg0", "WIREGUARD"},
		{"brook", "BROOK :18999"},
		{"hcr-server", "HCR :8880"},
		{"hcr-server-8080", "HCR :8080"},
		{"bilola-server", "BILOLA"},
		{"udp-custom", "UDP CUSTOM"},
		{"dnstt", "DNSTT"},
		{"slowdns", "SLOWDNS"},
		{"stunnel-epro", "STUNNEL :443"},
		{"dropbear", "DROPBEAR :444"},
		{"squid", "SQUID :3128"},
	}
}

// ─── IP PÚBLICA Y DOMINIO ───────────────────────────────────────────────────
func ipPublica() string {
	for _, u := range []string{"https://ifconfig.me", "https://api.ipify.org"} {
		if out := correr("curl", "-4", "-s", "--max-time", "5", u); out != "" && strings.Count(out, ".") == 3 {
			return out
		}
	}
	out := correr("hostname", "-I")
	for _, campo := range strings.Fields(out) {
		if strings.Count(campo, ".") == 3 {
			return campo
		}
	}
	return "0.0.0.0"
}

func dominio() string {
	if datos, err := os.ReadFile(rutaDominioFile()); err == nil {
		d := strings.TrimSpace(string(datos))
		if d != "" {
			return d
		}
	}
	return ipPublica()
}

// ─── USUARIO DEL SISTEMA (lo que hace el bash con useradd/chpasswd/chage) ───
func crearUsuarioSistema(usuario, clave string, dias, maxConn int) {
	if noTocarSistema() {
		fmt.Printf("  %s(prueba: no toco usuarios del sistema)%s\n", gris, nc)
		return
	}
	correr("useradd", "-m", "-s", "/bin/false", usuario)
	cmd := exec.Command("chpasswd")
	cmd.Stdin = strings.NewReader(fmt.Sprintf("%s:%s\n", usuario, clave))
	cmd.Run()
	correr("chage", "-E", expDia(dias), usuario)
	// límites de sesiones (SSH) + PAM maxlogins
	os.MkdirAll("/etc/ssh/sshd_config.d", 0755)
	os.WriteFile("/etc/ssh/sshd_config.d/"+usuario+".conf", []byte(fmt.Sprintf("MaxSessions %d\n", maxConn)), 0644)
	correr("systemctl", "restart", "ssh")
	// PAM
	if datos, err := os.ReadFile("/etc/pam.d/common-session"); err == nil && !strings.Contains(string(datos), "pam_limits.so") {
		f, err := os.OpenFile("/etc/pam.d/common-session", os.O_APPEND|os.O_WRONLY, 0644)
		if err == nil {
			f.WriteString("session required pam_limits.so\n")
			f.Close()
		}
	}
	// limits.conf: sacar el viejo y poner el nuevo
	if datos, err := os.ReadFile("/etc/security/limits.conf"); err == nil {
		var nuevas []string
		for _, l := range strings.Split(string(datos), "\n") {
			if strings.HasPrefix(strings.TrimSpace(l), usuario+"\t") || strings.HasPrefix(strings.TrimSpace(l), usuario+" ") {
				continue
			}
			nuevas = append(nuevas, l)
		}
		nuevas = append(nuevas, fmt.Sprintf("%s\tsoft\tmaxlogins\t%d", usuario, maxConn), fmt.Sprintf("%s\thard\tmaxlogins\t%d", usuario, maxConn))
		os.WriteFile("/etc/security/limits.conf", []byte(strings.Join(nuevas, "\n")), 0644)
	}
}

func borrarUsuarioSistema(usuario string) bool {
	if noTocarSistema() {
		fmt.Printf("  %s(prueba: no toco usuarios del sistema)%s\n", gris, nc)
		return true
	}
	correr("pkill", "-u", usuario)
	err := correrOculto("userdel", "-f", usuario)
	os.Remove("/etc/ssh/sshd_config.d/" + usuario + ".conf")
	return err == nil
}

func existeUsuarioSistema(usuario string) bool {
	// ojo: hay que mirar el CÓDIGO DE SALIDA (si no, el mensaje de error
	// "no such user" que va por stderr hace creer que el usuario existe)
	return exec.Command("id", "-u", usuario).Run() == nil
}

func expDia(dias int) string {
	return time.Now().AddDate(0, 0, dias).Format("2006-01-02")
}

// ─── DATOS DEL SISTEMA (para el dashboard) ──────────────────────────────────
func memInfo() (totalMB, usadoMB int, pct int) {
	datos, err := os.ReadFile("/proc/meminfo")
	if err != nil {
		return 0, 0, 0
	}
	valores := map[string]int{}
	for _, l := range strings.Split(string(datos), "\n") {
		partes := strings.Fields(l)
		if len(partes) >= 2 {
			n, _ := strconv.Atoi(partes[1])
			valores[strings.TrimSuffix(partes[0], ":")] = n
		}
	}
	total := valores["MemTotal"] / 1024
	libre := valores["MemAvailable"] / 1024
	usado := total - libre
	if total > 0 {
		pct = usado * 100 / total
	}
	return total, usado, pct
}

func discoInfo() (totalGB, usadoGB int, pct int) {
	out := correr("df", "-P", "/")
	lineas := strings.Split(out, "\n")
	if len(lineas) < 2 {
		return 0, 0, 0
	}
	campos := strings.Fields(lineas[1])
	if len(campos) < 5 {
		return 0, 0, 0
	}
	totalKB, _ := strconv.Atoi(campos[1])
	usadoKB, _ := strconv.Atoi(campos[2])
	pct, _ = strconv.Atoi(strings.TrimSuffix(campos[4], "%"))
	return totalKB / 1024 / 1024, usadoKB / 1024 / 1024, pct
}

func cargaCPU() string {
	datos, err := os.ReadFile("/proc/loadavg")
	if err != nil {
		return "-"
	}
	partes := strings.Fields(string(datos))
	if len(partes) == 0 {
		return "-"
	}
	return partes[0]
}

func nombreOS() string {
	datos, err := os.ReadFile("/etc/os-release")
	if err != nil {
		return "Linux"
	}
	for _, l := range strings.Split(string(datos), "\n") {
		if strings.HasPrefix(l, "PRETTY_NAME=") {
			v := strings.Trim(strings.TrimPrefix(l, "PRETTY_NAME="), "\"")
			v = strings.Split(v, " (")[0]
			v = strings.TrimSuffix(v, " Linux")
			if len(v) > 14 {
				v = v[:14]
			}
			return v
		}
	}
	return "Linux"
}

// ─── PUERTOS (para los instaladores) ────────────────────────────────────────
func puertoEscuchando(puerto int) bool {
	out := correr("ss", "-ltn")
	return strings.Contains(out, fmt.Sprintf(":%d ", puerto)) || strings.Contains(out, fmt.Sprintf(":%d\n", puerto))
}

func abrirPuertoTCP(puerto int) {
	if correr("nft", "list", "ruleset") != "" && existeComando("nft") {
		correr("nft", "add", "rule", "inet", "filter", "input", "tcp", "dport", strconv.Itoa(puerto), "accept")
		return
	}
	correr("iptables", "-I", "INPUT", "-p", "tcp", "--dport", strconv.Itoa(puerto), "-j", "ACCEPT")
}

// ─── SERVICIOS: acciones ────────────────────────────────────────────────────
func reiniciarServicio(svc string) bool {
	correr("systemctl", "daemon-reload")
	correr("systemctl", "enable", svc)
	return correrOculto("systemctl", "restart", svc) == nil
}

func escribirUnidad(nombre, contenido string) error {
	ruta := "/etc/systemd/system/" + nombre + ".service"
	return os.WriteFile(ruta, []byte(contenido), 0644)
}
