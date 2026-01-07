GET /players/{account_id}/wl
Win/Loss count

path Parameters
account_id
required
integer
Steam32 account ID

query Parameters
limit	
integer
Number of matches to limit to

offset	
integer
Number of matches to offset start by

win	
integer
Whether the player won

patch	
integer
Patch ID, from dotaconstants

game_mode	
integer
Game Mode ID

lobby_type	
integer
Lobby type ID

region	
integer
Region ID

date	
integer
Days previous

lane_role	
integer
Lane Role ID

hero_id	
integer
Hero ID

is_radiant	
integer
Whether the player was radiant

included_account_id	
integer
Account IDs in the match (array)

excluded_account_id	
integer
Account IDs not in the match (array)

with_hero_id	
integer
Hero IDs on the player's team (array)

against_hero_id	
integer
Hero IDs against the player's team (array)

significant	
integer
Whether the match was significant for aggregation purposes. Defaults to 1 (true), set this to 0 to return data for non-standard modes/matches.

having	
integer
The minimum number of games played, for filtering hero stats

sort	
string
The field to return matches sorted by in descending order

Responses
200 Success

get
/players/{account_id}/wl
Response samples
200
Content type
application/json; charset=utf-8

Copy
{
"win": 0,
"lose": 0
}
GET /players/{account_id}/recentMatches
Recent matches played (limited number of results)

path Parameters
account_id
required
integer
Steam32 account ID

Responses
200 Success

get
/players/{account_id}/recentMatches
Response samples
200
Content type
application/json; charset=utf-8

Copy
Expand allCollapse all
[
{ }
]
GET /players/{account_id}/matches
Matches played (full history, and supports column selection)

path Parameters
account_id
required
integer
Steam32 account ID

query Parameters
limit	
integer
Number of matches to limit to

offset	
integer
Number of matches to offset start by

win	
integer
Whether the player won

patch	
integer
Patch ID, from dotaconstants

game_mode	
integer
Game Mode ID

lobby_type	
integer
Lobby type ID

region	
integer
Region ID

date	
integer
Days previous

lane_role	
integer
Lane Role ID

hero_id	
integer
Hero ID

is_radiant	
integer
Whether the player was radiant

included_account_id	
integer
Account IDs in the match (array)

excluded_account_id	
integer
Account IDs not in the match (array)

with_hero_id	
integer
Hero IDs on the player's team (array)

against_hero_id	
integer
Hero IDs against the player's team (array)

significant	
integer
Whether the match was significant for aggregation purposes. Defaults to 1 (true), set this to 0 to return data for non-standard modes/matches.

having	
integer
The minimum number of games played, for filtering hero stats

sort	
string
The field to return matches sorted by in descending order

project	
string
Fields to project (array)

Responses
200 Success

get
/players/{account_id}/matches
Response samples
200
Content type
application/json; charset=utf-8

Copy
Expand allCollapse all
[
{
"match_id": 3703866531,
"player_slot": 0,
"radiant_win": true,
"duration": 0,
"game_mode": 0,
"lobby_type": 0,
"hero_id": 0,
"start_time": 0,
"version": 0,
"kills": 0,
"deaths": 0,
"assists": 0,
"skill": 0,
"average_rank": 0,
"leaver_status": 0,
"party_size": 0,
"hero_variant": 0
}
]
GET /players/{account_id}/heroes
Heroes played

path Parameters
account_id
required
integer
Steam32 account ID

query Parameters
limit	
integer
Number of matches to limit to

offset	
integer
Number of matches to offset start by

win	
integer
Whether the player won

patch	
integer
Patch ID, from dotaconstants

game_mode	
integer
Game Mode ID

lobby_type	
integer
Lobby type ID

region	
integer
Region ID

date	
integer
Days previous

lane_role	
integer
Lane Role ID

hero_id	
integer
Hero ID

is_radiant	
integer
Whether the player was radiant

included_account_id	
integer
Account IDs in the match (array)

excluded_account_id	
integer
Account IDs not in the match (array)

with_hero_id	
integer
Hero IDs on the player's team (array)

