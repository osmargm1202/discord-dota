package dota

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// ErrSearchNotSupported se devuelve cuando Stratz no ofrece búsqueda por nombre
var ErrSearchNotSupported = errors.New("Stratz no ofrece búsqueda por nombre; usa account_id directamente")

const stratzBaseURL = "https://api.stratz.com/graphql"

// Steam CDN base para avatares (Stratz steamAccount.avatar puede ser ruta relativa)
const steamAvatarBaseURL = "https://steamcdn-a.akamaihd.net/steamcommunity/public/images/avatars"

// StratzClient es el cliente para la API GraphQL de Stratz
type StratzClient struct {
	httpClient *http.Client
	token      string
	debug      bool // si true, escribe request/response en logs/stratz_debug.log
}

// NewStratzClient crea un nuevo cliente de Stratz
func NewStratzClient(token string) *StratzClient {
	return &StratzClient{
		httpClient: &http.Client{
			Timeout: 15 * time.Second,
		},
		token: token,
	}
}

// IsConfigured verifica si el cliente tiene token configurado
func (c *StratzClient) IsConfigured() bool {
	return c.token != ""
}

// SetDebug activa o desactiva el volcado de request/response a logs/stratz_debug.log
func (c *StratzClient) SetDebug(debug bool) {
	c.debug = debug
}

// GraphQL request/response types
type graphQLRequest struct {
	Query     string                 `json:"query"`
	Variables map[string]interface{} `json:"variables,omitempty"`
}

type graphQLResponse struct {
	Data   json.RawMessage `json:"data"`
	Errors []struct {
		Message string `json:"message"`
	} `json:"errors,omitempty"`
}

// StratzMatch representa una partida de Stratz
type StratzMatch struct {
	ID                int64            `json:"id"`
	DidRadiantWin     bool             `json:"didRadiantWin"`
	DurationSeconds   int              `json:"durationSeconds"`
	StartDateTime     int64            `json:"startDateTime"`
	GameMode          stratzIntOrStr   `json:"gameMode"`       // Stratz puede devolver int o string (enum)
	LobbyType         stratzIntOrStr   `json:"lobbyType"`      // Stratz puede devolver int o string (enum)
	RadiantKills      stratzIntOrArray `json:"radiantKills"`   // Stratz puede devolver int o array
	DireKills         stratzIntOrArray `json:"direKills"`      // Stratz puede devolver int o array
	ParsedDateTime    *int64           `json:"parsedDateTime"` // Long; si no es null y > 0, la partida está parseada
	Players           []StratzPlayer   `json:"players"`
	TopLaneOutcome    string           `json:"topLaneOutcome"`    // TIE, RADIANT_VICTORY, RADIANT_STOMP, DIRE_VICTORY, DIRE_STOMP
	MidLaneOutcome    string           `json:"midLaneOutcome"`    // idem
	BottomLaneOutcome string           `json:"bottomLaneOutcome"` // idem
}

// stratzIntOrArray acepta radiantKills/direKills como int o array desde la API (array = suma de elementos; API devuelve kills por minuto)
type stratzIntOrArray int

func (s *stratzIntOrArray) UnmarshalJSON(data []byte) error {
	data = bytes.TrimSpace(data)
	if len(data) == 0 {
		return nil
	}
	// Stratz puede devolver null si la partida no está parseada o el campo no está disponible
	if len(data) == 4 && string(data) == "null" {
		*s = 0
		return nil
	}
	if data[0] == '[' {
		// Array: sumar elementos (int o float64 según decoder)
		var arr []int
		if err := json.Unmarshal(data, &arr); err != nil {
			var arrF []float64
			if errF := json.Unmarshal(data, &arrF); errF != nil {
				return err
			}
			sum := 0
			for _, v := range arrF {
				sum += int(v)
			}
			*s = stratzIntOrArray(sum)
			return nil
		}
		sum := 0
		for _, v := range arr {
			sum += v
		}
		*s = stratzIntOrArray(sum)
		return nil
	}
	var n int
	if err := json.Unmarshal(data, &n); err != nil {
		var nF float64
		if errF := json.Unmarshal(data, &nF); errF != nil {
			return err
		}
		*s = stratzIntOrArray(int(nF))
		return nil
	}
	*s = stratzIntOrArray(n)
	return nil
}

