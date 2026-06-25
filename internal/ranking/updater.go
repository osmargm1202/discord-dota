package ranking

import (
	"bytes"
	"context"
	"dota-discord-bot/internal/db"
	minioclient "dota-discord-bot/internal/minio"
	"fmt"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/sirupsen/logrus"
)

// Updater orchestrates: calculate → render PNG → upload MinIO → edit Discord messages.
type Updater struct {
	db        *db.DB
	calc      *Calculator
	gen       *ImageGenerator
	minio     *minioclient.Client
	session   *discordgo.Session
	channelID string
	baseYear  int
}

func NewUpdater(
	database *db.DB,
	minio *minioclient.Client,
	session *discordgo.Session,
	channelID string,
	baseYear int,
) *Updater {
	return &Updater{
		db:        database,
		calc:      NewCalculator(database),
		gen:       NewImageGenerator(),
		minio:     minio,
		session:   session,
		channelID: channelID,
		baseYear:  baseYear,
	}
}

// InitChannel sends the 3 initial ranking messages if they don't exist yet.
func (u *Updater) InitChannel() error {
	if u.channelID == "" {
		return nil
	}
	types := []string{"individual", "team2", "team3"}
	for _, t := range types {
		_, msgID, err := u.db.GetRankingMessage(t)
		if err != nil {
			return fmt.Errorf("get ranking message %s: %w", t, err)
		}
		if msgID != "" {
			continue
		}
		msg, err := u.session.ChannelMessageSend(u.channelID, "*(ranking se actualizará cuando haya partidas registradas)*")
		if err != nil {
			return fmt.Errorf("send initial message %s: %w", t, err)
		}
		if err := u.db.SetRankingMessage(t, u.channelID, msg.ID); err != nil {
			return fmt.Errorf("save ranking message %s: %w", t, err)
		}
		logrus.Infof("ranking: created pinned message %s: %s", t, msg.ID)
	}
	// No llamar Refresh aquí — se activa con la primera partida real
	return nil
}

// Refresh recalculates, regenerates PNGs, and edits the 3 Discord messages as file attachments.
// MinIO upload is optional (archiving); the pinned message uses Discord attachment directly.
func (u *Updater) Refresh(now time.Time) error {
	if u.channelID == "" {
		return nil
	}
	weekStart, weekEnd := WeekBounds(now)
	weekLabel := fmt.Sprintf("Semana: %s → %s",
		weekStart.Format("Mon Jan 02"),
		weekEnd.AddDate(0, 0, -1).Format("Mon Jan 02, 2006"))

	year, week := now.ISOWeek()

	type imageJob struct {
		msgType string
		key     string
		render  func() ([]byte, error)
	}

	jobs := []imageJob{
		{
			msgType: "individual",
			key:     fmt.Sprintf("ranking-individual-%d-W%02d.png", year, week),
			render: func() ([]byte, error) {
				rows, err := u.calc.IndividualRanking(weekStart, weekEnd)
				if err != nil {
					return nil, err
				}
				return u.gen.RenderIndividual(rows, weekLabel)
			},
		},
		{
			msgType: "team2",
			key:     fmt.Sprintf("ranking-team2-%d-W%02d.png", year, week),
			render: func() ([]byte, error) {
				rows, err := u.calc.Team2Ranking(weekStart, weekEnd)
				if err != nil {
					return nil, err
				}
				return u.gen.RenderTeam2(rows, weekLabel)
			},
		},
		{
			msgType: "team3",
			key:     fmt.Sprintf("ranking-team3-%d-W%02d.png", year, week),
			render: func() ([]byte, error) {
				rows, err := u.calc.Team3Ranking(weekStart, weekEnd)
				if err != nil {
					return nil, err
				}
				return u.gen.RenderTeam3(rows, weekLabel)
			},
		},
	}

	// Find nearest week with data (up to 4 weeks back)
	effectiveStart, effectiveEnd := weekStart, weekEnd
	effectiveYear, effectiveWeek := year, week
	for offset := 0; offset <= 4; offset++ {
		t := now.AddDate(0, 0, -7*offset)
		ws, we := WeekBounds(t)
		rows, _ := u.calc.IndividualRanking(ws, we)
		if len(rows) > 0 {
			effectiveStart, effectiveEnd = ws, we
			effectiveYear, effectiveWeek = t.ISOWeek()
			break
		}
		if offset == 4 {
			logrus.Infof("ranking: no data in last 4 weeks, skipping refresh")
			return nil
		}
	}
	// Rebuild jobs with effective week bounds
	weekLabel = fmt.Sprintf("Semana %d-W%02d: %s → %s",
		effectiveYear, effectiveWeek,
		effectiveStart.Format("Mon Jan 02"),
		effectiveEnd.AddDate(0, 0, -1).Format("Mon Jan 02, 2006"))
	jobs = []imageJob{
		{
			msgType: "individual",
			key:     fmt.Sprintf("ranking-individual-%d-W%02d.png", effectiveYear, effectiveWeek),
			render: func() ([]byte, error) {
				rows, err := u.calc.IndividualRanking(effectiveStart, effectiveEnd)
				if err != nil {
					return nil, err
				}
				return u.gen.RenderIndividual(rows, weekLabel)
			},
		},
		{
			msgType: "team2",
			key:     fmt.Sprintf("ranking-team2-%d-W%02d.png", effectiveYear, effectiveWeek),
			render: func() ([]byte, error) {
				rows, err := u.calc.Team2Ranking(effectiveStart, effectiveEnd)
				if err != nil {
					return nil, err
				}
				return u.gen.RenderTeam2(rows, weekLabel)
			},
		},
		{
			msgType: "team3",
			key:     fmt.Sprintf("ranking-team3-%d-W%02d.png", effectiveYear, effectiveWeek),
			render: func() ([]byte, error) {
				rows, err := u.calc.Team3Ranking(effectiveStart, effectiveEnd)
				if err != nil {
					return nil, err
				}
				return u.gen.RenderTeam3(rows, weekLabel)
			},
		},
	}

	ctx := context.Background()
	for _, job := range jobs {
		imgBytes, err := job.render()
		if err != nil {
			logrus.Errorf("ranking: render %s: %v", job.msgType, err)
			continue
		}

		// Upload to MinIO for archiving (optional — skip if not configured)
		if u.minio != nil {
			if _, merr := u.minio.Upload(ctx, job.key, imgBytes); merr != nil {
				logrus.Warnf("ranking: minio upload %s: %v (continuando sin MinIO)", job.msgType, merr)
			}
		}

		_, msgID, err := u.db.GetRankingMessage(job.msgType)
		if err != nil || msgID == "" {
			logrus.Warnf("ranking: no message ID for %s", job.msgType)
			continue
		}

		// Edit pinned message with file attachment — works without public MinIO URL
		filename := fmt.Sprintf("ranking-%s.png", job.msgType)
		emptyStr := ""
		_, err = u.session.ChannelMessageEditComplex(&discordgo.MessageEdit{
			ID:      msgID,
			Channel: u.channelID,
			Content: &emptyStr,
			Files: []*discordgo.File{
				{
					Name:        filename,
					ContentType: "image/png",
					Reader:      bytes.NewReader(imgBytes),
				},
			},
		})
		if err != nil {
			logrus.Errorf("ranking: edit message %s (%s): %v", job.msgType, msgID, err)
		}
	}

	logrus.Infof("ranking: refreshed week %d-W%02d", year, week)
	return nil
}

