# 🦇 GHOST-MANAGER v29.0-go "ESPELHO" — cómo se usa

## Entrás igual que siempre
```
ghost-manager
```
Mismo menú, mismas 30 opciones + 2 nuevas (31 y 32). Todo en Go.

## Lo nuevo respecto al bash de siempre
```
31) 📏 Ver/poner el límite de GB de una cuenta      (por cuenta de V2Ray y/o SSH)
32) 🧹 Resetear el contador de GB (cuando renovás)
 6) 👥 Listar usuarios   → ahora muestra CONSUMIDO y LÍMITE reales de cada cuenta
19) 📡 Conexiones en vivo → lee el log del PORTERO (ghostprox) y muestra el PAÍS
20) 📊 Consumo/cuota      → la cuota se aplica al toque (SIGHUP, sin cortar a nadie)
21) 📊 Consumo por CUENTA → los GB reales de cada cuenta (antes no existía)
 1) 📊 Estado            → el :80 ahora dice "Portero WS activo (ghostprox)" de verdad
```

## Comandos directos (sin entrar al menú)
```
ghost-manager estado      estado del sistema
ghost-manager listar      usuarios SSH y V2Ray
ghost-manager consumo     consumo por cuenta + top IPs
ghost-manager --version   la firma
ghost-manager --firma     el cartel
ghost-manager --ayuda     la ayuda
```

## Si algo no te gusta: volver al bash (un comando)
```
bash /root/ghost-go/volver-al-menu-bash.sh
```
El bash original quedó intacto en:  /usr/local/bin/ghost-manager.bash
(md5 del original: c9c8a470b613a76cd7e5f98a24340a27 · 114.727 bytes)

## Pruebas (no tocan nada: usan bases de mentira en /tmp)
```
export GM_BIN=/root/ghost-go/ghost-manager-go
bash /root/ghost-go/gm/prueba-gm.sh     # 42 pruebas (menú, usuarios, consumos)
bash /root/ghost-go/gm/prueba-gm2.sh    # 42 pruebas (accesos, instaladores, extras)
```

## Archivos
```
/root/ghost-go/ghost-manager-go        el binario Go (el que usa el comando)
/root/ghost-go/gm/                     el código fuente + pruebas + scripts
/root/ghost-go/PLAN-port-ghost-manager-a-go.md   el plan y las diferencias
/root/ghost-go/docs/spec-instaladores.md         spec sacada del bash
/root/ghost-go/ghost-manager-teramont  copia de referencia del bash original
/root/ghost-go/volver-al-menu-bash.sh  vuelta atrás
```