// stratzGameModeToID mapea Stratz GameModeEnumType a Dota game_mode ID (game_mode.json)
var stratzGameModeToID = map[string]int{
	"NONE": 0, "UNKNOWN": 0,
	"ALL_PICK": 1, "CAPTAINS_MODE": 2, "RANDOM_DRAFT": 3, "SINGLE_DRAFT": 4,
	"ALL_RANDOM": 5, "INTRO": 6, "THE_DIRETIDE": 7, "REVERSE_CAPTAINS_MODE": 8,
	"THE_GREEVILING": 9, "TUTORIAL": 10, "MID_ONLY": 11, "LEAST_PLAYED": 12,
	"NEW_PLAYER_POOL": 13, "COMPENDIUM_MATCHMAKING": 14, "CUSTOM": 15,
	"CAPTAINS_DRAFT": 16, "BALANCED_DRAFT": 17, "ABILITY_DRAFT": 18, "EVENT": 19,
	"ALL_RANDOM_DEATH_MATCH": 20, "SOLO_MID": 21, "ALL_PICK_RANKED": 22,
	"TURBO": 23, "MUTATION": 24,
}

// stratzIntOrStr acepta gameMode/lobbyType como int o string (enum) desde la API
type stratzIntOrStr int

func (s *stratzIntOrStr) UnmarshalJSON(data []byte) error {
	if len(data) >= 2 && data[0] == '"' && data[len(data)-1] == '"' {
		var str string
		if err := json.Unmarshal(data, &str); err != nil {
			return err
		}
		var n int
		if _, err := fmt.Sscanf(str, "%d", &n); err == nil {
			*s = stratzIntOrStr(n)
			return nil
		}
		if id, ok := stratzGameModeToID[str]; ok {
			*s = stratzIntOrStr(id)
			return nil
		}
		*s = 0
		return nil
	}
	var n int
	if err := json.Unmarshal(data, &n); err != nil {
		return err
	}
	*s = stratzIntOrStr(n)
	return nil
}

// StratzPlayer representa un jugador en una partida de Stratz
type StratzPlayer struct {
	SteamAccountID      int64               `json:"steamAccountId"`
	IsRadiant           bool                `json:"isRadiant"`
	HeroID              int                 `json:"heroId"`
	Kills               int                 `json:"kills"`
	Deaths              int                 `json:"deaths"`
	Assists             int                 `json:"assists"`
	Level               int                 `json:"level"`
	GoldPerMinute       int                 `json:"goldPerMinute"`
	ExperiencePerMinute int                 `json:"experiencePerMinute"`
	HeroDamage          int                 `json:"heroDamage"`
	TowerDamage         int                 `json:"towerDamage"`
	HeroHealing         int                 `json:"heroHealing"`
	Lane                string              `json:"lane"` // enum SAFE_LANE, MID_LANE, OFF_LANE o vacío
	Role                string              `json:"role"` // enum CORE, SUPPORT o vacío
	SteamAccount        *StratzSteamAccount `json:"steamAccount"`
}

// StratzSteamAccount representa la cuenta de Steam de un jugador
type StratzSteamAccount struct {
	ID          int64  `json:"id"`
	Name        string `json:"name"`
	Avatar      string `json:"avatar"`
	IsAnonymous bool   `json:"isAnonymous"`
}

