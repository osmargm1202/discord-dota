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

// Refresh recalculates all-time ranking (BASE_YEAR → now), regenerates PNGs,
// and edits the 3 pinned Discord messages.
func (u *Updater) Refresh(now time.Time) error {
	if u.channelID == "" {
		return nil
	}

	// All-time: from Jan 1 of base year to now
	allStart := time.Date(u.baseYear, 1, 1, 0, 0, 0, 0, time.UTC)
	allEnd := now
	label := fmt.Sprintf("Ranking Total (desde %d)", u.baseYear)

	// Verify there is data
	rows, _ := u.calc.IndividualRanking(allStart, allEnd)
	if len(rows) == 0 {
		logrus.Infof("ranking: no data since %d, skipping refresh", u.baseYear)
		return nil
	}

	// Fetch cached avatars from MinIO for each player (best-effort, no download)
	if u.minio != nil {
		for i := range rows {
			key := fmt.Sprintf("assets/avatars/%d.jpg", rows[i].DotaID)
			if data, err := u.minio.GetCached(context.Background(), key); err == nil {
				rows[i].AvatarBytes = data
			}
		}
	}

	type imageJob struct {
		msgType string
		key     string
		render  func() ([]byte, error)
	}
	start, end := allStart, allEnd // capture for closures
	jobs := []imageJob{
		{
			msgType: "individual",
			key:     fmt.Sprintf("ranking-individual-total-%d.png", u.baseYear),
			render: func() ([]byte, error) {
				r, err := u.calc.IndividualRanking(start, end)
				if err != nil {
					return nil, err
				}
				return u.gen.RenderIndividual(r, label)
			},
		},
		{
			msgType: "team2",
			key:     fmt.Sprintf("ranking-team2-total-%d.png", u.baseYear),
			render: func() ([]byte, error) {
				r, err := u.calc.Team2Ranking(start, end)
				if err != nil {
					return nil, err
				}
				return u.gen.RenderTeam2(r, label)
			},
		},
		{
			msgType: "team3",
			key:     fmt.Sprintf("ranking-team3-total-%d.png", u.baseYear),
			render: func() ([]byte, error) {
				r, err := u.calc.Team3Ranking(start, end)
				if err != nil {
					return nil, err
				}
				return u.gen.RenderTeam3(r, label)
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

		if u.minio != nil {
			if _, merr := u.minio.Upload(ctx, job.key, imgBytes); merr != nil {
				logrus.Warnf("ranking: minio upload %s: %v", job.msgType, merr)
			}
		}

		_, msgID, err := u.db.GetRankingMessage(job.msgType)
		if err != nil || msgID == "" {
			logrus.Warnf("ranking: no message ID for %s", job.msgType)
			continue
		}

		filename := fmt.Sprintf("ranking-%s.png", job.msgType)
		emptyStr := ""
		noAttachments := []*discordgo.MessageAttachment{}
		_, err = u.session.ChannelMessageEditComplex(&discordgo.MessageEdit{
			ID:          msgID,
			Channel:     u.channelID,
			Content:     &emptyStr,
			Attachments: &noAttachments,
			Files: []*discordgo.File{
				{Name: filename, ContentType: "image/png", Reader: bytes.NewReader(imgBytes)},
			},
		})
		if err != nil {
			logrus.Errorf("ranking: edit message %s (%s): %v", job.msgType, msgID, err)
		}
	}

	logrus.Infof("ranking: refreshed all-time ranking since %d", u.baseYear)
	return nil
}

// SendTempRanking sends a ranking PNG to channelID and deletes it after ttl.
// Used for weekly/monthly on-demand rankings from slash commands.
func (u *Updater) SendTempRanking(channelID string, imgBytes []byte, filename string, ttl time.Duration) {
	msg, err := u.session.ChannelMessageSendComplex(channelID, &discordgo.MessageSend{
		Files: []*discordgo.File{{Name: filename, ContentType: "image/png", Reader: bytes.NewReader(imgBytes)}},
	})
	if err != nil {
		logrus.Warnf("ranking: send temp ranking: %v", err)
		return
	}
	go func() {
		time.Sleep(ttl)
		if err := u.session.ChannelMessageDelete(channelID, msg.ID); err != nil {
			logrus.Warnf("ranking: delete temp ranking msg: %v", err)
		}
	}()
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