// WeekPNG returns the current (or last non-empty) week individual ranking as raw PNG bytes.
func (u *Updater) WeekPNG() ([]byte, string, error) {
	now := time.Now()
	for offset := 0; offset <= 4; offset++ {
		t := now.AddDate(0, 0, -7*offset)
		weekStart, weekEnd := WeekBounds(t)
		year, week := t.ISOWeek()
		label := fmt.Sprintf("Semana %d-W%02d: %s → %s",
			year, week,
			weekStart.Format("Mon Jan 02"),
			weekEnd.AddDate(0, 0, -1).Format("Mon Jan 02, 2006"))
		players, err := u.calc.IndividualRanking(weekStart, weekEnd)
		if err != nil {
			return nil, label, err
		}
		if len(players) > 0 || offset == 4 {
			img, err := u.gen.RenderIndividual(players, label)
			return img, label, err
		}
	}
	return nil, "", fmt.Errorf("no ranking data found")
}

// MonthPNG returns a monthly individual ranking as raw PNG bytes (no MinIO needed).
func (u *Updater) MonthPNG(year, month int) ([]byte, string, error) {
	start, end := MonthBounds(year, month)
	label := fmt.Sprintf("Mes: %s %d", time.Month(month).String(), year)
	players, err := u.calc.IndividualRanking(start, end)
	if err != nil {
		return nil, label, err
	}
	img, err := u.gen.RenderIndividual(players, label)
	return img, label, err
}

// YearPNG returns the full year individual ranking as raw PNG bytes (no MinIO needed).
func (u *Updater) YearPNG(baseYear int) ([]byte, string, error) {
	start, end := YearBounds(baseYear)
	label := fmt.Sprintf("Año %d (completo)", baseYear)
	players, err := u.calc.IndividualRanking(start, end)
	if err != nil {
		return nil, label, err
	}
	img, err := u.gen.RenderIndividual(players, label)
	return img, label, err
}

// OnDemandMonth generates an embed via MinIO for a specific month (used by pinned channel).
func (u *Updater) OnDemandMonth(year, month int) (*discordgo.MessageEmbed, error) {
	img, label, err := u.MonthPNG(year, month)
	if err != nil {
		return nil, err
	}
	return u.uploadEmbed(img, label)
}

// OnDemandLastN generates an embed via MinIO for the full year (used by pinned channel).
func (u *Updater) OnDemandLastN(baseYear int) (*discordgo.MessageEmbed, error) {
	img, label, err := u.YearPNG(baseYear)
	if err != nil {
		return nil, err
	}
	return u.uploadEmbed(img, label)
}

func (u *Updater) uploadEmbed(imgBytes []byte, label string) (*discordgo.MessageEmbed, error) {
	if u.minio == nil {
		return nil, fmt.Errorf("MinIO no configurado")
	}
	key := fmt.Sprintf("ranking-ondemand-%d.png", time.Now().UnixMilli())
	imgURL, err := u.minio.Upload(context.Background(), key, imgBytes)
	if err != nil {
		return nil, err
	}
	return &discordgo.MessageEmbed{
		Description: label,
		Image:       &discordgo.MessageEmbedImage{URL: imgURL},
		Color:       0xC8AA6E,
	}, nil
}
