package discord

import (
	"bytes"
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
		b.sendFollowup(s, i, "❌ Sistema de ranking no configurado (requiere POSTGRES_DSN).")
		return
	}

	// Sin opciones → semana actual como archivo PNG adjunto (sin MinIO, funciona local)
	if len(subcommand.Options) == 0 {
		b.sendRankingFile(s, i, func() ([]byte, string, error) {
			return b.rankingUpdater.WeekPNG()
		})
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
		b.sendRankingFile(s, i, func() ([]byte, string, error) {
			return b.rankingUpdater.MonthPNG(year, monthNum)
		})

	case "ultimas":
		b.sendRankingFile(s, i, func() ([]byte, string, error) {
			return b.rankingUpdater.YearPNG(b.config.BaseYear)
		})
	}
}

// sendRankingFile genera PNG y lo manda como archivo adjunto directo.
// No requiere MinIO — funciona en desarrollo local.
func (b *Bot) sendRankingFile(s *discordgo.Session, i *discordgo.InteractionCreate, renderFn func() ([]byte, string, error)) {
	imgBytes, label, err := renderFn()
	if err != nil {
		getLogger().Errorf("ranking render: %v", err)
		b.sendFollowup(s, i, fmt.Sprintf("❌ Error generando imagen: %v", err))
		return
	}

	_, err = s.FollowupMessageCreate(i.Interaction, true, &discordgo.WebhookParams{
		Content: fmt.Sprintf("📊 **%s**", label),
		Files: []*discordgo.File{
			{
				Name:        "ranking.png",
				ContentType: "image/png",
				Reader:      bytes.NewReader(imgBytes),
			},
		},
	})
	if err != nil {
		getLogger().Errorf("ranking followup file: %v", err)
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

	b.sendFollowup(s, i, fmt.Sprintf("✅ Jugador **%s** (Dota ID: %d) agregado al ranking.", displayName, dotaID))
	getLogger().Infof("admin register: dota_id=%d display_name=%s", dotaID, displayName)

	if b.backfillSvc != nil {
		go b.backfillSvc.RunForUser(dotaID)
	}
	if b.rankingUpdater != nil {
		go func() {
			if err := b.rankingUpdater.Refresh(time.Now()); err != nil {
				getLogger().Errorf("ranking refresh after admin register: %v", err)
			}
		}()
	}
}
