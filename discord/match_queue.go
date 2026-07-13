package discord

import (
	"dota-discord-bot/dota"
	dbpkg "dota-discord-bot/internal/db"
	"errors"
	"fmt"
	"time"
)

const (
	parseQueueBatchSize = 5
	parseRetryInterval  = 10 * time.Minute
)

type discoveryQueueStore interface {
	EnqueueParse(matchID, dotaID int64, discordID string) error
	SetLastProcessedMatch(dotaID, matchID int64) error
}

type parseQueueStore interface {
	GetPendingParseQueue(limit int) ([]dbpkg.ParseQueueRow, error)
	IncrementParseAttempt(matchID, dotaID int64) error
	MarkParseDone(matchID, dotaID int64) error
}

type parseMatchClient interface {
	GetMatch(matchID int64) (*dota.StratzMatch, error)
	RequestParseMatch(matchID int64) error
}

func selectUndiscoveredMatchIDs(matches []dota.StratzMatch, checkpoint int64, hasCheckpoint bool) []int64 {
	if !hasCheckpoint {
		return nil
	}

	var ids []int64
	for _, match := range matches {
		if match.ID > checkpoint {
			ids = append(ids, match.ID)
		}
	}
	return ids
}

func discoverPlayerMatches(store discoveryQueueStore, matches []dota.StratzMatch, checkpoint int64, hasCheckpoint bool, dotaID int64, discordID string) error {
	if len(matches) == 0 {
		return nil
	}
	if !hasCheckpoint {
		return store.SetLastProcessedMatch(dotaID, matches[0].ID)
	}

	ids := selectUndiscoveredMatchIDs(matches, checkpoint, true)
	for _, matchID := range ids {
		if err := store.EnqueueParse(matchID, dotaID, discordID); err != nil {
			return fmt.Errorf("enqueue match %d for dota_id %d: %w", matchID, dotaID, err)
		}
	}
	if len(ids) == 0 {
		return nil
	}
	return store.SetLastProcessedMatch(dotaID, ids[0])
}

func processParseQueue(store parseQueueStore, client parseMatchClient, limit int, now time.Time, handleParsed func(dbpkg.ParseQueueRow, *dota.StratzMatch) error) error {
	rows, err := store.GetPendingParseQueue(limit)
	if err != nil {
		return err
	}

	var rowErrors []error
	for _, row := range rows {
		if row.LastAttempt.Valid && now.Sub(row.LastAttempt.Time) < parseRetryInterval {
			continue
		}
		match, err := client.GetMatch(row.MatchID)
		if err != nil || match == nil {
			attemptErr := store.IncrementParseAttempt(row.MatchID, row.DotaID)
			if err == nil {
				err = errors.New("Stratz returned nil match")
			}
			rowErrors = append(rowErrors, errors.Join(fmt.Errorf("get match %d: %w", row.MatchID, err), attemptErr))
			continue
		}
		if !dota.IsMatchParsed(match) {
			if err := client.RequestParseMatch(row.MatchID); err != nil {
				rowErrors = append(rowErrors, fmt.Errorf("request parse %d: %w", row.MatchID, err))
			}
			if err := store.IncrementParseAttempt(row.MatchID, row.DotaID); err != nil {
				rowErrors = append(rowErrors, fmt.Errorf("increment attempt %d/%d: %w", row.MatchID, row.DotaID, err))
			}
			continue
		}
		if err := handleParsed(row, match); err != nil {
			rowErrors = append(rowErrors, fmt.Errorf("handle match %d/%d: %w", row.MatchID, row.DotaID, err))
			continue
		}
		if err := store.MarkParseDone(row.MatchID, row.DotaID); err != nil {
			rowErrors = append(rowErrors, fmt.Errorf("mark done %d/%d: %w", row.MatchID, row.DotaID, err))
		}
	}
	return errors.Join(rowErrors...)
}
