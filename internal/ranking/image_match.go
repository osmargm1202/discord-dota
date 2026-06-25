package ranking

import (
	"bytes"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"time"

	"github.com/fogleman/gg"
	xdraw "golang.org/x/image/draw"
)

// MatchPlayer holds data for one player in the 5v5 list.
type MatchPlayer struct {
	HeroName   string
	PlayerName string
	IsMain     bool
	WinRate    float64 // -1 = unknown
	Wins       int
	Losses     int
}

// MatchRenderData holds all display data for a match notification PNG.
type MatchRenderData struct {
	PlayerName     string
	HeroName       string
	GameMode       string
	RankBracket    string
	IsWin          bool
	KDA            string
	Duration       string
	Level          int
	RadiantScore   int
	DireScore      int
	GPM            int
	XPM            int
	IMP            string
	HeroRecord     string
	LaneInfo       string
	LaneOutcome    string
	HeroDamage     int
	TowerDamage    int
	HeroHealing    int
	Streak         string
	MatchID        int64
	AnalysisOutcome string
	UpdatedAt      time.Time

	// Images (nil = skip / show fallback)
	HeroImgBytes  []byte
	AvatarBytes   []byte

	// 5v5 player list (may be empty)
	RadiantPlayers []MatchPlayer
	DirePlayers    []MatchPlayer
}

