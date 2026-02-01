# PRD: Notificaciones de Partidas Mejoradas con Miniaturas de Héroes

## Introducción

Mejorar las notificaciones automáticas de partidas del bot de Discord para Dota 2, agregando el record del héroe en las últimas 20 partidas (actualmente solo disponible en `/dota stats`), implementando enlaces a Stratz para partidas y jugadores, y mostrando miniaturas de los héroes utilizados. También se unificará el formato entre las notificaciones automáticas y el comando `/dota stats`.

## Goals

- Agregar el record del héroe (W/L últimas 20 partidas) a las notificaciones automáticas
- Unificar el formato de `/dota stats` con las notificaciones automáticas
- Usar enlaces de Stratz (https://stratz.com/matches/[ID] y https://stratz.com/players/[ACCOUNT_ID])
- Omitir enlaces para jugadores con 0/0 W/L (mostrar solo nombre)
- Descargar y almacenar miniaturas de héroes desde dota2.com
- Mostrar miniaturas de héroes en los mensajes de notificación

## User Stories

### US-001: Crear script de descarga de miniaturas de héroes
**Description:** Como desarrollador, necesito un script que descargue todas las miniaturas de héroes desde dota2.com para usarlas en los mensajes del bot.

**Acceptance Criteria:**
- [ ] Script que hace scraping de https://www.dota2.com/heroes para obtener lista de héroes
- [ ] Descarga la imagen de miniatura de cada héroe desde https://www.dota2.com/hero/[hero_name]
- [ ] Guarda las imágenes en `assets/heroes/` con formato `[hero_name].png`
- [ ] Crea un archivo JSON de mapeo `assets/heroes/hero-images.json` con {hero_id: filename}
- [ ] Maneja errores de descarga gracefully (log y continúa)
- [ ] Typecheck/lint passes

### US-002: Crear servicio de imágenes de héroes
**Description:** Como desarrollador, necesito un servicio que proporcione las rutas de las miniaturas de héroes para usarlas en los embeds de Discord.

**Acceptance Criteria:**
- [ ] Servicio que lee el mapeo de `hero-images.json`
- [ ] Método `getHeroImagePath(heroId: number): string | null`
- [ ] Método `getHeroImagePath(heroName: string): string | null` (por nombre)
- [ ] Retorna `null` si la imagen no existe
- [ ] Typecheck/lint passes

### US-003: Agregar record del héroe a notificaciones automáticas
**Description:** Como usuario, quiero ver mi record con el héroe usado (W/L en últimas 20 partidas) en las notificaciones automáticas de partidas.

**Acceptance Criteria:**
- [ ] La notificación automática muestra "Record con [Héroe]: X-Y (últimas 20)"
- [ ] Usa la misma lógica de `getHeroWinLoss` que usa `/dota stats`
- [ ] El formato es idéntico al mostrado por `/dota stats`
- [ ] Si no hay datos disponibles, muestra "0-0"
- [ ] Typecheck/lint passes

### US-004: Cambiar URLs a Stratz para partidas
**Description:** Como usuario, quiero que los enlaces de las partidas apunten a Stratz para ver detalles más completos.

**Acceptance Criteria:**
- [ ] El enlace de la partida usa formato `https://stratz.com/matches/[MATCH_ID]`
- [ ] El título del embed es clickeable y lleva a Stratz
- [ ] Aplica tanto a notificaciones automáticas como a `/dota stats`
- [ ] Typecheck/lint passes

### US-005: Cambiar URLs a Stratz para jugadores públicos
**Description:** Como usuario, quiero que los enlaces de jugadores en la lista de la partida apunten a sus perfiles de Stratz.

**Acceptance Criteria:**
- [ ] Jugadores con W/L > 0/0 tienen enlace `https://stratz.com/players/[ACCOUNT_ID]`
- [ ] Jugadores con 0/0 W/L muestran solo el nombre del héroe y nombre del jugador (sin enlace)
- [ ] El formato es: `[Héroe] - [Nombre Jugador](link)` para jugadores con datos
- [ ] El formato es: `[Héroe] - [Nombre Jugador]` para jugadores sin datos (0/0)
- [ ] Typecheck/lint passes

### US-006: Agregar miniatura del héroe principal al embed
**Description:** Como usuario, quiero ver la miniatura del héroe que usé en la partida dentro del mensaje de notificación.

**Acceptance Criteria:**
- [ ] El embed muestra la miniatura del héroe como thumbnail
- [ ] Usa `setThumbnail()` de Discord.js con la imagen local o URL
- [ ] Si la imagen no existe, no muestra thumbnail (no rompe el mensaje)
- [ ] Aplica tanto a notificaciones automáticas como a `/dota stats`
- [ ] Typecheck/lint passes

### US-007: Mostrar miniaturas de héroes en lista de jugadores
**Description:** Como usuario, quiero ver las miniaturas de los héroes de los demás jugadores en la lista de la partida.

**Acceptance Criteria:**
- [ ] Cada jugador en la lista muestra el emoji/icono del héroe (si Discord lo permite) o el nombre del héroe
- [ ] Si no es posible mostrar imágenes inline, usar el nombre del héroe con formato destacado
- [ ] La lista mantiene legibilidad y no se vuelve muy larga
- [ ] Typecheck/lint passes

### US-008: Unificar formato de /dota stats con notificaciones
**Description:** Como usuario, quiero que `/dota stats` muestre exactamente el mismo formato que las notificaciones automáticas.

**Acceptance Criteria:**
- [ ] `/dota stats` usa el mismo builder/formatter que las notificaciones automáticas
- [ ] Ambos muestran: resultado, héroe, KDA, duración, GPM/XPM, record del héroe
- [ ] Ambos usan URLs de Stratz
- [ ] Ambos muestran la miniatura del héroe
- [ ] Eliminar código duplicado entre ambas funcionalidades
- [ ] Typecheck/lint passes

## Functional Requirements

- FR-1: Crear directorio `assets/heroes/` para almacenar miniaturas de héroes
- FR-2: Script de scraping debe obtener URLs de imágenes de https://www.dota2.com/hero/[name]
- FR-3: Las imágenes deben guardarse en formato PNG con nombres en kebab-case
- FR-4: El archivo de mapeo debe relacionar `hero_id` (número) con el archivo de imagen
- FR-5: Las notificaciones automáticas deben llamar a `getHeroWinLoss()` para obtener el record
- FR-6: El formato del record debe ser: "Record con [Héroe]: X-Y (últ. 20)"
- FR-7: URLs de partidas: `https://stratz.com/matches/[MATCH_ID]`
- FR-8: URLs de jugadores: `https://stratz.com/players/[ACCOUNT_ID]`
- FR-9: Jugadores con 0 wins Y 0 losses no deben tener enlace clickeable
- FR-10: El embed debe usar `setThumbnail()` para mostrar el héroe del jugador principal
- FR-11: Extraer la lógica de formato a una función compartida para eliminar duplicación

## Non-Goals

- No se implementará caché de imágenes en CDN externo
- No se mostrarán imágenes inline en la lista de jugadores (limitación de Discord embeds)
- No se descargará automáticamente nuevas imágenes cuando se agreguen héroes (proceso manual)
- No se eliminará el comando `/dota stats`, solo se unificará su formato
- No se cambiarán otros comandos del bot

## Technical Considerations

- Las imágenes de héroes pueden adjuntarse como archivos locales usando `AttachmentBuilder` de Discord.js
- Alternativa: hostear imágenes en un servicio externo y usar URLs directas
- El scraping de dota2.com puede requerir manejo de rate limiting
- El mapeo de nombres de héroes debe considerar variaciones (ej: "Nature's Prophet" vs "furion")
- OpenDota API ya proporciona `hero_id`, usar esto para el mapeo
- Considerar usar las URLs de imágenes de OpenDota/Stratz CDN como alternativa a descargar

## Success Metrics

- Las notificaciones automáticas muestran el record del héroe en 100% de los casos
- Los enlaces de Stratz funcionan correctamente
- No hay regresión en el tiempo de respuesta de las notificaciones
- El código duplicado entre `/dota stats` y notificaciones se reduce a 0

## Open Questions

1. ¿Usar imágenes locales descargadas o URLs de CDN externo (OpenDota/Stratz tienen CDN de imágenes)?
   - Pros locales: No depende de servicios externos, más rápido
   - Pros CDN: No requiere almacenamiento, siempre actualizado
2. ¿El script de descarga debe ejecutarse como parte del build o manualmente?
3. ¿Qué hacer si un héroe nuevo se agrega a Dota 2 y no tenemos la imagen?