against_hero_id	
integer
Hero IDs against the player's team (array)

significant	
integer
Whether the match was significant for aggregation purposes. Defaults to 1 (true), set this to 0 to return data for non-standard modes/matches.

having	
integer
The minimum number of games played, for filtering hero stats

sort	
string
The field to return matches sorted by in descending order

Responses
200 Success

get
/players/{account_id}/heroes
Response samples
200
Content type
application/json; charset=utf-8

Copy
Expand allCollapse all
[
{
"hero_id": 0,
"last_played": 0,
"games": 0,
"win": 0,
"with_games": 0,
"with_win": 0,
"against_games": 0,
"against_win": 0
}
]
GET /players/{account_id}/peers
Players played with

path Parameters
account_id
required
integer
Steam32 account ID

query Parameters
limit	
integer
Number of matches to limit to

offset	
integer
Number of matches to offset start by

win	
integer
Whether the player won

patch	
integer
Patch ID, from dotaconstants

game_mode	
integer
Game Mode ID

lobby_type	
integer
Lobby type ID

region	
integer
Region ID

date	
integer
Days previous

lane_role	
integer
Lane Role ID

hero_id	
integer
Hero ID

is_radiant	
integer
Whether the player was radiant

included_account_id	
integer
Account IDs in the match (array)

excluded_account_id	
integer
Account IDs not in the match (array)

with_hero_id	
integer
Hero IDs on the player's team (array)

against_hero_id	
integer
Hero IDs against the player's team (array)

significant	
integer
Whether the match was significant for aggregation purposes. Defaults to 1 (true), set this to 0 to return data for non-standard modes/matches.

having	
integer
The minimum number of games played, for filtering hero stats

sort	
string
The field to return matches sorted by in descending order

Responses
200 Success

get
/players/{account_id}/peers
Response samples
200
Content type
application/json; charset=utf-8

Copy
Expand allCollapse all
[
{
"account_id": 0,
"last_played": 0,
"win": 0,
"games": 0,
"with_win": 0,
"with_games": 0,
"against_win": 0,
"against_games": 0,
"with_gpm_sum": 0,
"with_xpm_sum": 0,
"personaname": "420 booty wizard",
"name": "string",
"is_contributor": true,
"is_subscriber": true,
"last_login": "string",
"avatar": "string",
"avatarfull": "string"
}
]
GET /players/{account_id}/pros
Pro players played with

path Parameters
account_id
required
integer
Steam32 account ID

query Parameters
limit	
integer
Number of matches to limit to

offset	
integer
Number of matches to offset start by

win	
integer
Whether the player won

patch	
integer
Patch ID, from dotaconstants

game_mode	
integer
Game Mode ID

lobby_type	
integer
Lobby type ID

region	
integer
Region ID

date	
integer
Days previous

lane_role	
integer
Lane Role ID

hero_id	
integer
Hero ID

is_radiant	
integer
Whether the player was radiant

included_account_id	
integer
Account IDs in the match (array)

excluded_account_id	
integer
Account IDs not in the match (array)

with_hero_id	
integer
Hero IDs on the player's team (array)

against_hero_id	
integer
Hero IDs against the player's team (array)

significant	
integer
Whether the match was significant for aggregation purposes. Defaults to 1 (true), set this to 0 to return data for non-standard modes/matches.

having	
integer
The minimum number of games played, for filtering hero stats

sort	
string
The field to return matches sorted by in descending order

Responses
200 Success

get
/players/{account_id}/pros
Response samples
200
Content type
application/json; charset=utf-8

Copy
Expand allCollapse all
[
{
"account_id": 0,
"name": "string",
"country_code": "string",
"fantasy_role": 0,
"team_id": 0,
"team_name": "Newbee",
"team_tag": "string",
"is_locked": true,
"is_pro": true,
"locked_until": 0,
"steamid": "string",
"avatar": "string",
"avatarmedium": "string",
"avatarfull": "string",
"profileurl": "string",
"last_login": "2019-08-24T14:15:22Z",
"full_history_time": "2019-08-24T14:15:22Z",
"cheese": 0,
"fh_unavailable": true,
"loccountrycode": "string",
"last_played": 0,
"win": 0,
"games": 0,
"with_win": 0,
"with_games": 0,
"against_win": 0,
"against_games": 0,
"with_gpm_sum": 0,
"with_xpm_sum": 0
}
]
GET /players/{account_id}/totals
Totals in stats