// RenderMatch generates a rich PNG card for a match notification.
func (g *ImageGenerator) RenderMatch(d MatchRenderData) ([]byte, error) {
	const w = canvasW
	const heroH = 160.0
	const statsH = 72.0
	const detailH = 60.0
	const laneH = 42.0
	const dmgH = 52.0
	const plHeaderH = 28.0
	const plRowH = 30.0
	const footH = 42.0

	maxRows := len(d.RadiantPlayers)
	if len(d.DirePlayers) > maxRows {
		maxRows = len(d.DirePlayers)
	}
	hasPlayers := maxRows > 0

	h := heroH + statsH + detailH
	if d.LaneOutcome != "" {
		h += laneH
	}
	if d.HeroDamage > 0 || d.TowerDamage > 0 || d.HeroHealing > 0 {
		h += dmgH
	}
	if hasPlayers {
		h += plHeaderH + float64(maxRows)*plRowH
	}
	h += footH

	dc := gg.NewContext(int(w), int(h))
	g.drawBackground(dc)

	resultColor := colorGreen
	resultText := "✅  VICTORIA"
	if !d.IsWin {
		resultColor = colorRed
		resultText = "❌  DERROTA"
	}

	y := 0.0

	// ── Hero art header ───────────────────────────────────────────────
	if len(d.HeroImgBytes) > 0 {
		if heroImg, _, err := image.Decode(bytes.NewReader(d.HeroImgBytes)); err == nil {
			scaled := scaleImage(heroImg, int(w), int(heroH))
			dc.DrawImage(scaled, 0, 0)
		}
	}
	// Dark overlay so text is readable
	dc.SetRGBA(0.05, 0.07, 0.12, 0.72)
	dc.DrawRectangle(0, y, w, heroH)
	dc.Fill()
	// Result color accent bottom strip
	dc.SetColor(resultColor)
	dc.DrawRectangle(0, heroH-4, w, 4)
	dc.Fill()

	// Result badge box (top-right corner of hero header)
	const badgeW = 210.0
	const badgeH = 58.0
	badgeX := w - badgeW - paddingL
	badgeY := y + (heroH-badgeH)/2
	dc.SetColor(resultColor)
	dc.DrawRoundedRectangle(badgeX, badgeY, badgeW, badgeH, 8)
	dc.Fill()
	g.loadFont(dc, 22)
	dc.SetColor(colorBG)
	if d.AnalysisOutcome != "" {
		dc.DrawStringAnchored(resultText, badgeX+badgeW/2, badgeY+20, 0.5, 0.5)
		g.loadFont(dc, 14)
		dc.DrawStringAnchored(d.AnalysisOutcome, badgeX+badgeW/2, badgeY+43, 0.5, 0.5)
	} else {
		dc.DrawStringAnchored(resultText, badgeX+badgeW/2, badgeY+badgeH/2, 0.5, 0.5)
	}

	// Player name + rank (left side of header)
	g.loadFont(dc, 22)
	dc.SetColor(colorGold)
	playerLine := d.PlayerName
	if d.RankBracket != "" {
		playerLine = fmt.Sprintf("%s  [%s]", d.PlayerName, d.RankBracket)
	}
	dc.DrawStringAnchored(playerLine, paddingL, y+70, 0, 0.5)

	// Hero · GameMode
	g.loadFont(dc, 15)
	dc.SetColor(colorWhite)
	dc.DrawStringAnchored(fmt.Sprintf("%s  ·  %s", d.HeroName, d.GameMode), paddingL, y+110, 0, 0.5)

	y += heroH + 2

	// ── Stats bar (avatar circle + 5 stat cells) ──────────────────────
	dc.SetColor(colorPanel)
	dc.DrawRectangle(0, y, w, statsH)
	dc.Fill()

	const avatarR = 26.0
	const avatarX = paddingL + avatarR
	avatarCY := y + statsH/2

	if len(d.AvatarBytes) > 0 {
		if avatarImg, _, err := image.Decode(bytes.NewReader(d.AvatarBytes)); err == nil {
			dc.Push()
			dc.DrawCircle(avatarX, avatarCY, avatarR)
			dc.Clip()
			scaled := scaleImage(avatarImg, int(avatarR*2), int(avatarR*2))
			dc.DrawImageAnchored(scaled, int(avatarX), int(avatarCY), 0.5, 0.5)
			dc.Pop()
		}
	} else {
		// Initials fallback
		dc.SetColor(colorPanel)
		dc.DrawCircle(avatarX, avatarCY, avatarR)
		dc.Fill()
		if len(d.PlayerName) > 0 {
			g.loadFont(dc, 20)
			dc.SetColor(colorGold)
			dc.DrawStringAnchored(string([]rune(d.PlayerName)[0]), avatarX, avatarCY, 0.5, 0.5)
		}
	}
	// Subtle gold avatar border
	dc.SetColor(colorGold)
	dc.SetLineWidth(1.5)
	dc.DrawCircle(avatarX, avatarCY, avatarR)
	dc.Stroke()

	type statCell struct{ label, value string }
	statStartX := avatarX + avatarR + 14
	stats1 := []statCell{
		{"K/D/A", d.KDA},
		{"Duración", d.Duration},
		{"Nivel", fmt.Sprintf("%d", d.Level)},
		{"GPM / XPM", fmt.Sprintf("%d / %d", d.GPM, d.XPM)},
		{"IMP", d.IMP},
	}
	cellW := (w - statStartX) / float64(len(stats1))
	for idx, s := range stats1 {
		cx := statStartX + float64(idx)*cellW + cellW/2
		g.loadFont(dc, 11)
		dc.SetColor(colorGray)
		dc.DrawStringAnchored(s.label, cx, y+20, 0.5, 0.5)
		g.loadFont(dc, 16)
		dc.SetColor(colorWhite)
		dc.DrawStringAnchored(s.value, cx, y+50, 0.5, 0.5)
	}
	y += statsH + 2

	// ── Details row: Score | Hero record | Lane ───────────────────────
	dc.SetColor(colorRowAlt)
	dc.DrawRectangle(0, y, w, detailH)
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
		dc.DrawStringAnchored(s.label, cx, y+16, 0.5, 0.5)
		g.loadFont(dc, 14)
		dc.SetColor(colorWhite)
		dc.DrawStringAnchored(s.value, cx, y+42, 0.5, 0.5)
	}
	y += detailH + 2

	// ── Lane outcome (optional) ───────────────────────────────────────
	if d.LaneOutcome != "" {
		dc.SetColor(colorPanel)
		dc.DrawRectangle(0, y, w, laneH)
		dc.Fill()
		g.loadFont(dc, 13)
		dc.SetColor(colorBlue)
		dc.DrawStringAnchored("Resultado línea:", paddingL, y+laneH/2, 0, 0.5)
		dc.SetColor(colorWhite)
		dc.DrawStringAnchored(d.LaneOutcome, paddingL+180, y+laneH/2, 0, 0.5)
		y += laneH + 2
	}

	// ── Damage row (optional) ─────────────────────────────────────────
	if d.HeroDamage > 0 || d.TowerDamage > 0 || d.HeroHealing > 0 {
		dc.SetColor(colorRowAlt)
		dc.DrawRectangle(0, y, w, dmgH)
		dc.Fill()
		var dmgCells []statCell
		if d.HeroDamage > 0 {
			dmgCells = append(dmgCells, statCell{"Hero Damage", fmt.Sprintf("%d", d.HeroDamage)})
		}
		if d.TowerDamage > 0 {
			dmgCells = append(dmgCells, statCell{"Tower Damage", fmt.Sprintf("%d", d.TowerDamage)})
		}
		if d.HeroHealing > 0 {
			dmgCells = append(dmgCells, statCell{"Healing", fmt.Sprintf("%d", d.HeroHealing)})
		}
		dw := w / float64(len(dmgCells))
		for idx, s := range dmgCells {
			cx := float64(idx)*dw + dw/2
			g.loadFont(dc, 11)
			dc.SetColor(colorGray)
			dc.DrawStringAnchored(s.label, cx, y+15, 0.5, 0.5)
			g.loadFont(dc, 15)
			dc.SetColor(colorWhite)
			dc.DrawStringAnchored(s.value, cx, y+37, 0.5, 0.5)
		}
		y += dmgH + 2
	}

	// ── 5v5 Player list ───────────────────────────────────────────────
	if hasPlayers {
		// Radiant header
		dc.SetRGBA(0.08, 0.42, 0.12, 0.70)
		dc.DrawRectangle(0, y, w/2, plHeaderH)
		dc.Fill()
		// Dire header
		dc.SetRGBA(0.50, 0.10, 0.10, 0.70)
		dc.DrawRectangle(w/2, y, w/2, plHeaderH)
		dc.Fill()

		g.loadFont(dc, 12)
		dc.SetColor(colorWhite)
		dc.DrawStringAnchored("☀  RADIANT", w/4, y+plHeaderH/2, 0.5, 0.5)
		dc.DrawStringAnchored("🌙  DIRE", w*3/4, y+plHeaderH/2, 0.5, 0.5)

		// Divider line
		dc.SetColor(colorGold)
		dc.SetLineWidth(0.5)
		dc.DrawLine(w/2, y, w/2, y+plHeaderH+float64(maxRows)*plRowH)
		dc.Stroke()

		y += plHeaderH

		for i := 0; i < maxRows; i++ {
			rowY := y + float64(i)*plRowH
			// Alternating row bg
			if i%2 == 1 {
				dc.SetRGBA(17.0/255, 24.0/255, 39.0/255, 0.45)
				dc.DrawRectangle(0, rowY, w, plRowH)
				dc.Fill()
			}
			if i < len(d.RadiantPlayers) {
				drawMatchPlayerRow(g, dc, d.RadiantPlayers[i], 0, w/2, rowY, plRowH)
			}
			if i < len(d.DirePlayers) {
				drawMatchPlayerRow(g, dc, d.DirePlayers[i], w/2, w/2, rowY, plRowH)
			}
		}
		y += float64(maxRows)*plRowH + 2
	}

	// ── Footer ────────────────────────────────────────────────────────
	dc.SetColor(colorPanel)
	dc.DrawRectangle(0, y, w, footH)
	dc.Fill()

	g.loadFont(dc, 12)
	dc.SetColor(colorGray)
	ts := d.UpdatedAt.UTC().Format("Mon Jan 02 15:04 UTC")
	leftText := fmt.Sprintf("%s  ·  Match ID: %d", ts, d.MatchID)
	if d.Streak != "" {
		leftText = fmt.Sprintf("%s  ·  %s  ·  Match ID: %d", d.Streak, ts, d.MatchID)
	}
	dc.DrawStringAnchored(leftText, paddingL, y+footH/2, 0, 0.5)
	dc.SetColor(colorGold)
	dc.DrawStringAnchored("Stratz", w-paddingL, y+footH/2, 1, 0.5)

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

