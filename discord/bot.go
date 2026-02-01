package discord

import (
	"dota-discord-bot/config"
	"dota-discord-bot/dota"
	"dota-discord-bot/storage"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/bwmarrin/discordgo"
)

type Bot struct {
	session      *discordgo.Session
	dotaClient   *dota.Client
	stratzClient *dota.StratzClient
	userStore    *storage.UserStore
	config       *config.Config
	searchCache  map[string][]dota.SearchResponse // Cache temporal de búsquedas por usuario
	lastStatsDay string                           // fecha (2006-01-02) del último envío diario de stats
	statsMu      sync.Mutex                       // protege lastStatsDay
}

func NewBot(cfg *config.Config, dotaClient *dota.Client, stratzClient *dota.StratzClient, userStore *storage.UserStore) (*Bot, error) {
	if err := InitLogger(cfg.Debug); err != nil {
		return nil, fmt.Errorf("error inicializando logger: %w", err)
	}

	session, err := discordgo.New("Bot " + cfg.DiscordToken)
	if err != nil {
		return nil, fmt.Errorf("error creando sesión Discord: %w", err)
	}

	bot := &Bot{
		session:      session,
		dotaClient:   dotaClient,
		stratzClient: stratzClient,
		userStore:    userStore,
		config:       cfg,
		searchCache:  make(map[string][]dota.SearchResponse),
	}

	// Cambiar a interactionCreate para manejar slash commands
	session.AddHandler(bot.interactionCreate)
	// Para slash commands solo necesitamos intents básicos
	session.Identify.Intents = discordgo.IntentsGuilds

	return bot, nil
}

func (b *Bot) Start() error {
	getLogger().Info("Iniciando bot de Discord...")
	if err := b.session.Open(); err != nil {
		return fmt.Errorf("error abriendo conexión: %w", err)
	}
	getLogger().Info("Bot conectado exitosamente")

	// Registrar comandos slash
	if err := b.registerCommands(); err != nil {
		getLogger().Warnf("Error registrando comandos: %v", err)
	}

	return nil
}

func (b *Bot) registerCommands() error {
	getLogger().Info("Registrando comandos slash...")

	// OPCIÓN 1: Global (tarda hasta 1 hora en propagarse)
	// guildID := ""

	// OPCIÓN 2: Por guild específico (INSTANTÁNEO)
	guildID := b.config.ServerID
	if guildID == "" {
		getLogger().Warn("SERVER_ID no configurado en .env, los comandos se registrarán globalmente (puede tardar hasta 1 hora)")
	}

	// Obtener comandos existentes y eliminarlos primero para asegurar actualización
	existingCommands, err := b.session.ApplicationCommands(b.session.State.User.ID, guildID)
	if err == nil {
		for _, existingCmd := range existingCommands {
			if existingCmd.Name == "dota" {
				getLogger().Infof("Eliminando comando existente: /%s", existingCmd.Name)
				if err := b.session.ApplicationCommandDelete(b.session.State.User.ID, guildID, existingCmd.ID); err != nil {
					getLogger().Warnf("Error eliminando comando existente '%s': %v", existingCmd.Name, err)
				} else {
					getLogger().Infof("✅ Comando eliminado: /%s", existingCmd.Name)
				}
			}
		}
		// Esperar un momento para que Discord procese la eliminación
		time.Sleep(1 * time.Second)
	}

	commands := []*discordgo.ApplicationCommand{
		{
			Name:        "dota",
			Description: "Comandos del bot de Dota 2",
			Options: []*discordgo.ApplicationCommandOption{
				{
					Type:        discordgo.ApplicationCommandOptionSubCommand,
					Name:        "search",
					Description: "Buscar jugadores por nombre de Steam",
					Options: []*discordgo.ApplicationCommandOption{
						{
							Type:        discordgo.ApplicationCommandOptionString,
							Name:        "nombre",
							Description: "Nombre del jugador a buscar",
							Required:    true,
						},
					},
				},
				{
					Type:        discordgo.ApplicationCommandOptionSubCommand,
					Name:        "register",
					Description: "Registrar tu cuenta de Dota 2",
					Options: []*discordgo.ApplicationCommandOption{
						{
							Type:        discordgo.ApplicationCommandOptionString,
							Name:        "account_id",
							Description: "ID de Dota o número de resultado de búsqueda",
							Required:    true,
						},
						{
							Type:        discordgo.ApplicationCommandOptionUser,
							Name:        "usuario",
							Description: "Usuario de Discord a registrar (opcional, por defecto te registras tú)",
							Required:    false,
						},
					},
				},
				{
					Type:        discordgo.ApplicationCommandOptionSubCommand,
					Name:        "channel",
					Description: "Configurar canal de notificaciones",
					Options: []*discordgo.ApplicationCommandOption{
						{
							Type:        discordgo.ApplicationCommandOptionChannel,
							Name:        "canal",
							Description: "Canal para notificaciones",
							Required:    true,
						},
					},
				},
				{
					Type:        discordgo.ApplicationCommandOptionSubCommand,
					Name:        "stats",
					Description: "Estadísticas por héroe en el parche actual (W/L, % victorias)",
				},
				{
					Type:        discordgo.ApplicationCommandOptionSubCommand,
					Name:        "help",
					Description: "Mostrar ayuda",
				},
			},
		},
	}

	for _, cmd := range commands {
		_, err := b.session.ApplicationCommandCreate(b.session.State.User.ID, guildID, cmd)
		if err != nil {
			getLogger().Errorf("Error creando comando '%s': %v", cmd.Name, err)
		} else {
			getLogger().Infof("✅ Comando registrado: /%s", cmd.Name)
		}
	}

	return nil
}

func (b *Bot) Stop() {
	getLogger().Info("Cerrando bot...")
	b.session.Close()
}

func (b *Bot) interactionCreate(s *discordgo.Session, i *discordgo.InteractionCreate) {
	// Solo manejar comandos de aplicación (slash commands)
	if i.ApplicationCommandData().Name != "dota" {
		return
	}

	// Responder inmediatamente (Discord requiere respuesta en 3 segundos)
	err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
	})
	if err != nil {
		// 10062 = Unknown interaction (token expirado o interacción ya respondida, p. ej. evento duplicado)
		if strings.Contains(err.Error(), "10062") || strings.Contains(err.Error(), "Unknown interaction") {
			getLogger().Debugf("Interacción ya respondida o expirada, ignorando: %v", err)
		} else {
			getLogger().Errorf("Error respondiendo a interacción: %v", err)
		}
		return
	}

	// Obtener el subcomando
	if len(i.ApplicationCommandData().Options) == 0 {
		b.sendFollowup(s, i, "❌ Comando inválido. Usa `/dota help` para ver los comandos disponibles.")
		return
	}

	subcommand := i.ApplicationCommandData().Options[0]
	subcommandName := subcommand.Name

	username := "Usuario"
	if i.Member != nil {
		username = i.Member.User.Username
	} else if i.User != nil {
		username = i.User.Username
	}
	getLogger().Debugf("Comando recibido: /dota %s de %s", subcommandName, username)

	// Ejecutar el subcomando correspondiente
	switch subcommandName {
	case "search":
		b.handleSearchSlash(s, i, subcommand)
	case "register":
		b.handleRegisterSlash(s, i, subcommand)
	case "channel":
		b.handleChannelSlash(s, i, subcommand)
	case "stats":
		b.handleStatsSlash(s, i)
	case "help":
		b.handleHelpSlash(s, i)
	default:
		b.sendFollowup(s, i, "❌ Comando no reconocido. Usa `/dota help` para ver los comandos disponibles.")
	}
}