path Parameters
account_id
required
integer
Steam32 account ID

query Parameters
limit	
integer
Number of matches to limit to

offset	
integer
Number of matches to offset start by

win	
integer
Whether the player won

patch	
integer
Patch ID, from dotaconstants

game_mode	
integer
Game Mode ID

lobby_type	
integer
Lobby type ID

region	
integer
Region ID

date	
integer
Days previous

lane_role	
integer
Lane Role ID

hero_id	
integer
Hero ID

is_radiant	
integer
Whether the player was radiant

included_account_id	
integer
Account IDs in the match (array)

excluded_account_id	
integer
Account IDs not in the match (array)

with_hero_id	
integer
Hero IDs on the player's team (array)

against_hero_id	
integer
Hero IDs against the player's team (array)

significant	
integer
Whether the match was significant for aggregation purposes. Defaults to 1 (true), set this to 0 to return data for non-standard modes/matches.

having	
integer
The minimum number of games played, for filtering hero stats

sort	
string
The field to return matches sorted by in descending order

Responses
200 Success

get
/players/{account_id}/totals
Response samples
200
Content type
application/json; charset=utf-8

Copy
Expand allCollapse all
[
{
"field": "string",
"n": 0,
"sum": 0
}
]
GET /players/{account_id}/counts
Counts in categories

path Parameters
account_id
required
integer
Steam32 account ID

query Parameters
limit	
integer
Number of matches to limit to

offset	
integer
Number of matches to offset start by

win	
integer
Whether the player won

patch	
integer
Patch ID, from dotaconstants

game_mode	
integer
Game Mode ID

lobby_type	
integer
Lobby type ID

region	
integer
Region ID

date	
integer
Days previous

lane_role	
integer
Lane Role ID

hero_id	
integer
Hero ID

is_radiant	
integer
Whether the player was radiant

included_account_id	
integer
Account IDs in the match (array)

excluded_account_id	
integer
Account IDs not in the match (array)

with_hero_id	
integer
Hero IDs on the player's team (array)

against_hero_id	
integer
Hero IDs against the player's team (array)

significant	
integer
Whether the match was significant for aggregation purposes. Defaults to 1 (true), set this to 0 to return data for non-standard modes/matches.

having	
integer
The minimum number of games played, for filtering hero stats

sort	
string
The field to return matches sorted by in descending order

Responses
200 Success

get
/players/{account_id}/counts
Response samples
200
Content type
application/json; charset=utf-8

Copy
Expand allCollapse all
{
"leaver_status": { },
"game_mode": { },
"lobby_type": { },
"lane_role": { },
"region": { },
"patch": { }
}
GET /players/{account_id}/histograms
Distribution of matches in a single stat

path Parameters
account_id
required
integer
Steam32 account ID

field
required
string
Field to aggregate on

query Parameters
limit	
integer
Number of matches to limit to

offset	
integer
Number of matches to offset start by

win	
integer
Whether the player won

patch	
integer
Patch ID, from dotaconstants

game_mode	
integer
Game Mode ID

lobby_type	
integer
Lobby type ID

region	
integer
Region ID

date	
integer
Days previous

lane_role	
integer
Lane Role ID

hero_id	
integer
Hero ID

is_radiant	
integer
Whether the player was radiant

included_account_id	
integer
Account IDs in the match (array)

excluded_account_id	
integer
Account IDs not in the match (array)

with_hero_id	
integer
Hero IDs on the player's team (array)

against_hero_id	
integer
Hero IDs against the player's team (array)

significant	
integer
Whether the match was significant for aggregation purposes. Defaults to 1 (true), set this to 0 to return data for non-standard modes/matches.

having	
integer
The minimum number of games played, for filtering hero stats

sort	
string
The field to return matches sorted by in descending order

