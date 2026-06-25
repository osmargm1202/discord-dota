package ranking

import (
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
		msg, err := u.session.ChannelMessageSend(u.channelID, fmt.Sprintf("*(cargando ranking %s...)*", t))
		if err != nil {
			return fmt.Errorf("send initial message %s: %w", t, err)
		}
		if err := u.db.SetRankingMessage(t, u.channelID, msg.ID); err != nil {
			return fmt.Errorf("save ranking message %s: %w", t, err)
		}
		logrus.Infof("ranking: created pinned message %s: %s", t, msg.ID)
	}
	return u.Refresh(time.Now())
}

// Refresh recalculates, regenerates PNGs, uploads to MinIO, and edits the 3 Discord messages.
func (u *Updater) Refresh(now time.Time) error {
	if u.channelID == "" || u.minio == nil {
		return nil
	}
	weekStart, weekEnd := WeekBounds(now)
	weekLabel := fmt.Sprintf("Semana: %s → %s",
		weekStart.Format("Mon Jan 02"),
		weekEnd.AddDate(0, 0, -1).Format("Mon Jan 02, 2006"))

	year, week := now.ISOWeek()
	ctx := context.Background()

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

	for _, job := range jobs {
		imgBytes, err := job.render()
		if err != nil {
			logrus.Errorf("ranking: render %s: %v", job.msgType, err)
			continue
		}
		imgURL, err := u.minio.Upload(ctx, job.key, imgBytes)
		if err != nil {
			logrus.Errorf("ranking: upload %s: %v", job.msgType, err)
			continue
		}
		_, msgID, err := u.db.GetRankingMessage(job.msgType)
		if err != nil || msgID == "" {
			logrus.Warnf("ranking: no message ID for %s", job.msgType)
			continue
		}
		embed := &discordgo.MessageEmbed{
			Image: &discordgo.MessageEmbedImage{URL: imgURL},
			Color: 0xC8AA6E,
		}
		if _, err := u.session.ChannelMessageEditEmbed(u.channelID, msgID, embed); err != nil {
			logrus.Errorf("ranking: edit message %s (%s): %v", job.msgType, msgID, err)
		}
	}

	logrus.Infof("ranking: refreshed week %d-W%02d", year, week)
	return nil
}

// OnDemandMonth generates an embed image for a specific month.
func (u *Updater) OnDemandMonth(year, month int) (*discordgo.MessageEmbed, error) {
	start, end := MonthBounds(year, month)
	label := fmt.Sprintf("Mes: %s %d", time.Month(month).String(), year)
	return u.renderOnDemand(start, end, label)
}

// OnDemandLastN generates an embed image for the full year (last N not time-bounded — shows year stats).
func (u *Updater) OnDemandLastN(baseYear int) (*discordgo.MessageEmbed, error) {
	start, end := YearBounds(baseYear)
	label := fmt.Sprintf("Año %d (completo)", baseYear)
	return u.renderOnDemand(start, end, label)
}

func (u *Updater) renderOnDemand(start, end time.Time, label string) (*discordgo.MessageEmbed, error) {
	if u.minio == nil {
		return nil, fmt.Errorf("MinIO no configurado")
	}
	players, err := u.calc.IndividualRanking(start, end)
	if err != nil {
		return nil, err
	}
	imgBytes, err := u.gen.RenderIndividual(players, label)
	if err != nil {
		return nil, err
	}
	key := fmt.Sprintf("ranking-ondemand-%d.png", time.Now().UnixMilli())
	imgURL, err := u.minio.Upload(context.Background(), key, imgBytes)
	if err != nil {
		return nil, err
	}
	return &discordgo.MessageEmbed{
		Image: &discordgo.MessageEmbedImage{URL: imgURL},
		Color: 0xC8AA6E,
	}, nil
}