func (b *Bot) sendFollowup(s *discordgo.Session, i *discordgo.InteractionCreate, content string) {
	_, err := s.FollowupMessageCreate(i.Interaction, true, &discordgo.WebhookParams{
		Content: content,
	})
	if err != nil {
		getLogger().Errorf("Error enviando followup: %v", err)
	}
}

func (b *Bot) sendFollowupEmbed(s *discordgo.Session, i *discordgo.InteractionCreate, embed *discordgo.MessageEmbed) {
	_, err := s.FollowupMessageCreate(i.Interaction, true, &discordgo.WebhookParams{
		Embeds: []*discordgo.MessageEmbed{embed},
	})
	if err != nil {
		getLogger().Errorf("Error enviando followup embed: %v", err)
	}
}

// Handlers para Slash Commands
func (b *Bot) handleSearchSlash(s *discordgo.Session, i *discordgo.InteractionCreate, subcommand *discordgo.ApplicationCommandInteractionDataOption) {
	// Obtener el parámetro "nombre"
	var query string
	for _, option := range subcommand.Options {
		if option.Name == "nombre" {
			query = option.StringValue()
			break
		}
	}

	if query == "" {
		b.sendFollowup(s, i, "❌ Uso: `/dota search nombre:<nombre>`")
		return
	}

	getLogger().Debugf("Buscando jugadores: %s", query)

	if b.stratzClient == nil || !b.stratzClient.IsConfigured() {
		b.sendFollowup(s, i, "❌ Stratz no está configurado.")
		return
	}

	results, err := b.stratzClient.SearchPlayers(query)
	if err != nil {
		if errors.Is(err, dota.ErrSearchNotSupported) {
			b.sendFollowup(s, i, "🔍 La búsqueda por nombre no está disponible con Stratz.\n\nUsa **Steam ID** (account_id) directamente:\n`/dota register account_id:<tu_steam_id>`\n\nPuedes encontrar tu Steam ID en https://stratz.com (busca tu perfil o partidas).")
			return
		}
		getLogger().Errorf("Error buscando jugadores: %v", err)
		b.sendFollowup(s, i, fmt.Sprintf("❌ Error en la búsqueda: %v", err))
		return
	}

	if len(results) == 0 {
		b.sendFollowup(s, i, "❌ No se encontraron jugadores con ese nombre")
		return
	}

	// Limitar a 10 resultados
	if len(results) > 10 {
		results = results[:10]
	}

	// Guardar en cache
	userID := i.Member.User.ID
	if i.Member == nil && i.User != nil {
		userID = i.User.ID
	}
	b.searchCache[userID] = results

	// Construir mensaje
	var msg strings.Builder
	msg.WriteString("🔍 **Resultados de búsqueda:**\n\n")

	for idx, result := range results {
		personaname := result.Personaname
		if personaname == "" {
			personaname = "Sin nombre"
		}
		msg.WriteString(fmt.Sprintf("**%d.** %s (ID: %d)\n", idx+1, personaname, result.AccountID))
		if result.LastMatchTime != "" {
			msg.WriteString(fmt.Sprintf("   Última partida: %s\n", result.LastMatchTime))
		}
		msg.WriteString("\n")
	}

	msg.WriteString("Usa `/dota register account_id:<número>` para registrar un jugador\nO `/dota register account_id:<número> usuario:@amigo` para registrar a otro usuario")

	b.sendFollowup(s, i, msg.String())
}

func (b *Bot) handleRegisterSlash(s *discordgo.Session, i *discordgo.InteractionCreate, subcommand *discordgo.ApplicationCommandInteractionDataOption) {
	// Obtener el parámetro "account_id"
	var accountIDInput string
	var targetUser *discordgo.User

	for _, option := range subcommand.Options {
		if option.Name == "account_id" {
			accountIDInput = option.StringValue()
		} else if option.Name == "usuario" {
			targetUser = option.UserValue(s)
		}
	}

	if accountIDInput == "" {
		b.sendFollowup(s, i, "❌ Uso: `/dota register account_id:<account_id>` o `/dota register account_id:<número>` después de una búsqueda")
		return
	}

	var accountID string

	// Verificar si es un número (resultado de búsqueda)
	if num, err := strconv.Atoi(accountIDInput); err == nil && num > 0 && num <= 10 {
		// Es un número, buscar en cache
		// Usar el usuario que ejecuta el comando para el cache (no el target)
		cacheKey := i.Member.User.ID
		if i.Member == nil && i.User != nil {
			cacheKey = i.User.ID
		}
		if results, ok := b.searchCache[cacheKey]; ok && num <= len(results) {
			accountID = strconv.Itoa(results[num-1].AccountID)
			delete(b.searchCache, cacheKey) // Limpiar cache
		} else {
			b.sendFollowup(s, i, "❌ No hay resultados de búsqueda disponibles. Usa `/dota search nombre:<nombre>` primero.")
			return
		}
	} else {
		// Es un account_id directo
		accountID = accountIDInput
		// Validar que sea numérico
		if _, err := strconv.Atoi(accountID); err != nil {
			b.sendFollowup(s, i, "❌ El account_id debe ser un número")
			return
		}
	}

	// Verificar que el jugador existe (solo Stratz)
	if b.stratzClient == nil || !b.stratzClient.IsConfigured() {
		b.sendFollowup(s, i, "❌ Stratz no está configurado.")
		return
	}
	accountIDInt, errParse := strconv.ParseInt(accountID, 10, 64)
	if errParse != nil {
		b.sendFollowup(s, i, "❌ account_id inválido")
		return
	}
	profileStratz, err := b.stratzClient.GetPlayerProfile(accountIDInt)
	if err != nil {
		getLogger().Errorf("Error obteniendo perfil Stratz: %v", err)
		b.sendFollowup(s, i, fmt.Sprintf("❌ Error verificando jugador (Stratz): %v", err))
		return
	}
	if profileStratz == nil {
		b.sendFollowup(s, i, "❌ No se encontró el jugador en Stratz")
		return
	}

	// Determinar qué usuario registrar
	var userID string
	var discordUsername string

	if targetUser != nil {
		// Registrar al usuario especificado
		userID = targetUser.ID
		discordUsername = targetUser.Username
		getLogger().Debugf("Registrando usuario especificado: %s (%s)", discordUsername, userID)
	} else {
		// Registrar al usuario que ejecuta el comando
		if i.Member != nil && i.Member.User != nil {
			userID = i.Member.User.ID
			discordUsername = i.Member.User.Username
		} else if i.User != nil {
			userID = i.User.ID
			discordUsername = i.User.Username
		} else {
			b.sendFollowup(s, i, "❌ No se pudo identificar al usuario")
			return
		}
		getLogger().Debugf("Registrando usuario que ejecuta el comando: %s (%s)", discordUsername, userID)
	}

	// Registrar usuario
	if err := b.userStore.Set(userID, accountID); err != nil {
		getLogger().Errorf("Error guardando usuario: %v", err)
		b.sendFollowup(s, i, "❌ Error guardando registro")
		return
	}

	personaname := profileStratz.Name
	if personaname == "" {
		personaname = "Jugador"
	}

	b.sendFollowup(s, i, fmt.Sprintf("✅ **%s** (Discord) asociado con **%s** (Dota 2)\nID de Dota: %s", discordUsername, personaname, accountID))
	getLogger().Infof("Usuario Discord %s (%s) registrado con account_id %s", userID, discordUsername, accountID)
}

