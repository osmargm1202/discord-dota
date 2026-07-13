package discord

import "dota-discord-bot/dota"

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
