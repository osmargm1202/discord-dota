package discord

import (
	"fmt"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
)

var monthMap = map[string]int{
	"enero": 1, "febrero": 2, "marzo": 3, "abril": 4,
	"mayo": 5, "junio": 6, "julio": 7, "agosto": 8,
	"septiembre": 9, "octubre": 10, "noviembre": 11, "diciembre": 12,
}

func (b *Bot) handleRankingSlash(s *discordgo.Session, i *discordgo.InteractionCreate, subcommand *discordgo.ApplicationCommandInteractionDataOption) {
	if b.rankingUpdater == nil {
		b.sendFollowup(s, i, "❌ Sistema de ranking no configurado (requiere POSTGRES_DSN y MINIO_ENDPOINT).")
		return
	}

	if len(subcommand.Options) == 0 {
		if b.config.RankingChannelID != "" {
			b.sendFollowup(s, i, fmt.Sprintf("📊 Canal de ranking: <#%s>", b.config.RankingChannelID))
		} else {
			b.sendFollowup(s, i, "❌ RANKING_CHANNEL_ID no configurado.")
		}
		return
	}

	opt := subcommand.Options[0]
	switch opt.Name {
	case "mes":
		mesStr := strings.ToLower(strings.TrimSpace(opt.StringValue()))
		monthNum, ok := monthMap[mesStr]
		if !ok {
			b.sendFollowup(s, i, "❌ Mes inválido. Usa: enero, febrero, marzo, ... diciembre")
			return
		}
		year := time.Now().Year()
		embed, err := b.rankingUpdater.OnDemandMonth(year, monthNum)
		if err != nil {
			getLogger().Errorf("ranking mes %s: %v", mesStr, err)
			b.sendFollowup(s, i, fmt.Sprintf("❌ Error generando ranking: %v", err))
			return
		}
		b.sendFollowupEmbed(s, i, embed)

	case "ultimas":
		b.sendFollowup(s, i, "📊 Mostrando ranking del año completo...")
		embed, err := b.rankingUpdater.OnDemandLastN(b.config.BaseYear)
		if err != nil {
			getLogger().Errorf("ranking ultimas: %v", err)
			b.sendFollowup(s, i, fmt.Sprintf("❌ Error generando ranking: %v", err))
			return
		}
		b.sendFollowupEmbed(s, i, embed)
	}
}

func (b *Bot) handleAdminRegisterSlash(s *discordgo.Session, i *discordgo.InteractionCreate, subcommand *discordgo.ApplicationCommandInteractionDataOption) {
	if b.db == nil {
		b.sendFollowup(s, i, "❌ Base de datos no configurada.")
		return
	}

	var accountIDStr, nombre string
	for _, opt := range subcommand.Options {
		switch opt.Name {
		case "account_id":
			accountIDStr = opt.StringValue()
		case "nombre":
			nombre = opt.StringValue()
		}
	}

	if accountIDStr == "" {
		b.sendFollowup(s, i, "❌ account_id requerido.")
		return
	}

	var dotaID int64
	if _, err := fmt.Sscanf(accountIDStr, "%d", &dotaID); err != nil || dotaID <= 0 {
		b.sendFollowup(s, i, "❌ account_id debe ser un número válido.")
		return
	}

	displayName := nombre
	if displayName == "" {
		// Try to fetch name from Stratz
		profile, err := b.stratzClient.GetPlayerProfile(dotaID)
		if err == nil && profile != nil {
			displayName = profile.Name
		}
	}
	if displayName == "" {
		displayName = fmt.Sprintf("Player%d", dotaID)
	}

	if err := b.db.UpsertUser(nil, dotaID, &displayName); err != nil {
		b.sendFollowup(s, i, fmt.Sprintf("❌ Error guardando usuario: %v", err))
		return
	}

	b.sendFollowup(s, i, fmt.Sprintf("✅ Jugador **%s** (Dota ID: %d) agregado al ranking (sin Discord vinculado).", displayName, dotaID))
	getLogger().Infof("admin register: dota_id=%d display_name=%s", dotaID, displayName)

	// Trigger backfill for the new user in background
	if b.backfillSvc != nil {
		go b.backfillSvc.RunForUser(dotaID)
	}
	// Refresh ranking
	if b.rankingUpdater != nil {
		go func() {
			if err := b.rankingUpdater.Refresh(time.Now()); err != nil {
				getLogger().Errorf("ranking refresh after admin register: %v", err)
			}
		}()
	}
}