func (b *Bot) handleChannelSlash(s *discordgo.Session, i *discordgo.InteractionCreate, subcommand *discordgo.ApplicationCommandInteractionDataOption) {
	var channelID string

	// Obtener el parámetro "canal"
	for _, option := range subcommand.Options {
		if option.Name == "canal" {
			channelID = option.ChannelValue(s).ID
			break
		}
	}

	if channelID == "" {
		b.sendFollowup(s, i, "❌ Canal no válido")
		return
	}

	// Guardar canal
	if err := b.userStore.SetChannel(channelID); err != nil {
		getLogger().Errorf("Error guardando canal: %v", err)
		b.sendFollowup(s, i, "❌ Error guardando canal")
		return
	}

	b.sendFollowup(s, i, fmt.Sprintf("✅ Canal de notificaciones configurado: <#%s>", channelID))
	getLogger().Infof("Canal de notificaciones configurado: %s", channelID)
}

// getPlayerNameAndAvatar obtiene nombre y avatar del jugador (Stratz + fallback OpenDota), igual que la notificación de partida.
func (b *Bot) getPlayerNameAndAvatar(accountID string, accountIDInt int64) (playerName, avatarURL string) {
	profileStratz, _ := b.stratzClient.GetPlayerProfile(accountIDInt)
	if profileStratz != nil {
		playerName = profileStratz.Name
		avatarURL = profileStratz.Avatar
	}
	if b.dotaClient != nil {
		profileOD, errOD := b.dotaClient.GetPlayerProfile(accountID)
		if errOD == nil && profileOD != nil {
			if profileOD.Profile.Personaname != "" && playerName == "" {
				playerName = profileOD.Profile.Personaname
			}
			if profileOD.Profile.Avatarfull != "" && avatarURL == "" {
				avatarURL = profileOD.Profile.Avatarfull
			}
		}
	}
	return playerName, avatarURL
}

// buildStatsEmbed construye el embed de estadísticas por héroe (W/L, %). playerName en título; avatarURL opcional (Author + Thumbnail como en notificación).
func (b *Bot) buildStatsEmbed(heroStats []dota.StratzHeroStats, minGames, take int, playerName, avatarURL string) *discordgo.MessageEmbed {
	var red, yellow, green []string
	for _, h := range heroStats {
		winPct := 0.0
		if h.MatchCount > 0 {
			winPct = 100 * float64(h.WinCount) / float64(h.MatchCount)
		}
		name := b.dotaClient.GetHeroName(h.HeroID)
		line := fmt.Sprintf("%s | %d-%d | %.1f%%", name, h.WinCount, h.MatchCount-h.WinCount, winPct)
		switch {
		case winPct < 40:
			red = append(red, line)
		case winPct <= 50:
			yellow = append(yellow, line)
		default:
			green = append(green, line)
		}
	}
	const maxDesc = 4000
	var parts []string
	if len(red) > 0 {
		parts = append(parts, "🔴 **<40%**\n"+strings.Join(red, "\n"))
	}
	if len(yellow) > 0 {
		parts = append(parts, "🟡 **40-50%**\n"+strings.Join(yellow, "\n"))
	}
	if len(green) > 0 {
		parts = append(parts, "🟢 **>50%**\n"+strings.Join(green, "\n"))
	}
	description := strings.Join(parts, "\n\n")
	if len(description) > maxDesc {
		description = description[:maxDesc-3] + "..."
	}
	displayName := playerName
	if displayName == "" {
		displayName = "Jugador"
	}
	title := fmt.Sprintf("📊 Estadísticas por héroe — %s", displayName)
	embed := &discordgo.MessageEmbed{
		Title:       title,
		Description: description,
		Color:       0x3498db,
		Footer:      &discordgo.MessageEmbedFooter{Text: fmt.Sprintf("%d partidas analizadas • ≥%d partidas por héroe • Stratz", take, minGames)},
	}
	if avatarURL != "" {
		embed.Author = &discordgo.MessageEmbedAuthor{
			Name:    displayName,
			IconURL: avatarURL,
		}
		embed.Thumbnail = &discordgo.MessageEmbedThumbnail{URL: avatarURL}
	}
	return embed
}

func (b *Bot) handleStatsSlash(s *discordgo.Session, i *discordgo.InteractionCreate) {
	if b.stratzClient == nil || !b.stratzClient.IsConfigured() {
		b.sendFollowup(s, i, "❌ Stratz no está configurado.")
		return
	}
	users := b.userStore.GetAll()
	if len(users) == 0 {
		b.sendFollowup(s, i, "❌ No hay usuarios registrados. Usa `/dota register account_id:<tu_steam_id>` para registrar jugadores.")
		return
	}
	minGames := b.config.StatsMinGames
	take := b.config.StatsTake
	getLogger().Debugf("stats: mostrando %d usuario(s) registrado(s)", len(users))
	sent := 0
	for _, accountID := range users {
		accountIDInt, errParse := strconv.ParseInt(accountID, 10, 64)
		if errParse != nil {
			getLogger().Debugf("stats: account_id inválido omitido: %s", accountID)
			continue
		}
		playerName, avatarURL := b.getPlayerNameAndAvatar(accountID, accountIDInt)
		heroStats, err := b.stratzClient.GetPlayerHeroStats(accountIDInt, minGames, take)
		if err != nil {
			getLogger().Errorf("stats: GetPlayerHeroStats para %s: %v", accountID, err)
			continue
		}
		if len(heroStats) == 0 {
			getLogger().Debugf("stats: sin héroes con ≥%d partidas para %s", minGames, accountID)
			continue
		}
		embed := b.buildStatsEmbed(heroStats, minGames, take, playerName, avatarURL)
		b.sendFollowupEmbed(s, i, embed)
		sent++
		time.Sleep(500 * time.Millisecond) // evitar rate limit entre followups
	}
	if sent == 0 {
		b.sendFollowup(s, i, fmt.Sprintf("Ningún jugador registrado tiene héroes con al menos %d partidas en las últimas %d partidas analizadas.", minGames, take))
	}
}

func (b *Bot) handleHelpSlash(s *discordgo.Session, i *discordgo.InteractionCreate) {
	embed := &discordgo.MessageEmbed{
		Title:       "🎮 Comandos del Bot de Dota 2",
		Description: "Comandos disponibles. Usa register para asociar un Discord ID con un ID de Dota.",
		Color:       0x3498db,
		Fields: []*discordgo.MessageEmbedField{
			{
				Name:   "/dota search nombre:<nombre>",
				Value:  "Buscar jugadores por nombre de Steam. Devuelve hasta 10 resultados numerados.\n**Ejemplo:** `/dota search nombre:Desp4irs`",
				Inline: false,
			},
			{
				Name:   "/dota register account_id:<id> [usuario:@amigo]",
				Value:  "Asocia el ID de Dota a un usuario de Discord y lo guarda en `data/users.json`.\n- Si omites `usuario`, te registras tú.\n- Usa número tras una búsqueda (1-10) o ID directo.\n**Ejemplos:** `/dota register account_id:136201811` · `/dota register account_id:1` · `/dota register account_id:136201811 usuario:@amigo`",
				Inline: false,
			},
			{
				Name:   "/dota channel canal:<#canal>",
				Value:  "Configura el canal para notificaciones automáticas de nuevas partidas.\n**Ejemplo:** `/dota channel canal:#dota-updates`",
				Inline: false,
			},
			{
				Name:   "/dota stats",
				Value:  "Un mensaje por cada usuario registrado: estadísticas por héroe (W/L, %) con ≥STATS_MIN_GAMES partidas en las últimas STATS_TAKE partidas. Colores: 🔴 ≤40%, 🟡 40-50%, 🟢 ≥50%.",
				Inline: false,
			},
			{
				Name:   "/dota help",
				Value:  "Mostrar esta ayuda",
				Inline: false,
			},
		},
	}

	b.sendFollowupEmbed(s, i, embed)
}

