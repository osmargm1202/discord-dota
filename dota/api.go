package dota

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const baseURL = "https://api.opendota.com/api"

type Client struct {
	httpClient   *http.Client
	heroesCache  map[int]string
	heroImages   map[int]string
	gameModes    map[int]string
	lobbyTypes   map[int]string
	lastCacheUpdate time.Time
}

func NewClient() *Client {
	return &Client{
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
		heroesCache: make(map[int]string),
		heroImages:  make(map[int]string),
		gameModes:   make(map[int]string),
		lobbyTypes:  make(map[int]string),
	}
}

// SearchResponse representa un resultado de búsqueda
type SearchResponse struct {
	AccountID      int    `json:"account_id"`
	Personaname    string `json:"personaname"`
	Avatarfull     string `json:"avatarfull"`
	LastMatchTime  string `json:"last_match_time"`
	Similarity     float64 `json:"similarity"`
}

// PlayerRecentMatch representa una partida reciente
type PlayerRecentMatch struct {
	MatchID    int64  `json:"match_id"`
	PlayerSlot int    `json:"player_slot"`
	RadiantWin *bool  `json:"radiant_win"`
	Duration   int    `json:"duration"`
	GameMode   int    `json:"game_mode"`
	LobbyType  int    `json:"lobby_type"`
	HeroID     int    `json:"hero_id"`
	StartTime  int64  `json:"start_time"`
	Kills      int    `json:"kills"`
	Deaths     int    `json:"deaths"`
	Assists    int    `json:"assists"`
	Win        *int   `json:"win"`
	Lose       *int   `json:"lose"`
	IsRadiant  *bool  `json:"isRadiant"`
}

// MatchResponse representa una partida completa
type MatchResponse struct {
	MatchID      int64   `json:"match_id"`
	RadiantWin   *bool   `json:"radiant_win"`
	Duration     int     `json:"duration"`
	StartTime    int64   `json:"start_time"`
	GameMode     int     `json:"game_mode"`
	LobbyType    int     `json:"lobby_type"`
	RadiantScore int     `json:"radiant_score"`
	DireScore    int     `json:"dire_score"`
	Players      []Player `json:"players"`
}

// Player representa un jugador en una partida
type Player struct {
	AccountID    int     `json:"account_id"`
	PlayerSlot   int     `json:"player_slot"`
	HeroID       int     `json:"hero_id"`
	Kills        int     `json:"kills"`
	Deaths       int     `json:"deaths"`
	Assists      int     `json:"assists"`
	Win          *int    `json:"win"`
	Lose         *int    `json:"lose"`
	IsRadiant    *bool   `json:"isRadiant"`
	Personaname  string  `json:"personaname"`
	Level        int     `json:"level"`
	GoldPerMin   int     `json:"gold_per_min"`
	XpPerMin     int     `json:"xp_per_min"`
	HeroDamage   int     `json:"hero_damage"`
	TowerDamage  int     `json:"tower_damage"`
	HeroHealing  int     `json:"hero_healing"`
	KDA          float64 `json:"kda"`
	RankTier     *int    `json:"rank_tier"`
}

// PlayersResponse representa el perfil de un jugador
type PlayersResponse struct {
	Profile struct {
		AccountID   int    `json:"account_id"`
		Personaname string `json:"personaname"`
		Avatar      string `json:"avatar"`
		Avatarfull  string `json:"avatarfull"`
		Profileurl  string `json:"profileurl"`
		LastLogin   string `json:"last_login"`
	} `json:"profile"`
	RankTier        *int     `json:"rank_tier"`
	LeaderboardRank *int     `json:"leaderboard_rank"`
	ComputedMMR     *float64 `json:"computed_mmr"`
}

// Hero representa un héroe
type Hero struct {
	ID           int    `json:"id"`
	Name         string `json:"name"`
	LocalizedName string `json:"localized_name"`
	Img          string `json:"img"`
}

// WinLossResponse representa el resumen W/L de un jugador
type WinLossResponse struct {
	Win  int `json:"win"`
	Lose int `json:"lose"`
}