// StratzPlayerStats representa las estadísticas de un jugador
type StratzPlayerStats struct {
	SteamAccountID int64  `json:"steamAccountId"`
	Name           string `json:"name"`
	Avatar         string `json:"avatar"`
	WinCount       int    `json:"winCount"`
	MatchCount     int    `json:"matchCount"`
	RankBracket    string `json:"rankBracket"` // UNCALIBRATED, HERALD, GUARDIAN, CRUSADER, ARCHON, LEGEND, ANCIENT, DIVINE, IMMORTAL
}

// StratzHeroStats representa las estadísticas de un héroe para un jugador
type StratzHeroStats struct {
	HeroID     int `json:"heroId"`
	WinCount   int `json:"winCount"`
	MatchCount int `json:"matchCount"`
}

// Días considerados "parche actual" para estadísticas por héroe
const statsPatchDays = 120

// StatsPatchDays devuelve los días usados como "parche actual" en GetPlayerHeroStats
func StatsPatchDays() int { return statsPatchDays }

func (c *StratzClient) makeRequest(query string, variables map[string]interface{}, result interface{}) error {
	reqBody := graphQLRequest{
		Query:     query,
		Variables: variables,
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("error serializando request: %w", err)
	}

	req, err := http.NewRequest("POST", stratzBaseURL, bytes.NewBuffer(jsonBody))
	if err != nil {
		return fmt.Errorf("error creando request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("User-Agent", "STRATZ_API")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("error en request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("error leyendo respuesta: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("API retornó status %d: %s", resp.StatusCode, string(body))
	}

	var gqlResp graphQLResponse
	if err := json.Unmarshal(body, &gqlResp); err != nil {
		return fmt.Errorf("error decodificando respuesta: %w", err)
	}

	if len(gqlResp.Errors) > 0 {
		return fmt.Errorf("GraphQL error: %s", gqlResp.Errors[0].Message)
	}

	if err := json.Unmarshal(gqlResp.Data, result); err != nil {
		return fmt.Errorf("error decodificando data: %w", err)
	}

	if c.debug {
		c.writeDebugLog("request", query, variables, nil)
		c.writeDebugLog("response", "", nil, body)
	}

	return nil
}

// writeDebugLog escribe en logs/stratz_debug.log para depuración (solo si debug=true)
func (c *StratzClient) writeDebugLog(kind, query string, variables map[string]interface{}, body []byte) {
	const logDir = "logs"
	const logFile = "stratz_debug.log"
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return
	}
	f, err := os.OpenFile(filepath.Join(logDir, logFile), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		return
	}
	defer f.Close()
	ts := time.Now().Format("2006-01-02 15:04:05")
	if kind == "request" {
		varStr := ""
		if len(variables) > 0 {
			b, _ := json.MarshalIndent(variables, "", "  ")
			varStr = string(b)
		}
		fmt.Fprintf(f, "[%s] === STRATZ REQUEST ===\nQuery:\n%s\nVariables:\n%s\n", ts, query, varStr)
	} else {
		bodyStr := string(body)
		if len(bodyStr) > 8000 {
			bodyStr = bodyStr[:8000] + "\n... (truncado)"
		}
		fmt.Fprintf(f, "[%s] === STRATZ RESPONSE (raw) ===\n%s\n\n", ts, bodyStr)
	}
}

// GetMatch obtiene los detalles de una partida (lane outcomes, lane/role, parsedDateTime para saber si está parseada)
func (c *StratzClient) GetMatch(matchID int64) (*StratzMatch, error) {
	query := `
		query GetMatch($matchId: Long!) {
			match(id: $matchId) {
				id
				didRadiantWin
				durationSeconds
				startDateTime
				gameMode
				lobbyType
				radiantKills
				direKills
				parsedDateTime
				topLaneOutcome
				midLaneOutcome
				bottomLaneOutcome
				players {
					steamAccountId
					isRadiant
					heroId
					lane
					role
					kills
					deaths
					assists
					level
					goldPerMinute
					experiencePerMinute
					heroDamage
					towerDamage
					heroHealing
					steamAccount {
						id
						name
						avatar
						isAnonymous
					}
				}
			}
		}
	`

	var result struct {
		Match *StratzMatch `json:"match"`
	}

	if err := c.makeRequest(query, map[string]interface{}{"matchId": matchID}, &result); err != nil {
		return nil, err
	}

	return result.Match, nil
}

// GetPlayerRecentMatches obtiene las partidas recientes de un jugador (incluye parseStatus por partida)
func (c *StratzClient) GetPlayerRecentMatches(steamAccountID int64, limit int) ([]StratzMatch, error) {
	query := `
		query GetPlayerMatches($steamAccountId: Long!, $take: Int!) {
			player(steamAccountId: $steamAccountId) {
				matches(request: { take: $take }) {
					id
					didRadiantWin
					durationSeconds
					startDateTime
					gameMode
					lobbyType
					radiantKills
					direKills
					players {
						steamAccountId
						isRadiant
						heroId
						kills
						deaths
						assists
						level
						goldPerMinute
						experiencePerMinute
						heroDamage
						towerDamage
						heroHealing
						steamAccount {
							id
							name
							avatar
							isAnonymous
						}
					}
				}
			}
		}
	`

	var result struct {
		Player struct {
			Matches []StratzMatch `json:"matches"`
		} `json:"player"`
	}

	if err := c.makeRequest(query, map[string]interface{}{
		"steamAccountId": steamAccountID,
		"take":           limit,
	}, &result); err != nil {
		return nil, err
	}

	return result.Player.Matches, nil
}

// GetPlayerWinLoss obtiene el W/L de un jugador (últimas N partidas, opcionalmente filtrado por héroe)
func (c *StratzClient) GetPlayerWinLoss(steamAccountID int64, limit int, heroID int) (*WinLossResponse, error) {
	var query string
	var variables map[string]interface{}

	if heroID > 0 {
		query = `
			query GetPlayerHeroWL($steamAccountId: Long!, $take: Int!, $heroId: Short!) {
				player(steamAccountId: $steamAccountId) {
					matches(request: { take: $take, heroIds: [$heroId] }) {
						didRadiantWin
						players(steamAccountId: $steamAccountId) {
							isRadiant
						}
					}
				}
			}
		`
		variables = map[string]interface{}{
			"steamAccountId": steamAccountID,
			"take":           limit,
			"heroId":         heroID,
		}
	} else {
		query = `
			query GetPlayerWL($steamAccountId: Long!, $take: Int!) {
				player(steamAccountId: $steamAccountId) {
					matches(request: { take: $take }) {
						didRadiantWin
						players(steamAccountId: $steamAccountId) {
							isRadiant
						}
					}
				}
			}
		`
		variables = map[string]interface{}{
			"steamAccountId": steamAccountID,
			"take":           limit,
		}
	}

	var result struct {
		Player struct {
			Matches []struct {
				DidRadiantWin bool `json:"didRadiantWin"`
				Players       []struct {
					IsRadiant bool `json:"isRadiant"`
				} `json:"players"`
			} `json:"matches"`
		} `json:"player"`
	}

	if err := c.makeRequest(query, variables, &result); err != nil {
		return nil, err
	}

	wins := 0
	losses := 0
	for _, match := range result.Player.Matches {
		if len(match.Players) > 0 {
			isRadiant := match.Players[0].IsRadiant
			if (match.DidRadiantWin && isRadiant) || (!match.DidRadiantWin && !isRadiant) {
				wins++
			} else {
				losses++
			}
		}
	}

	return &WinLossResponse{Win: wins, Lose: losses}, nil
}

// GetPlayerProfile obtiene el perfil de un jugador (incluye rankBracket si está disponible)
func (c *StratzClient) GetPlayerProfile(steamAccountID int64) (*StratzPlayerStats, error) {
	query := `
		query GetPlayer($steamAccountId: Long!) {
			player(steamAccountId: $steamAccountId) {
				steamAccountId
				steamAccount {
					name
					avatar
					isAnonymous
				}
				winCount
				matchCount
				ranks(seasonRankIds: [0]) {
					rankBracket
				}
			}
		}
	`

	var result struct {
		Player struct {
			SteamAccountID int64 `json:"steamAccountId"`
			SteamAccount   struct {
				Name        string `json:"name"`
				Avatar      string `json:"avatar"`
				IsAnonymous bool   `json:"isAnonymous"`
			} `json:"steamAccount"`
			WinCount   int `json:"winCount"`
			MatchCount int `json:"matchCount"`
			Ranks      []struct {
				RankBracket string `json:"rankBracket"`
			} `json:"ranks"`
		} `json:"player"`
	}

	if err := c.makeRequest(query, map[string]interface{}{"steamAccountId": steamAccountID}, &result); err != nil {
		return nil, err
	}

	rankBracket := ""
	if len(result.Player.Ranks) > 0 && result.Player.Ranks[0].RankBracket != "" {
		rankBracket = result.Player.Ranks[0].RankBracket
	}

	avatar := NormalizeSteamAvatarURL(result.Player.SteamAccount.Avatar)

	return &StratzPlayerStats{
		SteamAccountID: result.Player.SteamAccountID,
		Name:           result.Player.SteamAccount.Name,
		Avatar:         avatar,
		WinCount:       result.Player.WinCount,
		MatchCount:     result.Player.MatchCount,
		RankBracket:    rankBracket,
	}, nil
}

// NormalizeSteamAvatarURL convierte avatar de Stratz (relativo o pequeño) a URL completa tamaño full para Discord.
// Steam: avatar = 32x32, avatarfull = _full.jpg (184x184). Si Stratz devuelve ruta relativa, se añade base CDN.
func NormalizeSteamAvatarURL(avatar string) string {
	if avatar == "" {
		return ""
	}
	url := avatar
	if !strings.HasPrefix(avatar, "http") {
		url = strings.TrimSuffix(steamAvatarBaseURL, "/") + "/" + strings.TrimPrefix(avatar, "/")
	}
	// Preferir tamaño full (184x184) para Discord
	if strings.HasSuffix(url, ".jpg") && !strings.Contains(url, "_full.") {
		url = strings.TrimSuffix(url, ".jpg") + "_full.jpg"
	}
	return url
}

// GetMultiplePlayersWinLoss obtiene W/L de múltiples jugadores en una sola query
func (c *StratzClient) GetMultiplePlayersWinLoss(steamAccountIDs []int64, limit int) (map[int64]*WinLossResponse, error) {
	if len(steamAccountIDs) == 0 {
		return make(map[int64]*WinLossResponse), nil
	}

	// Construir query dinámica para múltiples jugadores
	query := `query GetMultiplePlayersWL($take: Int!) {`

	for i, id := range steamAccountIDs {
		query += fmt.Sprintf(`
			player%d: player(steamAccountId: %d) {
				steamAccountId
				matches(request: { take: $take }) {
					didRadiantWin
					players(steamAccountId: %d) {
						isRadiant
					}
				}
			}
		`, i, id, id)
	}
	query += `}`

	var result map[string]*struct {
		SteamAccountID int64 `json:"steamAccountId"`
		Matches        []struct {
			DidRadiantWin bool `json:"didRadiantWin"`
			Players       []struct {
				IsRadiant bool `json:"isRadiant"`
			} `json:"players"`
		} `json:"matches"`
	}

	if err := c.makeRequest(query, map[string]interface{}{"take": limit}, &result); err != nil {
		return nil, err
	}

	wlMap := make(map[int64]*WinLossResponse)
	for _, player := range result {
		if player == nil {
			continue
		}
		wins := 0
		losses := 0
		for _, match := range player.Matches {
			if len(match.Players) > 0 {
				isRadiant := match.Players[0].IsRadiant
				if (match.DidRadiantWin && isRadiant) || (!match.DidRadiantWin && !isRadiant) {
					wins++
				} else {
					losses++
				}
			}
		}
		wlMap[player.SteamAccountID] = &WinLossResponse{Win: wins, Lose: losses}
	}

	return wlMap, nil
}

// GetPlayerHeroStats obtiene W/L por héroe en las últimas take partidas (sin filtro de parche).
// take: 1-100 (Stratz impone máx. 100). Solo devuelve héroes con al menos minGames partidas. Ordenado por partidas jugadas (desc).
func (c *StratzClient) GetPlayerHeroStats(steamAccountID int64, minGames, take int) ([]StratzHeroStats, error) {
	if take <= 0 {
		take = 100
	}
	if take > 100 {
		take = 100
	}
	matches, err := c.GetPlayerRecentMatches(steamAccountID, take)
	if err != nil {
		return nil, err
	}
	byHero := make(map[int]struct{ Win, Match int })
	for _, m := range matches {
		for _, p := range m.Players {
			if p.SteamAccountID != steamAccountID {
				continue
			}
			h := byHero[p.HeroID]
			h.Match++
			if (m.DidRadiantWin && p.IsRadiant) || (!m.DidRadiantWin && !p.IsRadiant) {
				h.Win++
			}
			byHero[p.HeroID] = h
			break
		}
	}
	var out []StratzHeroStats
	for heroID, v := range byHero {
		if v.Match < minGames {
			continue
		}
		out = append(out, StratzHeroStats{HeroID: heroID, WinCount: v.Win, MatchCount: v.Match})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].MatchCount > out[j].MatchCount })
	return out, nil
}
func GetHeroImageURLStratz(heroID int) string {
	return fmt.Sprintf("https://cdn.stratz.com/images/dota2/heroes/%d_icon.png", heroID)
}