// Handlers antiguos (mantener por compatibilidad, pero no se usan con slash commands)
func (b *Bot) handleRegister(s *discordgo.Session, m *discordgo.MessageCreate, args []string) {
	if len(args) == 0 {
		s.ChannelMessageSend(m.ChannelID, "❌ Uso: `/dota register <account_id>` o `/dota register <número>` después de una búsqueda")
		return
	}

	var accountID string

	// Verificar si es un número (resultado de búsqueda)
	if num, err := strconv.Atoi(args[0]); err == nil && num > 0 && num <= 10 {
		// Es un número, buscar en cache
		cacheKey := m.Author.ID
		if results, ok := b.searchCache[cacheKey]; ok && num <= len(results) {
			accountID = strconv.Itoa(results[num-1].AccountID)
			delete(b.searchCache, cacheKey) // Limpiar cache
		} else {
			s.ChannelMessageSend(m.ChannelID, "❌ No hay resultados de búsqueda disponibles. Usa `/dota search <nombre>` primero.")
			return
		}
	} else {
		// Es un account_id directo
		accountID = args[0]
		// Validar que sea numérico
		if _, err := strconv.Atoi(accountID); err != nil {
			s.ChannelMessageSend(m.ChannelID, "❌ El account_id debe ser un número")
			return
		}
	}

	// Verificar que el jugador existe (solo Stratz)
	if b.stratzClient == nil || !b.stratzClient.IsConfigured() {
		s.ChannelMessageSend(m.ChannelID, "❌ Stratz no está configurado.")
		return
	}
	accountIDInt, errParse := strconv.ParseInt(accountID, 10, 64)
	if errParse != nil {
		s.ChannelMessageSend(m.ChannelID, "❌ account_id inválido")
		return
	}
	profileStratz, err := b.stratzClient.GetPlayerProfile(accountIDInt)
	if err != nil {
		getLogger().Errorf("Error obteniendo perfil Stratz: %v", err)
		s.ChannelMessageSend(m.ChannelID, fmt.Sprintf("❌ Error verificando jugador: %v", err))
		return
	}
	if profileStratz == nil {
		s.ChannelMessageSend(m.ChannelID, "❌ No se encontró el jugador en Stratz")
		return
	}

	if err := b.userStore.Set(m.Author.ID, accountID); err != nil {
		getLogger().Errorf("Error guardando usuario: %v", err)
		s.ChannelMessageSend(m.ChannelID, "❌ Error guardando registro")
		return
	}

	personaname := profileStratz.Name
	if personaname == "" {
		personaname = "Jugador"
	}

	s.ChannelMessageSend(m.ChannelID, fmt.Sprintf("✅ Usuario registrado: **%s** (ID: %s)", personaname, accountID))
	getLogger().Infof("Usuario %s registrado con account_id %s", m.Author.ID, accountID)
}

func (b *Bot) handleSearch(s *discordgo.Session, m *discordgo.MessageCreate, args []string) {
	if len(args) == 0 {
		s.ChannelMessageSend(m.ChannelID, "❌ Uso: `/dota search <nombre>`")
		return
	}

	query := strings.Join(args, " ")
	getLogger().Debugf("Buscando jugadores: %s", query)

	if b.stratzClient == nil || !b.stratzClient.IsConfigured() {
		s.ChannelMessageSend(m.ChannelID, "❌ Stratz no está configurado.")
		return
	}
	results, err := b.stratzClient.SearchPlayers(query)
	if err != nil {
		if errors.Is(err, dota.ErrSearchNotSupported) {
			s.ChannelMessageSend(m.ChannelID, "🔍 La búsqueda por nombre no está disponible. Usa `/dota register account_id:<tu_steam_id>`. Encuentra tu ID en https://stratz.com")
			return
		}
		getLogger().Errorf("Error buscando jugadores: %v", err)
		s.ChannelMessageSend(m.ChannelID, fmt.Sprintf("❌ Error en la búsqueda: %v", err))
		return
	}

	if len(results) == 0 {
		s.ChannelMessageSend(m.ChannelID, "❌ No se encontraron jugadores con ese nombre")
		return
	}

	// Limitar a 10 resultados
	if len(results) > 10 {
		results = results[:10]
	}

	// Guardar en cache
	b.searchCache[m.Author.ID] = results

	// Construir mensaje
	var msg strings.Builder
	msg.WriteString("🔍 **Resultados de búsqueda:**\n\n")

	for i, result := range results {
		personaname := result.Personaname
		if personaname == "" {
			personaname = "Sin nombre"
		}
		msg.WriteString(fmt.Sprintf("**%d.** %s (ID: %d)\n", i+1, personaname, result.AccountID))
		if result.LastMatchTime != "" {
			msg.WriteString(fmt.Sprintf("   Última partida: %s\n", result.LastMatchTime))
		}
		msg.WriteString("\n")
	}

	msg.WriteString("Usa `/dota register <número>` para registrar un jugador")

	s.ChannelMessageSend(m.ChannelID, msg.String())
}

func (b *Bot) handleChannel(s *discordgo.Session, m *discordgo.MessageCreate, args []string) {
	if len(args) == 0 {
		s.ChannelMessageSend(m.ChannelID, "❌ Uso: `/dota channel <#canal>`")
		return
	}

	// Extraer ID del canal de la mención
	channelID := ""
	if len(m.MentionChannels) > 0 {
		channelID = m.MentionChannels[0].ID
	} else {
		// Intentar extraer del texto
		text := strings.Join(args, " ")
		if strings.HasPrefix(text, "<#") && strings.HasSuffix(text, ">") {
			channelID = strings.TrimPrefix(strings.TrimSuffix(text, ">"), "<#")
		}
	}

	if channelID == "" {
		s.ChannelMessageSend(m.ChannelID, "❌ Canal no válido. Menciona el canal con #")
		return
	}

	// Verificar que el canal existe
	_, err := s.Channel(channelID)
	if err != nil {
		s.ChannelMessageSend(m.ChannelID, "❌ Canal no encontrado")
		return
	}

	// Guardar canal
	if err := b.userStore.SetChannel(channelID); err != nil {
		getLogger().Errorf("Error guardando canal: %v", err)
		s.ChannelMessageSend(m.ChannelID, "❌ Error guardando canal")
		return
	}

	s.ChannelMessageSend(m.ChannelID, fmt.Sprintf("✅ Canal de notificaciones configurado: <#%s>", channelID))
	getLogger().Infof("Canal de notificaciones configurado: %s", channelID)
}

func (b *Bot) handleHelp(s *discordgo.Session, m *discordgo.MessageCreate) {
	embed := &discordgo.MessageEmbed{
		Title:       "🎮 Comandos del Bot de Dota 2",
		Description: "Lista de comandos disponibles",
		Color:       0x3498db,
		Fields: []*discordgo.MessageEmbedField{
			{
				Name:   "/dota search <nombre>",
				Value:  "Buscar jugadores por nombre de Steam",
				Inline: false,
			},
			{
				Name:   "/dota register <account_id>",
				Value:  "Registrar tu ID de Dota directamente",
				Inline: false,
			},
			{
				Name:   "/dota register <número>",
				Value:  "Registrar seleccionando un número de los resultados de búsqueda",
				Inline: false,
			},
			{
				Name:   "/dota channel <#canal>",
				Value:  "Configurar canal para notificaciones automáticas",
				Inline: false,
			},
			{
				Name:   "/dota stats",
				Value:  "Estadísticas por héroe en el parche actual (W/L, % victorias)",
				Inline: false,
			},
			{
				Name:   "/dota help",
				Value:  "Mostrar esta ayuda",
				Inline: false,
			},
		},
	}

	s.ChannelMessageSendEmbed(m.ChannelID, embed)
}

