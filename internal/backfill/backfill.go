package backfill

import (
	"dota-discord-bot/dota"
	"dota-discord-bot/internal/db"
	"fmt"
	"time"

	"github.com/sirupsen/logrus"
)

// Service fetches historical matches from Stratz and stores them in PG.
type Service struct {
	db      *db.DB
	stratz  *dota.StratzClient
	baseYear int
	delayMS  int
}

func New(database *db.DB, stratz *dota.StratzClient, baseYear, delayMS int) *Service {
	return &Service{db: database, stratz: stratz, baseYear: baseYear, delayMS: delayMS}
}

// Run fetches historical matches for all users from BASE_YEAR-01-01.
// Skips matches already in DB. Safe to call multiple times (idempotent).
func (s *Service) Run() {
	afterUnix := time.Date(s.baseYear, 1, 1, 0, 0, 0, 0, time.UTC).Unix()
	logrus.Infof("backfill: starting from %d-01-01 (unix %d)", s.baseYear, afterUnix)

	users, err := s.db.GetAllUsers()
	if err != nil {
		logrus.Errorf("backfill: get users: %v", err)
		return
	}
	if len(users) == 0 {
		logrus.Info("backfill: no users registered, skipping")
		return
	}

	for _, u := range users {
		logrus.Infof("backfill: processing dota_id %d", u.DotaID)
		if err := s.backfillUser(u.DotaID, afterUnix); err != nil {
			logrus.Errorf("backfill: dota_id %d: %v", u.DotaID, err)
		}
	}
	logrus.Info("backfill: complete")
}

// RunForUser runs backfill for a single user (e.g. after /dota admin register).
func (s *Service) RunForUser(dotaID int64) {
	afterUnix := time.Date(s.baseYear, 1, 1, 0, 0, 0, 0, time.UTC).Unix()
	logrus.Infof("backfill: running for new user dota_id %d", dotaID)
	if err := s.backfillUser(dotaID, afterUnix); err != nil {
		logrus.Errorf("backfill: dota_id %d: %v", dotaID, err)
	}
}

func (s *Service) backfillUser(dotaID int64, afterUnix int64) error {
	const pageSize = 100
	skip := 0
	total := 0

	for {
		matches, err := s.stratz.GetPlayerMatchesForBackfill(dotaID, pageSize, skip, afterUnix)
		if err != nil {
			return fmt.Errorf("fetch page skip=%d: %w", skip, err)
		}
		if len(matches) == 0 {
			break
		}

		for _, m := range matches {
			if err := s.storeMatch(dotaID, m); err != nil {
				logrus.Warnf("backfill: store match %d for %d: %v", m.ID, dotaID, err)
			}
			total++
			time.Sleep(time.Duration(s.delayMS) * time.Millisecond)
		}

		if total%50 == 0 {
			logrus.Infof("backfill: dota_id %d — %d matches stored so far", dotaID, total)
		}

		if len(matches) < pageSize {
			break
		}
		skip += pageSize
	}

	logrus.Infof("backfill: dota_id %d complete — %d matches", dotaID, total)
	return nil
}

func (s *Service) storeMatch(dotaID int64, m dota.BackfillMatch) error {
	exists, err := s.db.PlayerMatchExists(m.ID, dotaID)
	if err != nil {
		return err
	}
	if exists {
		return nil
	}

	startTime := time.Unix(m.StartDateTime, 0).UTC()
	if err := s.db.UpsertMatch(m.ID, startTime, m.DurationSecs, int(m.GameMode), m.DidRadiantWin, false); err != nil {
		return fmt.Errorf("upsert match: %w", err)
	}

	for _, p := range m.Players {
		if p.SteamAccountID != dotaID {
			continue
		}
		won := (m.DidRadiantWin && p.IsRadiant) || (!m.DidRadiantWin && !p.IsRadiant)
		mmrDelta := -25
		if won {
			mmrDelta = 25
		}
		return s.db.UpsertPlayerMatch(db.PlayerMatchRow{
			MatchID:     m.ID,
			DotaID:      dotaID,
			IsRadiant:   p.IsRadiant,
			HeroID:      p.HeroID,
			Kills:       p.Kills,
			Deaths:      p.Deaths,
			Assists:     p.Assists,
			Level:       p.Level,
			GPM:         p.GoldPerMinute,
			XPM:         p.ExperiencePerMinute,
			HeroDamage:  p.HeroDamage,
			TowerDamage: p.TowerDamage,
			Healing:     p.HeroHealing,
			Imp:         p.Imp,
			Award:       p.Award,
			Lane:        p.Lane,
			Role:        p.Role,
			Won:         won,
			MMRDelta:    mmrDelta,
		})
	}
	return nil
}
