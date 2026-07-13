package discord

import (
	"dota-discord-bot/dota"
	"reflect"
	"testing"
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