// isValidSnowflake valida que un string sea un snowflake válido de Discord (número)
func isValidSnowflake(id string) bool {
	if id == "" {
		return false
	}
	// Verificar que todos los caracteres sean dígitos
	for _, r := range id {
		if r < '0' || r > '9' {
			return false
		}
	}
	// Los snowflakes de Discord tienen entre 17 y 19 dígitos
	return len(id) >= 17 && len(id) <= 19
}

func (b *Bot) SendWelcomeMessage() error {
	// Obtener canal de notificaciones
	channelID, err := b.userStore.GetChannel()
	if err != nil || channelID == "" {
		// Usar canal por defecto de la configuración si está disponible
		channelID = b.config.NotificationChannelID
		if channelID == "" {
			getLogger().Info("No hay canal configurado, omitiendo mensaje de bienvenida")
			return nil
		}
	}

	// Validar que el channelID sea un número válido (snowflake de Discord)
	if !isValidSnowflake(channelID) {
		getLogger().Warnf("ID de canal inválido detectado: %s (debe ser un número). Limpiando configuración.", channelID)
		// Limpiar el canal inválido
		b.userStore.SetChannel("")
		return fmt.Errorf("ID de canal inválido: %s (debe ser un número de Discord)", channelID)
	}

	embed := &discordgo.MessageEmbed{
		Title:       "🤖 Bot de Dota 2 - ¡En línea!",
		Description: "El bot está funcionando y monitoreando partidas. Aquí están los comandos disponibles:",
		Color:       0x3498db,
		Fields: []*discordgo.MessageEmbedField{
			{
				Name:   "1️⃣ /dota help",
				Value:  "Muestra esta lista completa de comandos.",
				Inline: false,
			},
			{
				Name:   "2️⃣ /dota search nombre:<nombre>",
				Value:  "Busca jugadores de Dota 2 por nombre de Steam. Devuelve hasta 10 resultados numerados.\n**Ejemplo:** `/dota search nombre:Desp4irs`",
				Inline: false,
			},
			{
				Name:   "3️⃣ /dota register account_id:<id> [usuario:@amigo]",
				Value:  "Asocia un usuario de Discord con un ID de Dota 2 y lo guarda en `data/users.json`.\n- Si omites `usuario`, se registra quien ejecuta el comando.\n- Puedes registrar a un amigo con `usuario:@amigo`.\n**Ejemplos:** `/dota register account_id:136201811` · `/dota register account_id:1` · `/dota register account_id:136201811 usuario:@amigo`",
				Inline: false,
			},
			{
				Name:   "4️⃣ /dota channel canal:<#canal>",
				Value:  "Configura el canal para notificaciones automáticas de nuevas partidas.\n**Ejemplo:** `/dota channel canal:#dota-updates`",
				Inline: false,
			},
			{
				Name:   "5️⃣ /dota stats",
				Value:  "Estadísticas por héroe en el parche actual: W/L y % victorias (héroes con ≥10 partidas).",
				Inline: false,
			},
		},
		Footer: &discordgo.MessageEmbedFooter{
			Text: "El bot verifica nuevas partidas cada 10 minutos automáticamente",
		},
	}

	_, err = b.session.ChannelMessageSendEmbed(channelID, embed)
	if err != nil {
		return fmt.Errorf("error enviando mensaje de bienvenida: %w", err)
	}

	getLogger().Infof("Mensaje de bienvenida enviado al canal %s", channelID)
	return nil
}

func (b *Bot) CheckForNewMatches() error {
	getLogger().Debug("Verificando nuevas partidas...")

	users := b.userStore.GetAll()
	if len(users) == 0 {
		getLogger().Debug("No hay usuarios registrados")
		return nil
	}

	// Obtener canal de notificaciones
	channelID, err := b.userStore.GetChannel()
	if err != nil || channelID == "" {
		// Usar canal por defecto de la configuración si está disponible
		channelID = b.config.NotificationChannelID
		if channelID == "" {
			getLogger().Debug("No hay canal de notificaciones configurado")
			return nil
		}
	}

	// Validar que el channelID sea válido
	if !isValidSnowflake(channelID) {
		getLogger().Warnf("ID de canal inválido: %s. Saltando verificación de partidas.", channelID)
		return nil
	}

	if b.stratzClient == nil || !b.stratzClient.IsConfigured() {
		getLogger().Debug("Stratz no configurado, omitiendo verificación de partidas")
		return nil
	}

	for discordID, accountID := range users {
		lastMatchID, hasLastMatch := b.userStore.GetLastMatch(discordID)

		accountIDInt, errParse := strconv.ParseInt(accountID, 10, 64)
		if errParse != nil {
			getLogger().Warnf("account_id inválido para %s: %s", discordID, accountID)
			continue
		}

		// Partidas recientes desde Stratz
		matches, err := b.stratzClient.GetPlayerRecentMatches(accountIDInt, 5)
		if err != nil {
			getLogger().Errorf("Error obteniendo partidas Stratz para %s: %v", accountID, err)
			continue
		}

		if len(matches) == 0 {
			continue
		}

		latestStratzMatch := matches[0]
		if hasLastMatch && latestStratzMatch.ID == lastMatchID {
			continue
		}

		getLogger().Infof("Nueva partida detectada para %s: %d", accountID, latestStratzMatch.ID)

		// Detalles de la partida desde Stratz
		matchDetailsStratz, err := b.stratzClient.GetMatch(latestStratzMatch.ID)
		if err != nil {
			getLogger().Errorf("Error obteniendo detalles partida %d: %v", latestStratzMatch.ID, err)
			continue
		}

		// Si PARSED=true: solo notificar cuando la partida esté parseada (parsedDateTime > 0)
		if b.config.RequireParsed && matchDetailsStratz != nil && !dota.IsMatchParsed(matchDetailsStratz) {
			parsedVal := "null"
			if matchDetailsStratz.ParsedDateTime != nil {
				parsedVal = strconv.FormatInt(*matchDetailsStratz.ParsedDateTime, 10)
			}
			getLogger().Debugf("Partida %d no parseada (parsedDateTime=%s), solicitando parse y omitiendo notificación este ciclo", latestStratzMatch.ID, parsedVal)
			if errParse := b.stratzClient.RequestParseMatch(latestStratzMatch.ID); errParse != nil {
				getLogger().Debugf("RequestParseMatch para %d: %v (la API puede no exponer la mutación)", latestStratzMatch.ID, errParse)
			}
			// No actualizar lastMatchID: en el siguiente ciclo se reintentará
			continue
		}

		// Buscar al jugador en la partida
		var playerStratz *dota.StratzPlayer
		for i := range matchDetailsStratz.Players {
			if matchDetailsStratz.Players[i].SteamAccountID == accountIDInt {
				playerStratz = &matchDetailsStratz.Players[i]
				break
			}
		}
		if playerStratz == nil {
			getLogger().Errorf("Jugador %s no encontrado en partida %d", accountID, latestStratzMatch.ID)
			continue
		}

		profileStratz, _ := b.stratzClient.GetPlayerProfile(accountIDInt)

		// Convertir a tipos dota para sendMatchNotification
		matchDetails := dota.StratzMatchToMatchResponse(matchDetailsStratz)
		var player *dota.Player
		for j := range matchDetails.Players {
			if matchDetails.Players[j].AccountID == int(accountIDInt) {
				player = &matchDetails.Players[j]
				break
			}
		}
		if player == nil {
			continue
		}

		var profile *dota.PlayersResponse
		if profileStratz != nil {
			profile = &dota.PlayersResponse{}
			profile.Profile.Personaname = profileStratz.Name
			profile.Profile.Avatarfull = profileStratz.Avatar
			profile.Profile.AccountID = int(profileStratz.SteamAccountID)
			profile.RankBracket = profileStratz.RankBracket
		}
		// Nombre y avatar: fallback a OpenDota si Stratz no devuelve
		if b.dotaClient != nil {
			profileOD, errOD := b.dotaClient.GetPlayerProfile(accountID)
			if errOD == nil && profileOD != nil {
				if profile == nil {
					profile = &dota.PlayersResponse{}
					profile.Profile.AccountID = int(accountIDInt)
					if profileStratz != nil {
						profile.Profile.Personaname = profileStratz.Name
						profile.RankBracket = profileStratz.RankBracket
					}
				}
				if profileOD.Profile.Personaname != "" && profile.Profile.Personaname == "" {
					profile.Profile.Personaname = profileOD.Profile.Personaname
				}
				if profileOD.Profile.Avatarfull != "" && profile.Profile.Avatarfull == "" {
					profile.Profile.Avatarfull = profileOD.Profile.Avatarfull
				}
			}
		}

		if err := b.sendMatchNotification(channelID, matchDetails, player, profile, accountID); err != nil {
			getLogger().Errorf("Error enviando notificación: %v", err)
			continue
		}

		if err := b.userStore.SetLastMatch(discordID, latestStratzMatch.ID); err != nil {
			getLogger().Errorf("Error guardando última partida: %v", err)
		}

		time.Sleep(2 * time.Second)
	}

	return nil
}