func (c *Client) makeRequest(url string, result interface{}) error {
	// Rate limiting: esperar 1 segundo entre requests
	time.Sleep(1 * time.Second)

	resp, err := c.httpClient.Get(url)
	if err != nil {
		return fmt.Errorf("error en request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("API retornó status %d: %s", resp.StatusCode, string(body))
	}

	if err := json.NewDecoder(resp.Body).Decode(result); err != nil {
		return fmt.Errorf("error decodificando respuesta: %w", err)
	}

	return nil
}

// makePostRequest realiza un POST simple sin payload y valida status 200
func (c *Client) makePostRequest(url string) error {
	// Rate limiting: esperar 1 segundo entre requests
	time.Sleep(1 * time.Second)

	req, err := http.NewRequest(http.MethodPost, url, nil)
	if err != nil {
		return fmt.Errorf("error creando request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("error en request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("API retornó status %d: %s", resp.StatusCode, string(body))
	}

	// Consumir el body para no dejarlo abierto (aunque no lo usemos)
	_, _ = io.ReadAll(resp.Body)
	return nil
}

func (c *Client) SearchPlayers(query string) ([]SearchResponse, error) {
	url := fmt.Sprintf("%s/search?q=%s", baseURL, query)
	var results []SearchResponse
	if err := c.makeRequest(url, &results); err != nil {
		return nil, err
	}
	return results, nil
}

func (c *Client) GetPlayerProfile(accountID string) (*PlayersResponse, error) {
	url := fmt.Sprintf("%s/players/%s", baseURL, accountID)
	var profile PlayersResponse
	if err := c.makeRequest(url, &profile); err != nil {
		return nil, err
	}
	return &profile, nil
}

func (c *Client) GetRecentMatches(accountID string) ([]PlayerRecentMatch, error) {
	url := fmt.Sprintf("%s/players/%s/recentMatches", baseURL, accountID)
	var matches []PlayerRecentMatch
	if err := c.makeRequest(url, &matches); err != nil {
		return nil, err
	}
	return matches, nil
}

func (c *Client) GetMatchDetails(matchID int64) (*MatchResponse, error) {
	url := fmt.Sprintf("%s/matches/%d", baseURL, matchID)
	var match MatchResponse
	if err := c.makeRequest(url, &match); err != nil {
		return nil, err
	}
	return &match, nil
}

// GetWinLoss obtiene el resumen W/L; si limit>0 se envía como query param.
// Si heroID > 0, filtra por ese héroe específico.
func (c *Client) GetWinLoss(accountID string, limit int, heroID int) (*WinLossResponse, error) {
	url := fmt.Sprintf("%s/players/%s/wl", baseURL, accountID)
	queryParams := []string{}
	if limit > 0 {
		queryParams = append(queryParams, fmt.Sprintf("limit=%d", limit))
	}
	if heroID > 0 {
		queryParams = append(queryParams, fmt.Sprintf("hero_id=%d", heroID))
	}
	if len(queryParams) > 0 {
		url = fmt.Sprintf("%s?%s", url, strings.Join(queryParams, "&"))
	}
	var wl WinLossResponse
	if err := c.makeRequest(url, &wl); err != nil {
		return nil, err
	}
	return &wl, nil
}

// RefreshPlayer fuerza refresh de historial, medalla y nombre del perfil
func (c *Client) RefreshPlayer(accountID string) error {
	url := fmt.Sprintf("%s/players/%s/refresh", baseURL, accountID)
	return c.makePostRequest(url)
}

func (c *Client) FindPlayerInMatch(match *MatchResponse, accountID string) (*Player, error) {
	accountIDInt, err := strconv.Atoi(accountID)
	if err != nil {
		return nil, fmt.Errorf("account_id inválido: %w", err)
	}

	for i := range match.Players {
		if match.Players[i].AccountID == accountIDInt {
			return &match.Players[i], nil
		}
	}
	return nil, fmt.Errorf("jugador no encontrado en la partida")
}

func (c *Client) IsWin(match PlayerRecentMatch) bool {
	if match.Win != nil {
		return *match.Win == 1
	}
	if match.RadiantWin != nil && match.IsRadiant != nil {
		return *match.RadiantWin == *match.IsRadiant
	}
	// Fallback: usar player_slot
	isRadiant := match.PlayerSlot < 128
	if match.RadiantWin != nil {
		return isRadiant == *match.RadiantWin
	}
	return false
}

func (c *Client) IsWinFromPlayer(player Player, radiantWin *bool) bool {
	if player.Win != nil {
		return *player.Win == 1
	}
	if radiantWin != nil && player.IsRadiant != nil {
		return *radiantWin == *player.IsRadiant
	}
	// Fallback: usar player_slot
	isRadiant := player.PlayerSlot < 128
	if radiantWin != nil {
		return isRadiant == *radiantWin
	}
	return false
}

type StreakResult struct {
	Wins          int
	Losses        int
	CurrentStreak string
	StreakCount   int
	IsWinStreak   bool
}

func (c *Client) AnalyzeStreak(matches []PlayerRecentMatch) StreakResult {
	if len(matches) == 0 {
		return StreakResult{CurrentStreak: "Sin partidas"}
	}

	wins := 0
	losses := 0
	isFirstWin := c.IsWin(matches[0])
	streakCount := 0

	// Las partidas vienen de más reciente a más antigua.
	// Contar mientras el resultado sea igual al de la más reciente.
	for idx, match := range matches {
		isWin := c.IsWin(match)
		if isWin {
			wins++
		} else {
			losses++
		}

		if isWin == isFirstWin {
			streakCount++
		} else if idx > 0 {
			break
		}
	}

	isWinStreak := isFirstWin

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

func (c *Client) LoadConstants() error {
	// Cargar héroes
	var heroes []Hero
	url := fmt.Sprintf("%s/constants/heroes", baseURL)
	if err := c.makeRequest(url, &heroes); err != nil {
		return fmt.Errorf("error cargando héroes: %w", err)
	}
	for _, hero := range heroes {
		c.heroesCache[hero.ID] = hero.LocalizedName
		c.heroImages[hero.ID] = hero.Img
	}

	// Cargar game modes
	url = fmt.Sprintf("%s/constants/game_mode", baseURL)
	if err := c.makeRequest(url, &c.gameModes); err != nil {
		// No crítico, continuar sin game modes
	}

	// Cargar lobby types
	url = fmt.Sprintf("%s/constants/lobby_type", baseURL)
	if err := c.makeRequest(url, &c.lobbyTypes); err != nil {
		// No crítico, continuar sin lobby types
	}

	c.lastCacheUpdate = time.Now()
	return nil
}

func (c *Client) GetHeroName(heroID int) string {
	// Intentar cargar desde JSON local
	c.loadHeroesLocal()
	if name, ok := c.heroesCache[heroID]; ok {
		return name
	}
	// Fallback remoto
	if len(c.heroesCache) == 0 || time.Since(c.lastCacheUpdate) > 24*time.Hour {
		if err := c.LoadConstants(); err == nil {
			if name, ok := c.heroesCache[heroID]; ok {
				return name
			}
		}
	}
	return fmt.Sprintf("Hero %d", heroID)
}

func (c *Client) GetGameModeName(gameMode int) string {
	// Intentar cargar desde JSON local
	c.loadGameModesLocal()
	if name, ok := c.gameModes[gameMode]; ok {
		return name
	}
	// Fallback remoto
	if len(c.gameModes) == 0 {
		if err := c.LoadConstants(); err == nil {
			if name, ok := c.gameModes[gameMode]; ok {
				return name
			}
		}
	}
	return fmt.Sprintf("Mode %d", gameMode)
}

func (c *Client) GetLobbyTypeName(lobbyType int) string {
	if len(c.lobbyTypes) == 0 {
		if err := c.LoadConstants(); err != nil {
			return fmt.Sprintf("Lobby %d", lobbyType)
		}
	}
	if name, ok := c.lobbyTypes[lobbyType]; ok {
		return name
	}
	return fmt.Sprintf("Lobby %d", lobbyType)
}

func FormatDuration(seconds int) string {
	hours := seconds / 3600
	minutes := (seconds % 3600) / 60
	secs := seconds % 60

	if hours > 0 {
		return fmt.Sprintf("%d:%02d:%02d", hours, minutes, secs)
	}
	return fmt.Sprintf("%d:%02d", minutes, secs)
}

func GetRankName(rankTier *int) string {
	if rankTier == nil {
		return "Unranked"
	}

	rank := *rankTier
	tier := rank / 10
	star := rank % 10

	ranks := map[int]string{
		1: "Herald",
		2: "Guardian",
		3: "Crusader",
		4: "Archon",
		5: "Legend",
		6: "Ancient",
		7: "Divine",
		8: "Immortal",
	}

	if tierName, ok := ranks[tier]; ok {
		if tier == 8 {
			return tierName
		}
		return fmt.Sprintf("%s %d", tierName, star)
	}
	return fmt.Sprintf("Rank %d", rank)
}

// --- Helpers para cargar caches locales ---

func (c *Client) loadHeroesLocal() {
	if len(c.heroesCache) > 0 && len(c.heroImages) > 0 {
		return
	}
	data, err := os.ReadFile(filepath.Join("dota", "heroes.json"))
	if err != nil {
		return
	}
	var raw map[string]Hero
	if err := json.Unmarshal(data, &raw); err != nil {
		return
	}
	for _, h := range raw {
		if h.LocalizedName != "" {
			c.heroesCache[h.ID] = h.LocalizedName
		}
		if h.Img != "" {
			c.heroImages[h.ID] = h.Img
		}
	}
}

func (c *Client) loadGameModesLocal() {
	if len(c.gameModes) > 0 {
		return
	}
	data, err := os.ReadFile(filepath.Join("dota", "game_mode.json"))
	if err != nil {
		return
	}
	var raw map[string]struct {
		ID   int    `json:"id"`
		Name string `json:"name"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return
	}
	for _, gm := range raw {
		if gm.Name != "" {
			c.gameModes[gm.ID] = gm.Name
		}
	}
}

// GetHeroImageURL retorna la URL completa del icono del héroe (si se tiene)
func (c *Client) GetHeroImageURL(heroID int) string {
	c.loadHeroesLocal()
	if path, ok := c.heroImages[heroID]; ok && path != "" {
		return fmt.Sprintf("https://api.opendota.com%s", path)
	}
	return ""
}

