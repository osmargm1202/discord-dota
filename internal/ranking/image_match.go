package ranking

import (
	"bytes"
	"fmt"
	"time"

	"github.com/fogleman/gg"
)

// MatchRenderData holds all display data for a match notification PNG.
type MatchRenderData struct {
	PlayerName      string
	HeroName        string
	GameMode        string
	RankBracket     string
	IsWin           bool
	KDA             string // e.g. "12/3/8 (5.33 KDA)"
	Duration        string
	Level           int
	RadiantScore    int
	DireScore       int
	GPM             int
	XPM             int
	IMP             string
	HeroRecord      string // e.g. "8-4 (66.7%)"
	LaneInfo        string // e.g. "Safelane / Carry"
	LaneOutcome     string // e.g. "Victoria en línea"
	HeroDamage      int
	TowerDamage     int
	HeroHealing     int
	Streak          string // e.g. "3 victorias seguidas"
	MatchID         int64
	AnalysisOutcome string // e.g. "Paliza"
	UpdatedAt       time.Time
}

// RenderMatch generates a PNG card for a single match notification.
func (g *ImageGenerator) RenderMatch(d MatchRenderData) ([]byte, error) {
	const w = canvasW

	// Dynamic height: base + optional rows
	h := 250.0
	if d.LaneOutcome != "" {
		h += rowH
	}
	if d.HeroDamage > 0 || d.TowerDamage > 0 || d.HeroHealing > 0 {
		h += rowH
	}

	dc := gg.NewContext(int(w), int(h))
	g.drawBackground(dc)

	resultColor := colorGreen
	resultText := "✅ VICTORIA"
	if !d.IsWin {
		resultColor = colorRed
		resultText = "❌ DERROTA"
	}

	// ── Header bar ──────────────────────────────────────────────────
	dc.SetColor(resultColor)
	dc.DrawRectangle(0, 0, w, headerH)
	dc.Fill()

	g.loadFont(dc, 22)
	dc.SetColor(colorBG)
	dc.DrawStringAnchored(resultText, paddingL, headerH/2, 0, 0.5)

	// Analysis outcome badge (right side of header)
	if d.AnalysisOutcome != "" {
		g.loadFont(dc, 14)
		dc.DrawStringAnchored(d.AnalysisOutcome, w-paddingL, headerH/2, 1, 0.5)
	}

	y := headerH + 2

	// ── Player / Hero row ────────────────────────────────────────────
	dc.SetColor(colorPanel)
	dc.DrawRectangle(0, y, w, rowH+8)
	dc.Fill()

	g.loadFont(dc, 20)
	dc.SetColor(colorGold)
	playerLine := d.PlayerName
	if d.RankBracket != "" {
		playerLine = fmt.Sprintf("%s  [%s]", d.PlayerName, d.RankBracket)
	}
	dc.DrawStringAnchored(playerLine, paddingL, y+26, 0, 0.5)

	g.loadFont(dc, 15)
	dc.SetColor(colorGray)
	modeHero := fmt.Sprintf("%s  ·  %s", d.HeroName, d.GameMode)
	dc.DrawStringAnchored(modeHero, paddingL, y+48, 0, 0.5)

	y += rowH + 8 + 4

	// ── Stats grid row 1 ─────────────────────────────────────────────
	// KDA | Duration | Level | GPM | XPM | IMP
	type statCell struct{ label, value string }
	stats1 := []statCell{
		{"K/D/A", d.KDA},
		{"Duración", d.Duration},
		{"Nivel", fmt.Sprintf("%d", d.Level)},
		{"GPM / XPM", fmt.Sprintf("%d / %d", d.GPM, d.XPM)},
		{"IMP", d.IMP},
	}
	cellW := w / float64(len(stats1))

	dc.SetColor(colorPanel)
	dc.DrawRectangle(0, y, w, rowH+10)
	dc.Fill()

	for idx, s := range stats1 {
		cx := float64(idx)*cellW + cellW/2
		g.loadFont(dc, 11)
		dc.SetColor(colorGray)
		dc.DrawStringAnchored(s.label, cx, y+14, 0.5, 0.5)
		g.loadFont(dc, 16)
		dc.SetColor(colorWhite)
		dc.DrawStringAnchored(s.value, cx, y+36, 0.5, 0.5)
	}

	y += rowH + 10 + 4

	// ── Stats grid row 2: Score | Hero record | Lane ──────────────────
	dc.SetColor(colorRowAlt)
	dc.DrawRectangle(0, y, w, rowH+10)
	dc.Fill()

	score := fmt.Sprintf("Radiant %d — %d Dire", d.RadiantScore, d.DireScore)
	stats2 := []statCell{
		{"Score", score},
		{"Récord héroe (últ.20)", d.HeroRecord},
		{"Lane / Rol", d.LaneInfo},
	}
	cellW2 := w / float64(len(stats2))
	for idx, s := range stats2 {
		cx := float64(idx)*cellW2 + cellW2/2
		g.loadFont(dc, 11)
		dc.SetColor(colorGray)
		dc.DrawStringAnchored(s.label, cx, y+14, 0.5, 0.5)
		g.loadFont(dc, 15)
		dc.SetColor(colorWhite)
		dc.DrawStringAnchored(s.value, cx, y+36, 0.5, 0.5)
	}

	y += rowH + 10 + 4

	// ── Lane outcome (optional) ───────────────────────────────────────
	if d.LaneOutcome != "" {
		dc.SetColor(colorPanel)
		dc.DrawRectangle(0, y, w, rowH)
		dc.Fill()
		g.loadFont(dc, 13)
		dc.SetColor(colorBlue)
		dc.DrawStringAnchored("Resultado línea:", paddingL, y+rowH/2, 0, 0.5)
		dc.SetColor(colorWhite)
		dc.DrawStringAnchored(d.LaneOutcome, paddingL+160, y+rowH/2, 0, 0.5)
		y += rowH + 4
	}

	// ── Damage row (optional) ─────────────────────────────────────────
	if d.HeroDamage > 0 || d.TowerDamage > 0 || d.HeroHealing > 0 {
		dc.SetColor(colorRowAlt)
		dc.DrawRectangle(0, y, w, rowH)
		dc.Fill()

		dmgStats := []statCell{}
		if d.HeroDamage > 0 {
			dmgStats = append(dmgStats, statCell{"Hero Damage", fmt.Sprintf("%d", d.HeroDamage)})
		}
		if d.TowerDamage > 0 {
			dmgStats = append(dmgStats, statCell{"Tower Damage", fmt.Sprintf("%d", d.TowerDamage)})
		}
		if d.HeroHealing > 0 {
			dmgStats = append(dmgStats, statCell{"Healing", fmt.Sprintf("%d", d.HeroHealing)})
		}
		if len(dmgStats) > 0 {
			dw := w / float64(len(dmgStats))
			for idx, s := range dmgStats {
				cx := float64(idx)*dw + dw/2
				g.loadFont(dc, 11)
				dc.SetColor(colorGray)
				dc.DrawStringAnchored(s.label, cx, y+12, 0.5, 0.5)
				g.loadFont(dc, 15)
				dc.SetColor(colorWhite)
				dc.DrawStringAnchored(s.value, cx, y+32, 0.5, 0.5)
			}
		}
		y += rowH + 4
	}

	// ── Footer ────────────────────────────────────────────────────────
	dc.SetColor(colorPanel)
	dc.DrawRectangle(0, y, w, footerH+8)
	dc.Fill()

	g.loadFont(dc, 12)
	dc.SetColor(colorGray)
	ts := d.UpdatedAt.UTC().Format("Mon Jan 02 15:04 UTC")
	leftText := fmt.Sprintf("%s  ·  Match ID: %d", ts, d.MatchID)
	if d.Streak != "" {
		leftText = fmt.Sprintf("%s  ·  %s  ·  Match ID: %d", d.Streak, ts, d.MatchID)
	}
	dc.DrawStringAnchored(leftText, paddingL, y+float64(footerH+8)/2, 0, 0.5)
	dc.SetColor(colorGold)
	dc.DrawStringAnchored("Stratz API", w-paddingL, y+float64(footerH+8)/2, 1, 0.5)

	// Gold border
	dc.SetColor(colorGold)
	dc.SetLineWidth(1.5)
	dc.DrawRectangle(0, 0, w, float64(dc.Height()))
	dc.Stroke()

	var buf bytes.Buffer
	if err := dc.EncodePNG(&buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