// RunStatsScheduler ejecuta en bucle y, a la hora STATS_TIME (HH:MM), envía stats de todos los registrados al canal de notificaciones.
// Si STATS_TIME está vacío, retorna sin hacer nada.
func (b *Bot) RunStatsScheduler() {
	if b.config.StatsTime == "" {
		getLogger().Debug("STATS_TIME vacío, scheduler de stats desactivado")
		return
	}
	statsTime, err := time.Parse("15:04", b.config.StatsTime)
	if err != nil {
		getLogger().Warnf("STATS_TIME inválido (%q), usar HH:MM (ej. 20:00): %v", b.config.StatsTime, err)
		return
	}
	targetHour, targetMin := statsTime.Hour(), statsTime.Minute()
	if b.stratzClient == nil || !b.stratzClient.IsConfigured() {
		getLogger().Warn("Stats diarios: Stratz no configurado, scheduler desactivado")
		return
	}
	getLogger().Infof("Scheduler de stats diarios a las %s", b.config.StatsTime)

	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()
	for range ticker.C {
		now := time.Now()
		if now.Hour() != targetHour || now.Minute() != targetMin {
			continue
		}
		today := now.Format("2006-01-02")
		b.statsMu.Lock()
		if b.lastStatsDay == today {
			b.statsMu.Unlock()
			continue
		}
		b.lastStatsDay = today
		b.statsMu.Unlock()

		channelID, errChan := b.userStore.GetChannel()
		if errChan != nil || channelID == "" {
			getLogger().Warn("Stats diarios: no hay canal configurado, omitiendo")
			continue
		}
		users := b.userStore.GetAll()
		if len(users) == 0 {
			getLogger().Debug("Stats diarios: no hay usuarios registrados")
			continue
		}
		minGames := b.config.StatsMinGames
		take := b.config.StatsTake
		getLogger().Infof("Enviando stats diarios para %d jugador(es) a las %s", len(users), b.config.StatsTime)
		for discordID, accountID := range users {
			accountIDInt, errParse := strconv.ParseInt(accountID, 10, 64)
			if errParse != nil {
				getLogger().Debugf("Stats diarios: account_id inválido para %s: %s", discordID, accountID)
				continue
			}
			playerName, avatarURL := b.getPlayerNameAndAvatar(accountID, accountIDInt)
			heroStats, err := b.stratzClient.GetPlayerHeroStats(accountIDInt, minGames, take)
			if err != nil {
				getLogger().Errorf("Stats diarios: GetPlayerHeroStats para %s: %v", accountID, err)
				continue
			}
			if len(heroStats) == 0 {
				getLogger().Debugf("Stats diarios: sin héroes con ≥%d partidas para %s", minGames, accountID)
				continue
			}
			embed := b.buildStatsEmbed(heroStats, minGames, take, playerName, avatarURL)
			_, errSend := b.session.ChannelMessageSendEmbed(channelID, embed)
			if errSend != nil {
				getLogger().Errorf("Stats diarios: error enviando embed para %s: %v", accountID, errSend)
			}
			time.Sleep(1 * time.Second) // evitar rate limit
		}
	}
}

// formatLaneOutcomeEnum devuelve texto en español para LaneOutcomeEnums de Stratz.
func formatLaneOutcomeEnum(outcome string) string {
	switch strings.ToUpper(outcome) {
	case "RADIANT_VICTORY":
		return "Victoria Radiant"
	case "RADIANT_STOMP":
		return "Stomp Radiant"
	case "DIRE_VICTORY":
		return "Victoria Dire"
	case "DIRE_STOMP":
		return "Stomp Dire"
	case "TIE":
		return "Empate"
	default:
		if outcome == "" {
			return "—"
		}
		return outcome
	}
}

// formatLaneOutcomeWithColor devuelve el texto del outcome con emoji de color según si el jugador ganó o perdió esa línea.
// isRadiant = equipo del jugador. 🟢 = victoria de su equipo en esa línea, 🔴 = derrota, sin emoji = empate.
func formatLaneOutcomeWithColor(outcome string, isRadiant bool) string {
	text := formatLaneOutcomeEnum(outcome)
	upper := strings.ToUpper(outcome)
	if upper == "TIE" || upper == "" {
		return text
	}
	radiantWon := upper == "RADIANT_VICTORY" || upper == "RADIANT_STOMP"
	if radiantWon && isRadiant {
		return "🟢 " + text
	}
	if radiantWon && !isRadiant {
		return "🔴 " + text
	}
	// Dire won
	if !isRadiant {
		return "🟢 " + text
	}
	return "🔴 " + text
}

// playerLanePosition devuelve "top","mid","bottom" según Stratz lane + isRadiant; "" si jungle/roaming.
func playerLanePosition(lane string, isRadiant bool) string {
	switch strings.ToUpper(lane) {
	case "SAFE_LANE":
		if isRadiant {
			return "bottom"
		}
		return "top"
	case "OFF_LANE":
		if isRadiant {
			return "top"
		}
		return "bottom"
	case "MID_LANE":
		return "mid"
	default:
		return ""
	}
}

