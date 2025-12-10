# Bot de Discord para Dota 2

Bot de Discord que monitorea partidas de Dota 2 de usuarios registrados y envía notificaciones automáticas cuando se detectan nuevas partidas. Utiliza la API de OpenDota para obtener información de partidas y jugadores.

## Características

- 🔍 Búsqueda de jugadores por nombre de Steam
- 📝 Registro de usuarios de Discord con sus cuentas de Dota 2
- 📊 Estadísticas de partidas (últimas 20 partidas)
- 🏆 Ranking de jugadores ordenado por MMR y rango
- 🔔 Notificaciones automáticas de nuevas partidas
- 📈 Cálculo de rachas de victorias/derrotas
- 🎮 Información detallada de partidas (K/D/A, GPM/XPM, daño, etc.)

## Requisitos

- Go 1.21 o superior
- Token de bot de Discord
- Cuenta de Steam con perfil público (para obtener datos de Dota 2)

## Instalación

### 1. Clonar el repositorio

```bash
git clone <url-del-repositorio>
cd discord
```

### 2. Instalar dependencias

```bash
go mod download
```

### 3. Configurar variables de entorno

Copia el archivo `.env.example` a `.env` y configura las variables:

```bash
cp .env.example .env
```

Edita `.env` con tus valores:

```env
DISCORD_TOKEN=tu_token_de_discord
NOTIFICATION_CHANNEL_ID=id_del_canal
SERVER_ID=id_del_servidor
REFRESH_RATE=10
DEBUG=false
```

### 4. Compilar y ejecutar

```bash
go build -o dota-discord-bot .
./dota-discord-bot
```

O en modo debug:

```bash
./dota-discord-bot --debug
```

## Configuración

### Variables de entorno

- `DISCORD_TOKEN`: Token del bot de Discord (requerido)
- `NOTIFICATION_CHANNEL_ID`: ID del canal donde se enviarán las notificaciones (opcional, se puede configurar con `/dota channel`)
- `SERVER_ID`: ID del servidor de Discord (opcional, acelera el registro de comandos slash)
- `REFRESH_RATE`: Frecuencia de verificación de nuevas partidas en minutos (por defecto: 10)
- `DEBUG`: Activar logs en consola (por defecto: false)

### Crear un bot de Discord