Responses
200 Success

get
/players/{account_id}/histograms/{field}
Response samples
200
Content type
application/json; charset=utf-8

Copy
Expand allCollapse all
[
{ }
]
GET /players/{account_id}/wardmap
Wards placed in matches played

path Parameters
account_id
required
integer
Steam32 account ID

query Parameters
limit	
integer
Number of matches to limit to

offset	
integer
Number of matches to offset start by

win	
integer
Whether the player won

patch	
integer
Patch ID, from dotaconstants

game_mode	
integer
Game Mode ID

lobby_type	
integer
Lobby type ID

region	
integer
Region ID

date	
integer
Days previous

lane_role	
integer
Lane Role ID

hero_id	
integer
Hero ID

is_radiant	
integer
Whether the player was radiant

included_account_id	
integer
Account IDs in the match (array)

excluded_account_id	
integer
Account IDs not in the match (array)

with_hero_id	
integer
Hero IDs on the player's team (array)

against_hero_id	
integer
Hero IDs against the player's team (array)

significant	
integer
Whether the match was significant for aggregation purposes. Defaults to 1 (true), set this to 0 to return data for non-standard modes/matches.

having	
integer
The minimum number of games played, for filtering hero stats

sort	
string
The field to return matches sorted by in descending order

Responses
200 Success

get
/players/{account_id}/wardmap
Response samples
200
Content type
application/json; charset=utf-8

Copy
Expand allCollapse all
{
"obs": { },
"sen": { }
}
GET /players/{account_id}/wordcloud
Words said/read in matches played

path Parameters
account_id
required
integer
Steam32 account ID

query Parameters
limit	
integer
Number of matches to limit to

offset	
integer
Number of matches to offset start by

win	
integer
Whether the player won

patch	
integer
Patch ID, from dotaconstants

game_mode	
integer
Game Mode ID

lobby_type	
integer
Lobby type ID

region	
integer
Region ID

date	
integer
Days previous

lane_role	
integer
Lane Role ID

hero_id	
integer
Hero ID

is_radiant	
integer
Whether the player was radiant

included_account_id	
integer
Account IDs in the match (array)

excluded_account_id	
integer
Account IDs not in the match (array)

with_hero_id	
integer
Hero IDs on the player's team (array)

against_hero_id	
integer
Hero IDs against the player's team (array)

significant	
integer
Whether the match was significant for aggregation purposes. Defaults to 1 (true), set this to 0 to return data for non-standard modes/matches.

having	
integer
The minimum number of games played, for filtering hero stats

sort	
string
The field to return matches sorted by in descending order

Responses
200 Success

get
/players/{account_id}/wordcloud
Response samples
200
Content type
application/json; charset=utf-8

Copy
Expand allCollapse all
{
"my_word_counts": { },
"all_word_counts": { }
}
GET /players/{account_id}/ratings
Returns a history of the player rank tier/medal changes (replaces MMR)

path Parameters
account_id
required
integer
Steam32 account ID

Responses
200 Success

get
/players/{account_id}/ratings
Response samples
200
Content type
application/json; charset=utf-8

Copy
Expand allCollapse all
[
{
"account_id": 0,
"match_id": 3703866531,
"solo_competitive_rank": 0,
"competitive_rank": 0,
"time": 0
}
]
GET /players/{account_id}/rankings
Player hero rankings

path Parameters
account_id
required
integer
Steam32 account ID

Responses
200 Success

get
/players/{account_id}/rankings
Response samples
200
Content type
application/json; charset=utf-8

Copy
Expand allCollapse all
[
{
"hero_id": 0,
"score": 0,
"percent_rank": 0,
"card": 0
}
]
POST /players/{account_id}/refresh
Refresh player match history (up to 500), medal (rank), and profile name

path Parameters
account_id
required
integer
Steam32 account ID

Responses
200 Success

post
/players/{account_id}/refresh
Response samples
200
Content type
application/json; charset=utf-8

Copy
null
top players
GET /topPlayers
Get list of highly ranked players

query Parameters
turbo	
integer
Get ratings based on turbo matches

Responses
200 Success