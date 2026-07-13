package discord

import (
	"database/sql"
	"dota-discord-bot/dota"
	dbpkg "dota-discord-bot/internal/db"
	"errors"
	"reflect"
	"testing"
	"time"
)

func TestSelectUndiscoveredMatchIDs(t *testing.T) {
	tests := []struct {
		name          string
		matches       []dota.StratzMatch
		checkpoint    int64
		hasCheckpoint bool
		want          []int64
	}{
		{
			name: "selects every ID above checkpoint in API order",
			matches: []dota.StratzMatch{
				{ID: 105}, {ID: 104}, {ID: 103}, {ID: 102},
			},
			checkpoint:    102,
			hasCheckpoint: true,
			want:          []int64{105, 104, 103},
		},
		{
			name: "does not replay history without checkpoint",
			matches: []dota.StratzMatch{
				{ID: 105}, {ID: 104}, {ID: 103},
			},
			hasCheckpoint: false,
			want:          nil,
		},
		{
			name:          "returns none when checkpoint is current",
			matches:       []dota.StratzMatch{{ID: 105}, {ID: 104}},
			checkpoint:    105,
			hasCheckpoint: true,
			want:          nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := selectUndiscoveredMatchIDs(tt.matches, tt.checkpoint, tt.hasCheckpoint)
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("selectUndiscoveredMatchIDs() = %v, want %v", got, tt.want)
			}
		})
	}
}

type fakeDiscoveryStore struct {
	enqueued      []dbpkg.ParseQueueRow
	checkpoint    int64
	enqueueErrFor int64
}

func (f *fakeDiscoveryStore) EnqueueParse(matchID, dotaID int64, discordID string) error {
	if matchID == f.enqueueErrFor {
		return errors.New("enqueue failed")
	}
	f.enqueued = append(f.enqueued, dbpkg.ParseQueueRow{MatchID: matchID, DotaID: dotaID, DiscordID: discordID})
	return nil
}

func (f *fakeDiscoveryStore) SetLastProcessedMatch(_ int64, matchID int64) error {
	f.checkpoint = matchID
	return nil
}

func TestDiscoverPlayerMatchesEnqueuesAllBeforeAdvancingCheckpoint(t *testing.T) {
	store := &fakeDiscoveryStore{}
	matches := []dota.StratzMatch{{ID: 12}, {ID: 11}, {ID: 10}}

	if err := discoverPlayerMatches(store, matches, 10, true, 7, "discord-7"); err != nil {
		t.Fatal(err)
	}

	if got := []int64{store.enqueued[0].MatchID, store.enqueued[1].MatchID}; !reflect.DeepEqual(got, []int64{12, 11}) {
		t.Fatalf("enqueued %v, want [12 11]", got)
	}
	if store.checkpoint != 12 {
		t.Fatalf("checkpoint = %d, want 12", store.checkpoint)
	}
}

func TestDiscoverPlayerMatchesDoesNotAdvanceCheckpointAfterEnqueueFailure(t *testing.T) {
	store := &fakeDiscoveryStore{enqueueErrFor: 11}
	matches := []dota.StratzMatch{{ID: 12}, {ID: 11}, {ID: 10}}

	if err := discoverPlayerMatches(store, matches, 10, true, 7, "discord-7"); err == nil {
		t.Fatal("expected enqueue error")
	}
	if store.checkpoint != 0 {
		t.Fatalf("checkpoint advanced to %d", store.checkpoint)
	}
}

type fakeParseQueue struct {
	rows      []dbpkg.ParseQueueRow
	attempted []int64
	completed []int64
}

func (f *fakeParseQueue) GetPendingParseQueue(limit int) ([]dbpkg.ParseQueueRow, error) {
	if len(f.rows) > limit {
		return f.rows[:limit], nil
	}
	return f.rows, nil
}
func (f *fakeParseQueue) IncrementParseAttempt(matchID, _ int64) error {
	f.attempted = append(f.attempted, matchID)
	return nil
}
func (f *fakeParseQueue) MarkParseDone(matchID, _ int64) error {
	f.completed = append(f.completed, matchID)
	return nil
}

type fakeParseClient struct {
	matches   map[int64]*dota.StratzMatch
	fetched   []int64
	requested []int64
}

func (f *fakeParseClient) GetMatch(matchID int64) (*dota.StratzMatch, error) {
	f.fetched = append(f.fetched, matchID)
	return f.matches[matchID], nil
}
func (f *fakeParseClient) RequestParseMatch(matchID int64) error {
	f.requested = append(f.requested, matchID)
	return nil
}

func TestProcessParseQueueSkipsCooldownBeforeFetchingMatch(t *testing.T) {
	now := time.Now()
	queue := &fakeParseQueue{rows: []dbpkg.ParseQueueRow{{
		MatchID: 10,
		DotaID:  7,
		LastAttempt: sql.NullTime{
			Time:  now.Add(-time.Minute),
			Valid: true,
		},
	}}}
	client := &fakeParseClient{matches: map[int64]*dota.StratzMatch{10: {ID: 10}}}

	if err := processParseQueue(queue, client, 5, now, func(dbpkg.ParseQueueRow, *dota.StratzMatch) error {
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if len(client.fetched) != 0 {
		t.Fatalf("fetched cooldown matches %v, want none", client.fetched)
	}
}

func TestProcessParseQueueUnparsedRemainsPendingAndNewerParsedCompletes(t *testing.T) {
	parsedAt := int64(1)
	queue := &fakeParseQueue{rows: []dbpkg.ParseQueueRow{
		{MatchID: 20, DotaID: 7},
		{MatchID: 10, DotaID: 7, LastAttempt: sql.NullTime{Time: time.Now().Add(-time.Hour), Valid: true}},
	}}
	client := &fakeParseClient{matches: map[int64]*dota.StratzMatch{
		20: {ID: 20, ParsedDateTime: &parsedAt, TopLaneOutcome: "TIE", Players: []dota.StratzPlayer{{Lane: "MID_LANE", Imp: 1}}},
		10: {ID: 10},
	}}
	var handled []int64

	err := processParseQueue(queue, client, 5, time.Now(), func(row dbpkg.ParseQueueRow, _ *dota.StratzMatch) error {
		handled = append(handled, row.MatchID)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(queue.completed, []int64{20}) {
		t.Fatalf("completed = %v, want [20]", queue.completed)
	}
	if !reflect.DeepEqual(client.requested, []int64{10}) || !reflect.DeepEqual(queue.attempted, []int64{10}) {
		t.Fatalf("requested=%v attempted=%v, want [10]", client.requested, queue.attempted)
	}
	if !reflect.DeepEqual(handled, []int64{20}) {
		t.Fatalf("handled = %v, want [20]", handled)
	}
}