// StratzMatchToMatchResponse convierte una partida Stratz al tipo OpenDota (MatchResponse)
func StratzMatchToMatchResponse(m *StratzMatch) *MatchResponse {
	if m == nil {
		return nil
	}
	radiantWin := m.DidRadiantWin
	players := make([]Player, 0, len(m.Players))
	var radiantIdx, direIdx int
	for i := range m.Players {
		sp := &m.Players[i]
		players = append(players, *StratzPlayerToPlayer(sp, m.DidRadiantWin, &radiantIdx, &direIdx))
	}
	radiantScore := int(m.RadiantKills)
	direScore := int(m.DireKills)
	// Fallback: si Stratz devuelve 0-0 (p. ej. null o partida sin parsear), calcular desde kills de jugadores
	if radiantScore == 0 && direScore == 0 && len(m.Players) > 0 {
		for i := range m.Players {
			p := &m.Players[i]
			if p.IsRadiant {
				radiantScore += p.Kills
			} else {
				direScore += p.Kills
			}
		}
	}
	return &MatchResponse{
		MatchID:           m.ID,
		RadiantWin:        &radiantWin,
		Duration:          m.DurationSeconds,
		StartTime:         m.StartDateTime,
		GameMode:          int(m.GameMode),
		LobbyType:         int(m.LobbyType),
		RadiantScore:      radiantScore,
		DireScore:         direScore,
		Players:           players,
		TopLaneOutcome:    m.TopLaneOutcome,
		MidLaneOutcome:    m.MidLaneOutcome,
		BottomLaneOutcome: m.BottomLaneOutcome,
	}
}

