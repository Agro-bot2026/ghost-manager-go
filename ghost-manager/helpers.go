// ghost-manager en Go — helpers: procesos con stdin, descargas, aleatorios
package main

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"
)

// execChpasswd: cambia la clave de un usuario del sistema (sin exponerla en el comando)
func execChpasswd(usuario, clave string) *exec.Cmd {
	cmd := exec.Command("chpasswd")
	cmd.Stdin = strings.NewReader(usuario + ":" + clave + "\n")
	return cmd
}

// execCrontab: reemplaza el crontab del usuario actual
func execCrontab(contenido string) *exec.Cmd {
	cmd := exec.Command("crontab", "-")
	cmd.Stdin = strings.NewReader(contenido)
	return cmd
}

// generarHex: n bytes al azar en hex (el bash usa openssl rand -hex N)
func generarHex(nBytes int) string {
	b := make([]byte, nBytes)
	rand.Read(b)
	return hex.EncodeToString(b)
}

// descargar: baja una URL a un archivo (como el curl -fsSL del bash)
func descargar(url, destino string) bool {
	cliente := &http.Client{Timeout: 120 * time.Second}
	resp, err := cliente.Get(url)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return false
	}
	tmp := destino + ".tmp"
	f, err := os.Create(tmp)
	if err != nil {
		return false
	}
	n, err := io.Copy(f, resp.Body)
	f.Close()
	if err != nil || n == 0 {
		os.Remove(tmp)
		return false
	}
	return os.Rename(tmp, destino) == nil
}

// descargarTexto: baja una URL y devuelve el texto (para los instaladores)
func descargarTexto(url string) string {
	cliente := &http.Client{Timeout: 60 * time.Second}
	resp, err := cliente.Get(url)
	if err != nil {
		return ""
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return ""
	}
	datos, err := io.ReadAll(resp.Body)
	if err != nil {
		return ""
	}
	return string(datos)
}

// sh: corre un comando de shell (para las recetas que el bash hacía con bash -c)
func sh(guion string) string {
	out, _ := exec.Command("bash", "-c", guion).CombinedOutput()
	return strings.TrimSpace(string(out))
}

func logLinea(formato string, args ...interface{}) {
	fmt.Printf("  "+formato+"\n", args...)
}
