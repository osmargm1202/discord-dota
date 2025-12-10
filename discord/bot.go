package discord

import (
	"dota-discord-bot/config"
	"dota-discord-bot/dota"
	"dota-discord-bot/storage"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/bwmarrin/discordgo"
)

type Bot struct {
	session     *discordgo.Session
	dotaClient  *dota.Client
	userStore   *storage.UserStore
	config      *config.Config
	searchCache map[string][]dota.SearchResponse // Cache temporal de búsquedas por usuario
}

func NewBot(cfg *config.Config, dotaClient *dota.Client, userStore *storage.UserStore) (*Bot, error) {
	if err := InitLogger(cfg.Debug); err != nil {
		return nil, fmt.Errorf("error inicializando logger: %w", err)
	}

	session, err := discordgo.New("Bot " + cfg.DiscordToken)
	if err != nil {
		return nil, fmt.Errorf("error creando sesión Discord: %w", err)
	}

	bot := &Bot{
		session:     session,
		dotaClient:  dotaClient,
		userStore:   userStore,
		config:      cfg,
		searchCache: make(map[string][]dota.SearchResponse),
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
					Name:        "stats",
					Description: "Ver estadísticas de partidas",
					Options: []*discordgo.ApplicationCommandOption{
						{
							Type:        discordgo.ApplicationCommandOptionUser,
							Name:        "usuario",
							Description: "Usuario del que ver estadísticas (opcional)",
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
					Name:        "rank",
					Description: "Ver ranking de jugadores registrados",
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
		getLogger().Errorf("Error respondiendo a interacción: %v", err)
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
	case "stats":
		b.handleStatsSlash(s, i, subcommand)
	case "rank":
		b.handleRankSlash(s, i)
	case "channel":
		b.handleChannelSlash(s, i, subcommand)
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

	results, err := b.dotaClient.SearchPlayers(query)
	if err != nil {
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

	// Verificar que el jugador existe
	profile, err := b.dotaClient.GetPlayerProfile(accountID)
	if err != nil {
		getLogger().Errorf("Error obteniendo perfil: %v", err)
		b.sendFollowup(s, i, fmt.Sprintf("❌ Error verificando jugador: %v", err))
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

	personaname := profile.Profile.Personaname
	if personaname == "" {
		personaname = "Jugador"
	}

	b.sendFollowup(s, i, fmt.Sprintf("✅ **%s** (Discord) asociado con **%s** (Dota 2)\nID de Dota: %s", discordUsername, personaname, accountID))
	getLogger().Infof("Usuario Discord %s (%s) registrado con account_id %s", userID, discordUsername, accountID)
}

func (b *Bot) handleStatsSlash(s *discordgo.Session, i *discordgo.InteractionCreate, subcommand *discordgo.ApplicationCommandInteractionDataOption) {
	var targetUserID string

	// Verificar si hay usuario mencionado
	if subcommand.Options != nil {
		for _, option := range subcommand.Options {
			if option.Name == "usuario" {
				targetUserID = option.UserValue(s).ID
				break
			}
		}
	}

	// Si no hay usuario, usar el que ejecutó el comando
	if targetUserID == "" {
		if i.Member != nil {
			targetUserID = i.Member.User.ID
		} else if i.User != nil {
			targetUserID = i.User.ID
		} else {
			b.sendFollowup(s, i, "❌ No se pudo identificar al usuario")
			return
		}
	}

	accountID, ok := b.userStore.Get(targetUserID)
	if !ok {
		b.sendFollowup(s, i, "❌ Usuario no registrado. Usa `/dota register account_id:<account_id>` o `/dota search nombre:<nombre>`")
		return
	}

	// Obtener partidas recientes
	matches, err := b.dotaClient.GetRecentMatches(accountID)
	if err != nil {
		getLogger().Errorf("Error obteniendo partidas: %v", err)
		b.sendFollowup(s, i, fmt.Sprintf("❌ Error obteniendo partidas: %v", err))
		return
	}

	if len(matches) == 0 {
		b.sendFollowup(s, i, "❌ No se encontraron partidas recientes")
		return
	}

	// Analizar últimas 20 partidas
	limit := 20
	if len(matches) < limit {
		limit = len(matches)
	}
	recentMatches := matches[:limit]

	streak := b.dotaClient.AnalyzeStreak(recentMatches)

	// Obtener perfil para nombre
	profile, _ := b.dotaClient.GetPlayerProfile(accountID)
	personaname := "Jugador"
	if profile != nil && profile.Profile.Personaname != "" {
		personaname = profile.Profile.Personaname
	}

	// Obtener nombre del usuario de Discord
	discordUsername := "Usuario"
	if i.Member != nil && i.Member.User != nil {
		discordUsername = i.Member.User.Username
	} else if i.User != nil {
		discordUsername = i.User.Username
	}

	// Obtener W/L de las últimas 20 partidas (endpoint específico)
	wl, err := b.dotaClient.GetWinLoss(accountID, limit)
	if err != nil {
		getLogger().Warnf("No se pudo obtener W/L: %v", err)
	}
	winLossText := "N/A"
	winRateText := "N/A"
	if wl != nil {
		total := wl.Win + wl.Lose
		if total > 0 {
			winLossText = fmt.Sprintf("%d / %d", wl.Win, wl.Lose)
			winRateText = fmt.Sprintf("%.1f%%", float64(wl.Win)/float64(total)*100)
		}
	}

	// Obtener detalles de la partida más reciente para enriquecer el embed
	latestMatch := recentMatches[0]
	matchDetails, err := b.dotaClient.GetMatchDetails(latestMatch.MatchID)
	var playerInMatch *dota.Player
	if err == nil && matchDetails != nil {
		playerInMatch, _ = b.dotaClient.FindPlayerInMatch(matchDetails, accountID)
	}

	heroName := b.dotaClient.GetHeroName(latestMatch.HeroID)
	heroImg := b.dotaClient.GetHeroImageURL(latestMatch.HeroID)
	gameMode := "Mode desconocido"
	lobbyType := "Lobby desconocido"
	durationText := "N/A"
	scoreText := "N/A"
	if matchDetails != nil {
		gameMode = b.dotaClient.GetGameModeName(matchDetails.GameMode)
		lobbyType = b.dotaClient.GetLobbyTypeName(matchDetails.LobbyType)
		durationText = dota.FormatDuration(matchDetails.Duration)
		scoreText = fmt.Sprintf("Radiant %d - %d Dire", matchDetails.RadiantScore, matchDetails.DireScore)
	}

	matchURL := fmt.Sprintf("https://www.dotabuff.com/matches/%d", latestMatch.MatchID)

	deaths := latestMatch.Deaths
	if deaths == 0 {
		deaths = 1
	}
	kdaText := fmt.Sprintf("%d/%d/%d (%.2f KDA)", latestMatch.Kills, latestMatch.Deaths, latestMatch.Assists, float64(latestMatch.Kills+latestMatch.Assists)/float64(deaths))

	rankText := "Unranked"
	levelText := "N/A"
	gpmxpm := "N/A"
	damages := "N/A"
	if playerInMatch != nil {
		rankText = dota.GetRankName(playerInMatch.RankTier)
		levelText = strconv.Itoa(playerInMatch.Level)
		gpmxpm = fmt.Sprintf("%d / %d", playerInMatch.GoldPerMin, playerInMatch.XpPerMin)
		damages = fmt.Sprintf("%d / %d", playerInMatch.HeroDamage, playerInMatch.TowerDamage)
	}

	// Construir embed
	embed := &discordgo.MessageEmbed{
		Title:       fmt.Sprintf("📊 [%s](%s)", heroName, matchURL),
		Description: fmt.Sprintf("Últimas %d partidas\nDiscord: %s\nJugador: %s", limit, discordUsername, personaname),
		Color:       0x3498db,
		Thumbnail: &discordgo.MessageEmbedThumbnail{
			URL: heroImg,
		},
		Fields: []*discordgo.MessageEmbedField{
			{
				Name:   "Héroe",
				Value:  heroName,
				Inline: true,
			},
			{
				Name:   "Modo",
				Value:  fmt.Sprintf("%s (Lobby %s)", gameMode, lobbyType),
				Inline: true,
			},
			{
				Name:   "K/D/A",
				Value:  kdaText,
				Inline: true,
			},
			{
				Name:   "Duración",
				Value:  durationText,
				Inline: true,
			},
			{
				Name:   "Nivel",
				Value:  levelText,
				Inline: true,
			},
			{
				Name:   "Score",
				Value:  scoreText,
				Inline: true,
			},
			{
				Name:   "GPM / XPM",
				Value:  gpmxpm,
				Inline: true,
			},
			{
				Name:   "Hero Damage / Tower Damage",
				Value:  damages,
				Inline: true,
			},
			{
				Name:   "Rank",
				Value:  rankText,
				Inline: true,
			},
			{
				Name:   "W/L (últimas 20)",
				Value:  fmt.Sprintf("%s (Winrate %s)", winLossText, winRateText),
				Inline: false,
			},
			{
				Name:   "🎯 Racha actual",
				Value:  streak.CurrentStreak,
				Inline: false,
			},
		},
		Footer: &discordgo.MessageEmbedFooter{
			Text: fmt.Sprintf("Match ID: %d", latestMatch.MatchID),
		},
	}

	b.sendFollowupEmbed(s, i, embed)
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

func (b *Bot) handleRankSlash(s *discordgo.Session, i *discordgo.InteractionCreate) {
	// Obtener todos los usuarios registrados
	allUsers := b.userStore.GetAll()
	
	if len(allUsers) == 0 {
		b.sendFollowup(s, i, "❌ No hay usuarios registrados. Usa `/dota register` para registrar jugadores.")
		return
	}

	// Estructura para almacenar información de ranking
	type RankEntry struct {
		DiscordID   string
		DiscordName string
		Personaname string
		MMR         float64
		RankTier    *int
		AccountID   string
		AvatarURL   string
	}

	var entries []RankEntry

	// Obtener información de cada usuario
	for discordID, accountID := range allUsers {
		// Obtener perfil del jugador
		profile, err := b.dotaClient.GetPlayerProfile(accountID)
		if err != nil {
			getLogger().Warnf("Error obteniendo perfil para account_id %s: %v", accountID, err)
			continue
		}

		// Obtener nombre de Discord
		discordName := "Usuario desconocido"
		user, err := s.User(discordID)
		if err == nil && user != nil {
			discordName = user.Username
		}

		// Extraer información
		personaname := profile.Profile.Personaname
		if personaname == "" {
			personaname = "Jugador"
		}

		mmr := 0.0
		if profile.ComputedMMR != nil {
			mmr = *profile.ComputedMMR
		}

		// Obtener avatar (preferir Steam, fallback a Discord)
		avatarURL := profile.Profile.Avatarfull
		if avatarURL == "" {
			avatarURL = profile.Profile.Avatar
		}
		// Si no hay avatar de Steam, usar avatar de Discord
		if avatarURL == "" {
			if user != nil && user.Avatar != "" {
				avatarURL = user.AvatarURL("")
			}
		}

		entries = append(entries, RankEntry{
			DiscordID:   discordID,
			DiscordName: discordName,
			Personaname: personaname,
			MMR:         mmr,
			RankTier:    profile.RankTier,
			AccountID:   accountID,
			AvatarURL:   avatarURL,
		})
	}

	if len(entries) == 0 {
		b.sendFollowup(s, i, "❌ No se pudo obtener información de los usuarios registrados.")
		return
	}

	// Ordenar: primero por RankTier (mayor es mejor), luego por MMR (mayor es mejor)
	// Usamos sort.Slice para ordenar de menor a mayor (último al primero)
	for i := 0; i < len(entries)-1; i++ {
		for j := i + 1; j < len(entries); j++ {
			// Comparar RankTier primero
			rankI := 0
			rankJ := 0
			if entries[i].RankTier != nil {
				rankI = *entries[i].RankTier
			}
			if entries[j].RankTier != nil {
				rankJ = *entries[j].RankTier
			}

			// Si tienen el mismo rango, comparar por MMR
			if rankI == rankJ {
				if entries[i].MMR < entries[j].MMR {
					entries[i], entries[j] = entries[j], entries[i]
				}
			} else if rankI < rankJ {
				entries[i], entries[j] = entries[j], entries[i]
			}
		}
	}

	// Construir múltiples embeds con imágenes de jugadores
	// Discord permite hasta 10 embeds por mensaje
	var embeds []*discordgo.MessageEmbed

	// Embed principal con título
	mainEmbed := &discordgo.MessageEmbed{
		Title:       "🏆 Ranking de Jugadores",
		Description: fmt.Sprintf("Total de jugadores: %d\nOrdenado por rango y MMR (menor a mayor)\n\n", len(entries)),
		Color:       0xFFD700, // Color dorado
	}
	embeds = append(embeds, mainEmbed)

	// Discord limita a 10 embeds por mensaje, así que limitamos a 9 jugadores (1 para el título)
	maxEntries := len(entries)
	if maxEntries > 9 {
		maxEntries = 9
	}

	for idx := 0; idx < maxEntries; idx++ {
		entry := entries[idx]
		position := idx + 1
		
		// Obtener nombre del rango
		rankName := "Unranked"
		if entry.RankTier != nil {
			rankName = dota.GetRankName(entry.RankTier)
		}

		// Formatear MMR
		mmrText := "N/A"
		if entry.MMR > 0 {
			mmrText = fmt.Sprintf("%.0f", entry.MMR)
		}

		// Emoji según posición
		emoji := "🥉"
		if position == 1 {
			emoji = "🥇"
		} else if position == 2 {
			emoji = "🥈"
		} else if position == 3 {
			emoji = "🥉"
		}

		// Construir descripción del jugador
		var desc strings.Builder
		desc.WriteString(fmt.Sprintf("%s **#%d**\n", emoji, position))
		desc.WriteString(fmt.Sprintf("**%s** (@%s)\n", entry.Personaname, entry.DiscordName))
		desc.WriteString(fmt.Sprintf("MMR: **%s**\n", mmrText))
		desc.WriteString(fmt.Sprintf("Rango: **%s**", rankName))

		// Crear embed para este jugador
		playerEmbed := &discordgo.MessageEmbed{
			Title:       fmt.Sprintf("#%d - %s", position, entry.Personaname),
			Description: desc.String(),
			Color:       0x3498db,
			Thumbnail: &discordgo.MessageEmbedThumbnail{
				URL: entry.AvatarURL,
			},
		}

		embeds = append(embeds, playerEmbed)
	}

	if len(entries) > 9 {
		// Agregar embed final con información de jugadores restantes
		remainingEmbed := &discordgo.MessageEmbed{
			Description: fmt.Sprintf("... y %d jugador(es) más", len(entries)-9),
			Color:       0x95a5a6,
		}
		embeds = append(embeds, remainingEmbed)
	}

	// Enviar múltiples embeds
	_, err := s.FollowupMessageCreate(i.Interaction, true, &discordgo.WebhookParams{
		Embeds: embeds,
	})
	if err != nil {
		getLogger().Errorf("Error enviando embeds de ranking: %v", err)
		b.sendFollowup(s, i, "❌ Error mostrando ranking")
	}
}

func (b *Bot) handleHelpSlash(s *discordgo.Session, i *discordgo.InteractionCreate) {
	embed := &discordgo.MessageEmbed{
		Title:       "🎮 Comandos del Bot de Dota 2",
		Description: "Comandos disponibles. Usa register para asociar un Discord ID con un ID de Dota y que stats funcione sin parámetros.",
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
				Name:   "/dota stats [usuario:<@usuario>]",
				Value:  "Estadísticas de las últimas 20 partidas del usuario registrado (victorias/derrotas/racha).\n**Ejemplos:** `/dota stats` · `/dota stats usuario:@amigo`",
				Inline: false,
			},
			{
				Name:   "/dota rank",
				Value:  "Muestra el ranking de todos los jugadores registrados, ordenados por MMR y rango (del menor al mayor).",
				Inline: false,
			},
			{
				Name:   "/dota channel canal:<#canal>",
				Value:  "Configura el canal para notificaciones automáticas de nuevas partidas.\n**Ejemplo:** `/dota channel canal:#dota-updates`",
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

	// Verificar que el jugador existe
	profile, err := b.dotaClient.GetPlayerProfile(accountID)
	if err != nil {
		getLogger().Errorf("Error obteniendo perfil: %v", err)
		s.ChannelMessageSend(m.ChannelID, fmt.Sprintf("❌ Error verificando jugador: %v", err))
		return
	}

	// Registrar usuario
	if err := b.userStore.Set(m.Author.ID, accountID); err != nil {
		getLogger().Errorf("Error guardando usuario: %v", err)
		s.ChannelMessageSend(m.ChannelID, "❌ Error guardando registro")
		return
	}

	personaname := profile.Profile.Personaname
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

	results, err := b.dotaClient.SearchPlayers(query)
	if err != nil {
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

func (b *Bot) handleStats(s *discordgo.Session, m *discordgo.MessageCreate, args []string) {
	var targetUserID string

	// Verificar si hay mención
	if len(m.Mentions) > 0 {
		targetUserID = m.Mentions[0].ID
	} else {
		targetUserID = m.Author.ID
	}

	accountID, ok := b.userStore.Get(targetUserID)
	if !ok {
		s.ChannelMessageSend(m.ChannelID, "❌ Usuario no registrado. Usa `/dota register <account_id>` o `/dota search <nombre>`")
		return
	}

	// Obtener partidas recientes
	matches, err := b.dotaClient.GetRecentMatches(accountID)
	if err != nil {
		getLogger().Errorf("Error obteniendo partidas: %v", err)
		s.ChannelMessageSend(m.ChannelID, fmt.Sprintf("❌ Error obteniendo partidas: %v", err))
		return
	}

	if len(matches) == 0 {
		s.ChannelMessageSend(m.ChannelID, "❌ No se encontraron partidas recientes")
		return
	}

	// Analizar últimas 20 partidas
	limit := 20
	if len(matches) < limit {
		limit = len(matches)
	}
	recentMatches := matches[:limit]

	streak := b.dotaClient.AnalyzeStreak(recentMatches)

	// Obtener perfil para nombre
	profile, _ := b.dotaClient.GetPlayerProfile(accountID)
	personaname := "Jugador"
	if profile != nil && profile.Profile.Personaname != "" {
		personaname = profile.Profile.Personaname
	}

	// Construir embed
	embed := &discordgo.MessageEmbed{
		Title:       fmt.Sprintf("📊 Estadísticas de %s", personaname),
		Description: fmt.Sprintf("Últimas %d partidas\nDiscord: %s", limit, m.Author.Username),
		Color:       0x3498db,
		Fields: []*discordgo.MessageEmbedField{
			{
				Name:   "✅ Victorias",
				Value:  strconv.Itoa(streak.Wins),
				Inline: true,
			},
			{
				Name:   "❌ Derrotas",
				Value:  strconv.Itoa(streak.Losses),
				Inline: true,
			},
			{
				Name:   "🎯 Racha actual",
				Value:  streak.CurrentStreak,
				Inline: false,
			},
		},
		Footer: &discordgo.MessageEmbedFooter{
			Text: fmt.Sprintf("Winrate: %.1f%%", float64(streak.Wins)/float64(limit)*100),
		},
	}

	s.ChannelMessageSendEmbed(m.ChannelID, embed)
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
				Name:   "/dota stats [@usuario]",
				Value:  "Ver estadísticas de las últimas 20 partidas (tuyas o de otro usuario)",
				Inline: false,
			},
			{
				Name:   "/dota channel <#canal>",
				Value:  "Configurar canal para notificaciones automáticas",
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
				Name:   "4️⃣ /dota stats [usuario:@usuario]",
				Value:  "Muestra estadísticas de las últimas 20 partidas del usuario registrado (victorias/derrotas/racha).\n**Ejemplos:** `/dota stats` · `/dota stats usuario:@amigo`",
				Inline: false,
			},
			{
				Name:   "5️⃣ /dota channel canal:<#canal>",
				Value:  "Configura el canal para notificaciones automáticas de nuevas partidas.\n**Ejemplo:** `/dota channel canal:#dota-updates`",
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

	for discordID, accountID := range users {
		// Obtener última partida conocida
		lastMatchID, hasLastMatch := b.userStore.GetLastMatch(discordID)

		// Obtener partidas recientes
		matches, err := b.dotaClient.GetRecentMatches(accountID)
		if err != nil {
			getLogger().Errorf("Error obteniendo partidas para %s: %v", accountID, err)
			continue
		}

		if len(matches) == 0 {
			continue
		}

		// La primera partida es la más reciente
		latestMatch := matches[0]

		// Verificar si es una nueva partida
		if hasLastMatch && latestMatch.MatchID == lastMatchID {
			continue // No hay nueva partida
		}

		// Nueva partida detectada
		getLogger().Infof("Nueva partida detectada para %s: %d", accountID, latestMatch.MatchID)

		// Obtener detalles completos de la partida
		matchDetails, err := b.dotaClient.GetMatchDetails(latestMatch.MatchID)
		if err != nil {
			getLogger().Errorf("Error obteniendo detalles de partida %d: %v", latestMatch.MatchID, err)
			continue
		}

		// Encontrar al jugador en la partida
		player, err := b.dotaClient.FindPlayerInMatch(matchDetails, accountID)
		if err != nil {
			getLogger().Errorf("Error encontrando jugador en partida: %v", err)
			continue
		}

		// Obtener perfil del jugador
		profile, err := b.dotaClient.GetPlayerProfile(accountID)
		if err != nil {
			getLogger().Errorf("Error obteniendo perfil: %v", err)
			// Continuar sin perfil
		}

		// Publicar notificación
		if err := b.sendMatchNotification(channelID, matchDetails, player, profile, accountID); err != nil {
			getLogger().Errorf("Error enviando notificación: %v", err)
			continue
		}

		// Actualizar última partida conocida
		if err := b.userStore.SetLastMatch(discordID, latestMatch.MatchID); err != nil {
			getLogger().Errorf("Error guardando última partida: %v", err)
		}

		// Rate limiting: esperar entre notificaciones
		time.Sleep(2 * time.Second)
	}

	return nil
}

func (b *Bot) sendMatchNotification(channelID string, match *dota.MatchResponse, player *dota.Player, profile *dota.PlayersResponse, accountID string) error {
	// Determinar resultado
	isWin := b.dotaClient.IsWinFromPlayer(*player, match.RadiantWin)
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

	// Obtener nombre del héroe
	heroName := b.dotaClient.GetHeroName(player.HeroID)

	// Obtener modo de juego
	gameModeName := b.dotaClient.GetGameModeName(match.GameMode)
	lobbyTypeName := b.dotaClient.GetLobbyTypeName(match.LobbyType)

	// Calcular racha (se usará más abajo)
	var recentMatches []dota.PlayerRecentMatch
	recentMatches, err := b.dotaClient.GetRecentMatches(accountID)

	// Construir embed
	embed := &discordgo.MessageEmbed{
		Title:       fmt.Sprintf("%s - %s", personaname, resultText),
		Description: fmt.Sprintf("**%s** | %s", heroName, gameModeName),
		Color:       resultColor,
		Thumbnail: &discordgo.MessageEmbedThumbnail{
			URL: avatarURL,
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
				Value:  fmt.Sprintf("%s (%s)", gameModeName, lobbyTypeName),
				Inline: true,
			},
		},
		Footer: &discordgo.MessageEmbedFooter{
			Text: fmt.Sprintf("Match ID: %d", match.MatchID),
		},
		URL: fmt.Sprintf("https://www.dotabuff.com/matches/%d", match.MatchID),
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
		Profile    *dota.PlayersResponse
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

	getLogger().Debugf("Verificando %d jugadores con AccountID != 0 para perfil público (en paralelo)", len(playersToCheck))

	// Usar goroutines para verificar jugadores en paralelo
	var wg sync.WaitGroup
	var mu sync.Mutex
	verifiedPlayersChan := make(chan VerifiedPlayer, len(playersToCheck))

	// Verificar cada jugador en una goroutine separada
	for _, p := range playersToCheck {
		wg.Add(1)
		go func(player dota.Player) {
			defer wg.Done()

			accountIDStr := strconv.Itoa(player.AccountID)
			getLogger().Debugf("Verificando jugador AccountID: %d, HeroID: %d, Personaname: '%s'", player.AccountID, player.HeroID, player.Personaname)

			// Intentar obtener el perfil del jugador
			playerProfile, err := b.dotaClient.GetPlayerProfile(accountIDStr)
			if err != nil {
				getLogger().Debugf("  ❌ No se pudo obtener perfil para AccountID %d: %v", player.AccountID, err)
				return
			}

			// Verificar que el perfil tenga datos válidos
			if playerProfile == nil {
				getLogger().Debugf("  ❌ Perfil nulo para AccountID %d", player.AccountID)
				return
			}

			// Verificar que tenga información del perfil
			if playerProfile.Profile.AccountID == 0 {
				getLogger().Debugf("  ❌ Perfil sin AccountID válido para AccountID %d", player.AccountID)
				return
			}

			// Intentar obtener W/L de las últimas 20 partidas para verificar que el perfil es completamente público
			wl, err := b.dotaClient.GetWinLoss(accountIDStr, 20)
			if err != nil {
				getLogger().Debugf("  ❌ No se pudo obtener W/L para AccountID %d (perfil no completamente público): %v", player.AccountID, err)
				return
			}

			if wl == nil {
				getLogger().Debugf("  ❌ W/L nulo para AccountID %d (perfil no completamente público)", player.AccountID)
				return
			}

			// Obtener nombre del héroe
			heroName := b.dotaClient.GetHeroName(player.HeroID)
			
			// Obtener nombre del jugador (preferir del perfil, luego del player, luego AccountID)
			playerName := ""
			if playerProfile.Profile.Personaname != "" {
				playerName = playerProfile.Profile.Personaname
			} else if player.Personaname != "" {
				playerName = player.Personaname
			} else {
				playerName = fmt.Sprintf("Jugador %d", player.AccountID)
			}

			getLogger().Debugf("  ✅ Perfil público verificado: AccountID %d, Nombre: '%s', Héroe: '%s', W/L: %d/%d", 
				player.AccountID, playerName, heroName, wl.Win, wl.Lose)

			// Enviar jugador verificado al channel
			verifiedPlayersChan <- VerifiedPlayer{
				Player:     player,
				HeroName:   heroName,
				PlayerName: playerName,
				Profile:    playerProfile,
				WinLoss:    wl,
			}
		}(p)
	}

	// Esperar a que todas las goroutines terminen
	go func() {
		wg.Wait()
		close(verifiedPlayersChan)
	}()

	// Recopilar resultados del channel
	for vp := range verifiedPlayersChan {
		mu.Lock()
		verifiedPlayers = append(verifiedPlayers, vp)
		mu.Unlock()
	}

	getLogger().Debugf("Total de jugadores con perfil público verificado: %d de %d", len(verifiedPlayers), len(playersToCheck))

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

		var playersList strings.Builder
		const maxFieldLength = 1000 // Dejar margen para el límite de 1024 caracteres

		// Función helper para agregar jugadores a la lista
		addPlayersToList := func(players []VerifiedPlayer, teamName string) bool {
			if len(players) == 0 {
				return true
			}

			// Agregar encabezado del equipo
			header := fmt.Sprintf("**%s**\n", teamName)
			if playersList.Len()+len(header) > maxFieldLength {
				return false
			}
			playersList.WriteString(header)

			for _, vp := range players {
				dotabuffURL := fmt.Sprintf("https://www.dotabuff.com/players/%d", vp.Player.AccountID)
				
				// Formatear W/L
				wlText := "N/A"
				if vp.WinLoss != nil {
					total := vp.WinLoss.Win + vp.WinLoss.Lose
					if total > 0 {
						winRate := float64(vp.WinLoss.Win) / float64(total) * 100
						wlText = fmt.Sprintf("%d/%d (%.1f%%)", vp.WinLoss.Win, vp.WinLoss.Lose, winRate)
					} else {
						wlText = "0/0"
					}
				}
				
				line := fmt.Sprintf("%s | [%s](%s) | ID: %d | W/L: %s\n", 
					vp.HeroName, vp.PlayerName, dotabuffURL, vp.Player.AccountID, wlText)

				// Verificar si agregar esta línea excedería el límite
				if playersList.Len()+len(line) > maxFieldLength {
					playersList.WriteString("... y más")
					return false
				}

				playersList.WriteString(line)
			}

			// Agregar separador entre equipos si hay Dire después
			if len(direPlayers) > 0 {
				separator := "\n"
				if playersList.Len()+len(separator) > maxFieldLength {
					return false
				}
				playersList.WriteString(separator)
			}

			return true
		}

		// Agregar primero Radiant, luego Dire
		if !addPlayersToList(radiantPlayers, "☀️ Radiant") {
			// Si se excedió el límite, no agregar Dire
		} else {
			addPlayersToList(direPlayers, "🌙 Dire")
		}

		if playersList.Len() > 0 {
			embed.Fields = append(embed.Fields, &discordgo.MessageEmbedField{
				Name:   "👥 Jugadores (Perfiles Públicos)",
				Value:  playersList.String(),
				Inline: false,
			})
		}
	}

	// Agregar racha en el footer
	if err == nil && len(recentMatches) > 0 {
		limit := 10
		if len(recentMatches) < limit {
			limit = len(recentMatches)
		}
		streak := b.dotaClient.AnalyzeStreak(recentMatches[:limit])
		embed.Footer.Text = fmt.Sprintf("%s | Match ID: %d", streak.CurrentStreak, match.MatchID)
	}

	_, err = b.session.ChannelMessageSendEmbed(channelID, embed)
	return err
}