// buildLaneOutcomeText construye (1) línea de victoria/derrota en fase de línea para el jugador (texto pequeño) y (2) resumen por línea.
// En el resumen por línea: 🟢 = victoria del equipo del jugador en esa línea, 🔴 = derrota, sin emoji = empate.
func (b *Bot) buildLaneOutcomeText(match *dota.MatchResponse, player *dota.Player) (lanePhaseLine, laneSummary string) {
	isRadiant := player.IsRadiant != nil && *player.IsRadiant
	topO := formatLaneOutcomeWithColor(match.TopLaneOutcome, isRadiant)
	midO := formatLaneOutcomeWithColor(match.MidLaneOutcome, isRadiant)
	botO := formatLaneOutcomeWithColor(match.BottomLaneOutcome, isRadiant)
	pos := playerLanePosition(player.Lane, isRadiant)

	// Resumen siempre: Top / Mid / Bottom (marcar (tú) en la línea del jugador)
	topLabel := "Top"
	midLabel := "Mid"
	botLabel := "Bottom"
	if pos == "top" {
		topLabel = "Top (tú)"
	} else if pos == "mid" {
		midLabel = "Mid (tú)"
	} else if pos == "bottom" {
		botLabel = "Bottom (tú)"
	}
	laneSummary = fmt.Sprintf("%s: %s\n%s: %s\n%s: %s", topLabel, topO, midLabel, midO, botLabel, botO)

	// Victoria/derrota en fase de línea solo si jugó una línea (no jungle/roaming)
	if pos == "" {
		return "", laneSummary
	}
	var outcome string
	switch pos {
	case "top":
		outcome = match.TopLaneOutcome
	case "mid":
		outcome = match.MidLaneOutcome
	case "bottom":
		outcome = match.BottomLaneOutcome
	default:
		return "", laneSummary
	}
	outcome = strings.ToUpper(outcome)
	var laneResult string
	switch outcome {
	case "TIE":
		laneResult = "*Empate en fase de línea*"
	case "RADIANT_VICTORY", "RADIANT_STOMP":
		if isRadiant {
			laneResult = "*✅ Victoria en fase de línea*"
		} else {
			laneResult = "*❌ Derrota en fase de línea*"
		}
	case "DIRE_VICTORY", "DIRE_STOMP":
		if isRadiant {
			laneResult = "*❌ Derrota en fase de línea*"
		} else {
			laneResult = "*✅ Victoria en fase de línea*"
		}
	default:
		laneResult = ""
	}
	return laneResult, laneSummary
}

