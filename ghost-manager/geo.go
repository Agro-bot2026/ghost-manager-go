// ghost-manager en Go — base de países (GeoIP) para las conexiones en vivo
package main

import (
	"bufio"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
)

func geoDBPath() string { return env0("GM_GEO_DB", "/etc/ghost-monitor/geo_countries.db") }

// cache de países ya consultados (para no tocar la base en cada línea)
var geoCache = map[string]string{}

var geoOnce sync.Once

// asegurarGeoDB: baja la base de países una sola vez (igual que el bash)
func asegurarGeoDB() {
	geoOnce.Do(func() {
		if existeArchivo(geoDBPath()) {
			return
		}
		if noTocarSistema() {
			fmt.Printf("  %s(prueba: no bajo la base de países)%s\n", gris, nc)
			return
		}
		fmt.Printf("  %s  🌍 Descargando base de países (1 vez, ~3MB)...%s\n", cian, nc)
		os.MkdirAll("/etc/ghost-monitor", 0755)
		cliente := &http.Client{}
		resp, err := cliente.Get("https://raw.githubusercontent.com/sapics/ip-location-db/main/dbip-country/dbip-country-ipv4.csv")
		if err != nil || resp.StatusCode != 200 {
			aviso("no pude bajar la base de países (las conexiones se ven sin país)")
			return
		}
		defer resp.Body.Close()
		db, err := abrirDB(geoDBPath())
		if err != nil {
			return
		}
		defer db.Close()
		db.Exec("DROP TABLE IF EXISTS geo;")
		db.Exec("CREATE TABLE geo (start INTEGER, end INTEGER, cc TEXT);")
		tx, _ := db.Begin()
		stmt, err := tx.Prepare("INSERT INTO geo VALUES (?,?,?);")
		if err != nil {
			aviso("no pude preparar la base de países")
			return
		}
		sc := bufio.NewScanner(resp.Body)
		sc.Buffer(make([]byte, 1024*1024), 1024*1024)
		n := 0
		for sc.Scan() {
			p := strings.Split(strings.TrimSpace(sc.Text()), ",")
			if len(p) != 3 {
				continue
			}
			a, ok1 := ipAEntero(p[0])
			b, ok2 := ipAEntero(p[1])
			if !ok1 || !ok2 {
				continue
			}
			stmt.Exec(a, b, p[2])
			n++
		}
		stmt.Close()
		tx.Commit()
		os.Chmod(geoDBPath(), 0644)
		oks(fmt.Sprintf("base de países lista (%d rangos)", n))
	})
}

func ipAEntero(ip string) (uint32, bool) {
	partes := strings.Split(ip, ".")
	if len(partes) != 4 {
		return 0, false
	}
	var n uint32
	for _, p := range partes {
		v, err := strconv.Atoi(p)
		if err != nil || v < 0 || v > 255 {
			return 0, false
		}
		n = n<<8 | uint32(v)
	}
	return n, true
}

// paisDe: código de país de una IP (o "" si no se sabe)
func paisDe(ip string) string {
	ip = strings.TrimPrefix(ip, "::ffff:")
	if v, ok := geoCache[ip]; ok {
		return v
	}
	res := ""
	if existeArchivo(geoDBPath()) {
		if n, ok := ipAEntero(ip); ok {
			if db, err := abrirDB(geoDBPath()); err == nil {
				var cc string
				db.QueryRow("SELECT cc FROM geo WHERE start<=? AND end>=? ORDER BY start DESC LIMIT 1;", n, n).Scan(&cc)
				db.Close()
				if cc != "" {
					res = banderaPais(cc)
				}
			}
		}
	}
	geoCache[ip] = res
	return res
}

// banderaPais: código ISO → banderita + nombre (los de la región primero)
func banderaPais(cc string) string {
	nombres := map[string]string{
		"AR": "🇦🇷 Argentina", "BR": "🇧🇷 Brasil", "CL": "🇨🇱 Chile", "CO": "🇨🇴 Colombia",
		"MX": "🇲🇽 México", "PE": "🇵🇪 Perú", "UY": "🇺🇾 Uruguay", "PY": "🇵🇾 Paraguay",
		"BO": "🇧🇴 Bolivia", "EC": "🇪🇨 Ecuador", "VE": "🇻🇪 Venezuela", "US": "🇺🇸 EE.UU.",
		"ES": "🇪🇸 España", "CN": "🇨🇳 China", "RU": "🇷🇺 Rusia", "DE": "🇩🇪 Alemania",
		"FR": "🇫🇷 Francia", "IT": "🇮🇹 Italia", "GB": "🇬🇧 Reino Unido", "NL": "🇳🇱 Países Bajos",
		"CA": "🇨🇦 Canadá", "IN": "🇮🇳 India", "ID": "🇮🇩 Indonesia", "TR": "🇹🇷 Turquía",
		"SE": "🇸🇪 Suecia", "SG": "🇸🇬 Singapur", "JP": "🇯🇵 Japón", "KR": "🇰🇷 Corea",
		"PL": "🇵🇱 Polonia", "UA": "🇺🇦 Ucrania", "ZA": "🇿🇦 Sudáfrica", "AU": "🇦🇺 Australia",
		"PT": "🇵🇹 Portugal", "RO": "🇷🇴 Rumania", "VN": "🇻🇳 Vietnam", "IR": "🇮🇷 Irán",
	}
	if v, ok := nombres[cc]; ok {
		return v
	}
	if cc == "??" || cc == "" {
		return ""
	}
	return "🌍 " + cc
}
