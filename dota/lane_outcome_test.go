package dota

import "testing"

func TestPlayerLaneSlot(t *testing.T) {
	cases := []struct {
		lane      string
		isRadiant bool
		want      string
	}{
		{"SAFE_LANE", true, "bottom"},
		{"SAFE_LANE", false, "top"},
		{"OFF_LANE", true, "top"},
		{"OFF_LANE", false, "bottom"},
		{"MID_LANE", true, "mid"},
		{"MID_LANE", false, "mid"},
		{"ROAMING", true, ""},
		{"", true, ""},
	}
	for _, c := range cases {
		got := PlayerLaneSlot(c.lane, c.isRadiant)
		if got != c.want {
			t.Errorf("PlayerLaneSlot(%q, %v) = %q, want %q", c.lane, c.isRadiant, got, c.want)
		}
	}
}

func TestLaneOutcomeForPlayer(t *testing.T) {
	cases := []struct {
		name             string
		lane             string
		isRadiant        bool
		top, mid, bottom string
		want             string
	}{
		{"radiant safe lane wins bottom (radiant stomp)", "SAFE_LANE", true, "TIE", "TIE", "RADIANT_STOMP", "won"},
		{"dire safe lane top loses to radiant stomp", "SAFE_LANE", false, "RADIANT_STOMP", "TIE", "TIE", "lost"},
		{"dire off lane bottom wins vs dire_victory", "OFF_LANE", false, "TIE", "TIE", "DIRE_VICTORY", "won"},
		{"radiant off lane top loses vs dire_victory", "OFF_LANE", true, "DIRE_VICTORY", "TIE", "TIE", "lost"},
		{"mid lane tie", "MID_LANE", true, "TIE", "TIE", "TIE", "tied"},
		{"roaming has no lane outcome", "ROAMING", true, "RADIANT_STOMP", "TIE", "TIE", ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := LaneOutcomeForPlayer(c.lane, c.isRadiant, c.top, c.mid, c.bottom)
			if got != c.want {
				t.Errorf("got %q, want %q", got, c.want)
			}
		})
	}
}