func (b *Bot) sendMatchNotification(channelID string, match *dota.MatchResponse, player *dota.Player, profile *dota.PlayersResponse, accountID string) error {
	// Determinar resultado (RadiantWin + IsRadiant)
	isWin := false
	if match.RadiantWin != nil && player.IsRadiant != nil {
		isWin = *match.RadiantWin == *player.IsRadiant
	}
	resultText := "❌ Derrota"
	resultColor := 0xe74c3c // Rojo
	if isWin {
		resultText = "✅ Victoria"
		resultColor = 0x2ecc71 // Verde
	}

	// Obtener nombre del jugador
	personaname := "Jugador"
	avatarURL := ""
	if profile != nil {
		if profile.Profile.Personaname != "" {
			personaname = profile.Profile.Personaname
		}
		avatarURL = profile.Profile.Avatarfull
	} else if player.Personaname != "" {
		personaname = player.Personaname
	}

	// Obtener nombre del héroe y URLs (datos locales desde dota package)
	heroName := b.dotaClient.GetHeroName(player.HeroID)
	heroImg := b.dotaClient.GetHeroImageURL(player.HeroID)
	if heroImg == "" {
		heroImg = dota.GetHeroImageURLStratz(player.HeroID)
	}
	heroIconURL := b.dotaClient.GetHeroIconURL(player.HeroID)
	if heroIconURL == "" {
		heroIconURL = dota.GetHeroImageURLStratz(player.HeroID)
	}

	gameModeName := b.dotaClient.GetGameModeName(match.GameMode)
	gameModeDisplayName := dota.GameModeDisplayName(gameModeName)

	if b.config.Debug {
		getLogger().Debugf("Match %d: RadiantScore=%d DireScore=%d GameMode=%d gameModeName=%q",
			match.MatchID, match.RadiantScore, match.DireScore, match.GameMode, gameModeName)
	}

	// W/L del héroe desde Stratz
	heroRecordText := "N/A"
	if b.stratzClient != nil && b.stratzClient.IsConfigured() {
		accountIDInt, _ := strconv.ParseInt(accountID, 10, 64)
		heroWL, err := b.stratzClient.GetPlayerWinLoss(accountIDInt, 20, player.HeroID)
		if err != nil {
			getLogger().Warnf("No se pudo obtener W/L del héroe %s para account_id %s: %v", heroName, accountID, err)
		} else if heroWL != nil {
			total := heroWL.Win + heroWL.Lose
			if total > 0 {
				winRate := float64(heroWL.Win) / float64(total) * 100
				heroRecordText = fmt.Sprintf("%d-%d (%.1f%%)", heroWL.Win, heroWL.Lose, winRate)
			}
		}
	}

	// Racha desde Stratz (para footer más abajo)
	var recentStratzMatches []dota.StratzMatch
	if b.stratzClient != nil && b.stratzClient.IsConfigured() {
		accountIDInt, _ := strconv.ParseInt(accountID, 10, 64)
		recentStratzMatches, _ = b.stratzClient.GetPlayerRecentMatches(accountIDInt, 10)
	}

	// Lane outcome: resumen por línea y victoria/derrota en fase de línea (si jugó una línea; jungle/roaming no se marca)
	lanePhaseLine, laneSummary := b.buildLaneOutcomeText(match, player)

	// Título: nombre [RANGO] - Victoria/Derrota (rango solo si está disponible)
	title := fmt.Sprintf("%s - %s", personaname, resultText)
	if profile != nil && profile.RankBracket != "" {
		title = fmt.Sprintf("%s [%s] - %s", personaname, profile.RankBracket, resultText)
	}

	description := fmt.Sprintf("**%s** | %s", heroName, gameModeDisplayName)
	if lanePhaseLine != "" {
		description += "\n" + lanePhaseLine
	}

	// Construir embed: Image = héroe (abajo); Thumbnail solo si hay avatar del jugador (nunca icono del héroe ahí)
	embed := &discordgo.MessageEmbed{
		Title:       title,
		Description: description,
		Color:       resultColor,
		Image: &discordgo.MessageEmbedImage{
			URL: heroImg,
		},
		Fields: []*discordgo.MessageEmbedField{
			{
				Name:   "K/D/A",
				Value:  fmt.Sprintf("%d/%d/%d (%.2f KDA)", player.Kills, player.Deaths, player.Assists, player.KDA),
				Inline: true,
			},
			{
				Name:   "Duración",
				Value:  dota.FormatDuration(match.Duration),
				Inline: true,
			},
			{
				Name:   "Nivel",
				Value:  strconv.Itoa(player.Level),
				Inline: true,
			},
			{
				Name:   "Score",
				Value:  fmt.Sprintf("Radiant %d - %d Dire", match.RadiantScore, match.DireScore),
				Inline: true,
			},
			{
				Name:   "GPM/XPM",
				Value:  fmt.Sprintf("%d / %d", player.GoldPerMin, player.XpPerMin),
				Inline: true,
			},
			{
				Name:   "Modo",
				Value:  gameModeDisplayName,
				Inline: true,
			},
			{
				Name:   fmt.Sprintf("Record con %s (últ. 20)", heroName),
				Value:  heroRecordText,
				Inline: false,
			},
		},
		Footer: &discordgo.MessageEmbedFooter{
			Text: fmt.Sprintf("Match ID: %d", match.MatchID),
		},
		URL: fmt.Sprintf("https://stratz.com/matches/%d", match.MatchID),
	}
	if avatarURL != "" {
		embed.Author = &discordgo.MessageEmbedAuthor{
			Name:    personaname,
			IconURL: avatarURL,
		}
		embed.Thumbnail = &discordgo.MessageEmbedThumbnail{URL: avatarURL}
	}

	// Lane/Rol (si Stratz los devuelve): campo Inline
	if player.Lane != "" || player.Role != "" {
		laneText := player.Lane
		if player.Role != "" {
			if laneText != "" {
				laneText += " / " + player.Role
			} else {
				laneText = player.Role
			}
		}
		embed.Fields = append(embed.Fields, &discordgo.MessageEmbedField{
			Name:   "Lane / Rol",
			Value:  laneText,
			Inline: true,
		})
	}

	// Resultado por línea (Stratz lane outcomes)
	if laneSummary != "" {
		embed.Fields = append(embed.Fields, &discordgo.MessageEmbedField{
			Name:   "Resultado por línea",
			Value:  laneSummary,
			Inline: false,
		})
	}

	// Agregar hero damage, tower damage, healing si están disponibles
	if player.HeroDamage > 0 || player.TowerDamage > 0 || player.HeroHealing > 0 {
		damageFields := []*discordgo.MessageEmbedField{}
		if player.HeroDamage > 0 {
			damageFields = append(damageFields, &discordgo.MessageEmbedField{
				Name:   "Hero Damage",
				Value:  fmt.Sprintf("%d", player.HeroDamage),
				Inline: true,
			})
		}
		if player.TowerDamage > 0 {
			damageFields = append(damageFields, &discordgo.MessageEmbedField{
				Name:   "Tower Damage",
				Value:  fmt.Sprintf("%d", player.TowerDamage),
				Inline: true,
			})
		}
		if player.HeroHealing > 0 {
			damageFields = append(damageFields, &discordgo.MessageEmbedField{
				Name:   "Hero Healing",
				Value:  fmt.Sprintf("%d", player.HeroHealing),
				Inline: true,
			})
		}
		embed.Fields = append(embed.Fields, damageFields...)
	}

	// Agregar rank si está disponible
	if profile != nil && profile.RankTier != nil {
		rankName := dota.GetRankName(profile.RankTier)
		embed.Fields = append(embed.Fields, &discordgo.MessageEmbedField{
			Name:   "Rank",
			Value:  rankName,
			Inline: true,
		})
	}

	// Agregar lista de jugadores con perfiles públicos verificados
	type VerifiedPlayer struct {
		Player     dota.Player
		HeroName   string
		PlayerName string
		WinLoss    *dota.WinLossResponse
	}

	var verifiedPlayers []VerifiedPlayer
	var playersToCheck []dota.Player

	// Primero filtrar por AccountID != 0
	for _, p := range match.Players {
		if p.AccountID != 0 {
			playersToCheck = append(playersToCheck, p)
		}
	}

	getLogger().Debugf("Verificando %d jugadores con AccountID != 0", len(playersToCheck))

	// Solo Stratz para W/L de jugadores
	if b.stratzClient != nil && b.stratzClient.IsConfigured() {
		var playerIDs []int64
		for _, p := range playersToCheck {
			playerIDs = append(playerIDs, int64(p.AccountID))
		}

		wlMap, errStratz := b.stratzClient.GetMultiplePlayersWinLoss(playerIDs, 20)
		if errStratz != nil {
			getLogger().Warnf("Error obteniendo W/L de Stratz: %v", errStratz)
		} else {
			for _, p := range playersToCheck {
				wl, ok := wlMap[int64(p.AccountID)]
				if !ok || wl == nil {
					continue
				}
				if wl.Win+wl.Lose == 0 {
					getLogger().Debugf("  ❌ Jugador con 0/0 W/L (AccountID %d), omitiendo", p.AccountID)
					continue
				}

				heroName := b.dotaClient.GetHeroName(p.HeroID)
				playerName := p.Personaname
				if playerName == "" {
					playerName = fmt.Sprintf("Jugador %d", p.AccountID)
				}

				getLogger().Debugf("  ✅ Stratz: AccountID %d, Nombre: '%s', Héroe: '%s', W/L: %d/%d",
					p.AccountID, playerName, heroName, wl.Win, wl.Lose)

				verifiedPlayers = append(verifiedPlayers, VerifiedPlayer{
					Player:     p,
					HeroName:   heroName,
					PlayerName: playerName,
					WinLoss:    wl,
				})
			}
		}
	}

	getLogger().Debugf("Total de jugadores verificados: %d de %d", len(verifiedPlayers), len(playersToCheck))

	if len(verifiedPlayers) > 0 {
		// Separar jugadores por equipo (Radiant: 0-4, Dire: 128-132)
		var radiantPlayers []VerifiedPlayer
		var direPlayers []VerifiedPlayer

		for _, vp := range verifiedPlayers {
			if vp.Player.PlayerSlot < 128 {
				radiantPlayers = append(radiantPlayers, vp)
			} else {
				direPlayers = append(direPlayers, vp)
			}
		}

		// Ordenar cada equipo por player_slot
		sortPlayers := func(players []VerifiedPlayer) {
			for i := 0; i < len(players)-1; i++ {
				for j := i + 1; j < len(players); j++ {
					if players[i].Player.PlayerSlot > players[j].Player.PlayerSlot {
						players[i], players[j] = players[j], players[i]
					}
				}
			}
		}

		sortPlayers(radiantPlayers)
		sortPlayers(direPlayers)

		// Un solo campo en el mensaje principal: Héroe | Jugador | W/L (antes del footer)
		var playersList strings.Builder
		const maxFieldLength = 1000
		addPlayersToList := func(players []VerifiedPlayer, teamName string) bool {
			if len(players) == 0 {
				return true
			}
			header := fmt.Sprintf("**%s**\n", teamName)
			if playersList.Len()+len(header) > maxFieldLength {
				return false
			}
			playersList.WriteString(header)
			for _, vp := range players {
				stratzURL := fmt.Sprintf("https://stratz.com/players/%d", vp.Player.AccountID)
				wlText := "N/A"
				if vp.WinLoss != nil {
					total := vp.WinLoss.Win + vp.WinLoss.Lose
					if total > 0 {
						winRate := float64(vp.WinLoss.Win) / float64(total) * 100
						wlText = fmt.Sprintf("%d/%d (%.1f%%)", vp.WinLoss.Win, vp.WinLoss.Lose, winRate)
					}
				}
				line := fmt.Sprintf("%s | [%s](%s) | W/L: %s\n", vp.HeroName, vp.PlayerName, stratzURL, wlText)
				if playersList.Len()+len(line) > maxFieldLength {
					playersList.WriteString("... y más")
					return false
				}
				playersList.WriteString(line)
			}
			return true
		}
		addPlayersToList(radiantPlayers, "☀️ Radiant")
		addPlayersToList(direPlayers, "🌙 Dire")
		if playersList.Len() > 0 {
			embed.Fields = append(embed.Fields, &discordgo.MessageEmbedField{
				Name:   "👥 Jugadores (Perfiles Públicos)",
				Value:  playersList.String(),
				Inline: false,
			})
		}
	}

	// Agregar racha en el footer (desde Stratz)
	if len(recentStratzMatches) > 0 {
		accountIDInt, _ := strconv.ParseInt(accountID, 10, 64)
		streak := dota.AnalyzeStreakFromStratzMatches(recentStratzMatches, accountIDInt)
		embed.Footer.Text = fmt.Sprintf("%s | Match ID: %d", streak.CurrentStreak, match.MatchID)
	}

	_, err := b.session.ChannelMessageSendEmbed(channelID, embed)
	return err
}