// StratzPlayerToPlayer convierte un jugador Stratz al tipo OpenDota (Player)
func StratzPlayerToPlayer(sp *StratzPlayer, radiantWin bool, radiantIdx, direIdx *int) *Player {
	if sp == nil {
		return nil
	}
	win := 0
	lose := 0
	isRadiant := sp.IsRadiant
	if (radiantWin && isRadiant) || (!radiantWin && !isRadiant) {
		win = 1
	} else {
		lose = 1
	}
	var playerSlot int
	if isRadiant {
		playerSlot = *radiantIdx
		*radiantIdx++
	} else {
		playerSlot = 128 + *direIdx
		*direIdx++
	}
	personaname := ""
	if sp.SteamAccount != nil {
		personaname = sp.SteamAccount.Name
	}
	kda := 0.0
	if sp.Deaths > 0 {
		kda = float64(sp.Kills+sp.Assists) / float64(sp.Deaths)
	} else if sp.Kills+sp.Assists > 0 {
		kda = float64(sp.Kills + sp.Assists)
	}
	return &Player{
		AccountID:   int(sp.SteamAccountID),
		PlayerSlot:  playerSlot,
		HeroID:      sp.HeroID,
		Kills:       sp.Kills,
		Deaths:      sp.Deaths,
		Assists:     sp.Assists,
		Win:         &win,
		Lose:        &lose,
		IsRadiant:   &isRadiant,
		Personaname: personaname,
		Level:       sp.Level,
		GoldPerMin:  sp.GoldPerMinute,
		XpPerMin:    sp.ExperiencePerMinute,
		HeroDamage:  sp.HeroDamage,
		TowerDamage: sp.TowerDamage,
		HeroHealing: sp.HeroHealing,
		KDA:         kda,
		Lane:        sp.Lane,
		Role:        sp.Role,
	}
}