// drawMatchPlayerRow renders one player entry in the 5v5 table.
func drawMatchPlayerRow(g *ImageGenerator, dc *gg.Context, p MatchPlayer, startX, colW, rowY, rowH float64) {
	cy := rowY + rowH/2

	// Highlight main player
	if p.IsMain {
		dc.SetRGBA(200.0/255, 170.0/255, 110.0/255, 0.12)
		dc.DrawRectangle(startX, rowY, colW, rowH)
		dc.Fill()
		dc.SetColor(colorGold)
		dc.SetLineWidth(0.8)
		dc.DrawRectangle(startX+0.4, rowY+0.4, colW-0.8, rowH-0.8)
		dc.Stroke()
	}

	// Hero name (left 40% of column)
	g.loadFont(dc, 12)
	if p.IsMain {
		dc.SetColor(colorGold)
	} else {
		dc.SetColor(colorWhite)
	}
	hero := truncateStr(p.HeroName, 14)
	dc.DrawStringAnchored(hero, startX+8, cy, 0, 0.5)

	// Player name (center)
	g.loadFont(dc, 11)
	dc.SetColor(colorGray)
	name := truncateStr(p.PlayerName, 12)
	dc.DrawStringAnchored(name, startX+colW*0.52, cy, 0.5, 0.5)

	// Win rate (right)
	g.loadFont(dc, 11)
	if p.WinRate >= 0 {
		dc.SetColor(winPctColor(p.WinRate))
		wr := fmt.Sprintf("%.0f%%(%d-%d)", p.WinRate, p.Wins, p.Losses)
		dc.DrawStringAnchored(wr, startX+colW-6, cy, 1, 0.5)
	} else {
		dc.SetColor(colorGray)
		dc.DrawStringAnchored("—", startX+colW-6, cy, 1, 0.5)
	}
}

// scaleImage scales src to dstW×dstH using bilinear interpolation.
func scaleImage(src image.Image, dstW, dstH int) image.Image {
	dst := image.NewRGBA(image.Rect(0, 0, dstW, dstH))
	xdraw.BiLinear.Scale(dst, dst.Bounds(), src, src.Bounds(), xdraw.Over, nil)
	return dst
}

// truncateStr cuts s to maxRunes and appends "…" if needed.
func truncateStr(s string, maxRunes int) string {
	r := []rune(s)
	if len(r) <= maxRunes {
		return s
	}
	return string(r[:maxRunes-1]) + "…"
}