1. Ve a [Discord Developer Portal](https://discord.com/developers/applications)
2. Haz clic en "New Application" y dale un nombre
3. Ve a la sección "Bot" en el menú lateral
4. Haz clic en "Add Bot" y confirma
5. En "Token", haz clic en "Reset Token" o "Copy" para obtener tu token
6. Guarda el token en tu archivo `.env` como `DISCORD_TOKEN`

### Invitar el bot al servidor

1. En Discord Developer Portal, ve a "OAuth2" > "URL Generator"
2. Selecciona los scopes:
   - `bot`
   - `applications.commands` (para comandos slash)
3. Selecciona los permisos del bot:
   - `Send Messages`
   - `Embed Links`
   - `Read Message History`
   - `Use Slash Commands`
4. Copia la URL generada y ábrela en tu navegador
5. Selecciona el servidor donde quieres agregar el bot

### Activar Modo Desarrollador en Discord

Para obtener IDs de canales, usuarios y servidores:

1. Abre Discord (aplicación de escritorio o web)
2. Ve a Configuración de Usuario (engranaje)
3. Ve a "Avanzado"
4. Activa "Modo Desarrollador"
5. Ahora puedes hacer clic derecho en cualquier canal, usuario o servidor y seleccionar "Copiar ID"

### Obtener IDs necesarios

- **Channel ID**: Clic derecho en el canal > "Copiar ID"
- **Server ID (Guild ID)**: Clic derecho en el nombre del servidor > "Copiar ID"
- **User ID**: Clic derecho en el usuario > "Copiar ID" (requiere Modo Desarrollador)

## Uso con Docker

### Construir la imagen

```bash
docker build -t dota-discord-bot .
```

### Ejecutar con docker-compose

```bash
docker-compose up -d
```

El archivo `docker-compose.yml` incluye:
- Volumen persistente para `data/` (usuarios y partidas)
- Volumen para `logs/`
- Variables de entorno desde `.env`
- Valores por defecto: `REFRESH_RATE=10`, `DEBUG=false`
- Puerto 8080 expuesto (aunque el bot no lo use directamente)

### Desplegar en servidor remoto

El script `deploy` construye la imagen, la sube al registry y la despliega en el servidor:

```bash
./deploy
```

**Nota**: Asegúrate de configurar:
- Acceso al registry `orgmcr.or-gm.com`
- SSH configurado para el servidor `server`
- Ruta correcta del archivo `.env` en el servidor
- Rutas de volúmenes correctas en el script `deploy`

## Comandos

Todos los comandos usan la sintaxis de Slash Commands de Discord:

### `/dota search nombre:<nombre>`

Busca jugadores de Dota 2 por nombre de Steam.

**Ejemplo:**
```
/dota search nombre:Desp4irs
```

### `/dota register account_id:<id> [usuario:@amigo]`

Registra una cuenta de Dota 2 con un usuario de Discord.

- Si omites `usuario`, te registras a ti mismo
- Puedes usar el número de resultado de una búsqueda (1-10) o el ID directo
- Puedes registrar a otros usuarios usando `usuario:@amigo`

**Ejemplos:**
```
/dota register account_id:136201811
/dota register account_id:1
/dota register account_id:136201811 usuario:@amigo
```

### `/dota stats [usuario:@usuario]`

Muestra estadísticas de las últimas 20 partidas del usuario registrado.

- Incluye victorias/derrotas, racha actual, y detalles de la última partida
- Si no especificas usuario, muestra tus propias estadísticas

**Ejemplos:**
```
/dota stats
/dota stats usuario:@amigo
```

### `/dota rank`

Muestra el ranking de todos los jugadores registrados, ordenados por MMR y rango (del menor al mayor).

- Incluye imagen de avatar de cada jugador
- Muestra posición, nombre, MMR y rango
- Los primeros 3 lugares tienen medallas especiales 🥇🥈🥉

**Ejemplo:**
```
/dota rank
```

### `/dota channel canal:<#canal>`

Configura el canal donde se enviarán las notificaciones automáticas de nuevas partidas.

**Ejemplo:**
```
/dota channel canal:#dota-updates
```

### `/dota help`

Muestra la ayuda con todos los comandos disponibles.

## Estructura de datos

El bot guarda información en archivos JSON en la carpeta `data/`:

- `data/users.json`: Mapeo de Discord ID a Dota 2 Account ID
- `data/last_matches.json`: Última partida conocida por cada usuario
- `data/notification_channel.json`: Canal configurado para notificaciones

## Notificaciones automáticas

El bot verifica periódicamente (cada `REFRESH_RATE` minutos) si hay nuevas partidas para los usuarios registrados. Cuando detecta una nueva partida, envía una notificación al canal configurado con:

- Resultado de la partida (Victoria/Derrota)
- Héroe jugado
- K/D/A y KDA ratio
- Duración de la partida
- Modo de juego y tipo de lobby
- Score (Radiant vs Dire)
- Nivel alcanzado
- GPM/XPM
- Daño a héroes/torres
- Curación
- Racha actual
- Link a Dotabuff

## Logs

Los logs se guardan en `logs/bot.log` por defecto. En modo debug (`DEBUG=true` o `--debug`), los logs también se muestran en consola.

## Solución de problemas

### El bot no responde a comandos

1. Verifica que el bot esté en línea en Discord
2. Asegúrate de que los comandos slash estén registrados (puede tardar hasta 1 hora si no configuraste `SERVER_ID`)
3. Verifica que el bot tenga permisos para usar comandos slash en el canal

### No se detectan nuevas partidas

1. Verifica que el usuario esté registrado con `/dota stats`
2. Asegúrate de que el perfil de Steam sea público
3. Revisa los logs para ver si hay errores de API

### Error al obtener datos de OpenDota

- La API de OpenDota tiene límites de rate. El bot espera 1 segundo entre requests
- Si el perfil no está actualizado en OpenDota, puede que no aparezcan partidas recientes
- Algunos jugadores pueden tener el perfil privado

## Desarrollo

### Estructura del proyecto

```
discord/
├── config/          # Configuración y carga de variables de entorno
├── discord/         # Lógica del bot de Discord
├── dota/            # Cliente de API de OpenDota
├── storage/         # Persistencia de datos (JSON)
├── data/            # Datos persistentes (usuarios, matches)
├── logs/            # Archivos de log
├── main.go          # Punto de entrada
└── go.mod           # Dependencias
```

### Agregar nuevas funcionalidades

1. Agrega el comando en `discord/bot.go` en `registerCommands()`
2. Implementa el handler correspondiente
3. Agrega el caso en `interactionCreate()`
4. Actualiza la documentación en este README

## Licencia

[Especificar licencia si aplica]

## Créditos

- API de OpenDota: https://www.opendota.com/
- Discord Go library: https://github.com/bwmarrin/discordgo