// AnalyzeStreakFromStratzMatches calcula la racha desde partidas Stratz para un jugador
func AnalyzeStreakFromStratzMatches(matches []StratzMatch, steamAccountID int64) StreakResult {
	if len(matches) == 0 {
		return StreakResult{CurrentStreak: "Sin partidas"}
	}
	wins := 0
	losses := 0
	streakCount := 0
	var isWinStreak bool
	for idx, m := range matches {
		var found bool
		var isRadiant bool
		for _, p := range m.Players {
			if p.SteamAccountID == steamAccountID {
				found = true
				isRadiant = p.IsRadiant
				break
			}
		}
		if !found {
			continue
		}
		won := (m.DidRadiantWin && isRadiant) || (!m.DidRadiantWin && !isRadiant)
		if won {
			wins++
		} else {
			losses++
		}
		if idx == 0 {
			isWinStreak = won
			streakCount = 1
		} else {
			if won == isWinStreak {
				streakCount++
			} else {
				break
			}
		}
	}
	var currentStreak string
	if isWinStreak {
		currentStreak = fmt.Sprintf("%d victorias consecutivas 🔥", streakCount)
	} else {
		currentStreak = fmt.Sprintf("%d derrotas consecutivas 💀", streakCount)
	}
	return StreakResult{
		Wins:          wins,
		Losses:        losses,
		CurrentStreak: currentStreak,
		StreakCount:   streakCount,
		IsWinStreak:   isWinStreak,
	}
}

// IsMatchParsed indica si la partida está parseada según la API (MatchType.parsedDateTime).
// Si parsedDateTime es null o 0, la partida no está parseada.
func IsMatchParsed(m *StratzMatch) bool {
	if m == nil {
		return false
	}
	if m.ParsedDateTime == nil {
		return false
	}
	return *m.ParsedDateTime > 0
}

// RequestParseMatch solicita el parse de una partida en Stratz (si la API expone la mutación)
func (c *StratzClient) RequestParseMatch(matchID int64) error {
	query := `
		mutation RequestParse($matchId: Long!) {
			requestParse(matchId: $matchId)
		}
	`
	var result struct {
		RequestParse *bool `json:"requestParse"`
	}
	err := c.makeRequest(query, map[string]interface{}{"matchId": matchID}, &result)
	if err != nil {
		return err
	}
	if result.RequestParse != nil && !*result.RequestParse {
		return fmt.Errorf("requestParse devolvió false para match %d", matchID)
	}
	return nil
}

// SearchPlayers con Stratz no está soportado (Stratz no expone búsqueda por nombre en la API pública)
func (c *StratzClient) SearchPlayers(query string) ([]SearchResponse, error) {
	return nil, ErrSearchNotSupported
}
