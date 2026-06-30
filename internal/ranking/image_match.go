package ranking

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	stdraw "image/draw"
	_ "image/jpeg"
	_ "image/png"
	"strings"
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
	IMP        int    // Individual Match Performance from Stratz (0 = not available)
	ShowIMP    bool   // true when Stratz returned a non-zero IMP
	Award      string // "MVP", "TOP_CORE", "TOP_SUPPORT", "NONE" or ""
}

// MatchRenderData holds all display data for a match notification PNG.
type MatchRenderData struct {
	PlayerName      string
	HeroName        string
	GameMode        string
	RankBracket     string
	IsWin           bool
	KDA             string
	Duration        string
	Level           int
	RadiantScore    int
	DireScore       int
	GPM             int
	XPM             int
	IMP             string
	HeroRecord      string
	LaneInfo        string
	LaneOutcome     string
	HeroDamage      int
	TowerDamage     int
	HeroHealing     int
	Streak          string
	MatchID         int64
	AnalysisOutcome string
	UpdatedAt       time.Time

	// Images (nil = skip / show fallback)
	HeroImgBytes []byte
	AvatarBytes  []byte

	// 5v5 player list (may be empty)
	RadiantPlayers []MatchPlayer
	DirePlayers    []MatchPlayer
	RadiantWon     bool // true = Radiant won; false = Dire won

	// Raw Stratz lane outcomes for colored rendering
	TopLaneOutcome    string // TIE, RADIANT_VICTORY, RADIANT_STOMP, DIRE_VICTORY, DIRE_STOMP
	MidLaneOutcome    string
	BottomLaneOutcome string
}

// RenderMatch generates a rich PNG card for a match notification.
func (g *ImageGenerator) RenderMatch(d MatchRenderData) ([]byte, error) {
	const w = canvasW

	// Hero header: left = 160×160 square image; right = dark bg + text + badge
	const heroH = 160.0
	const heroSquareW = 160.0

	// Stats bar
	const statsH = 72.0
	// Details row
	const detailH = 60.0
	// Damage row
	const dmgH = 52.0
	// Player list header + rows
	const plHeaderH = 28.0
	const plRowH = 30.0
	// Footer
	const footH = 42.0

	// Lane section: dynamic height based on line count
	var laneLines []string
	laneH := 0.0
	if strings.TrimSpace(d.LaneOutcome) != "" {
		laneLines = strings.Split(strings.TrimSpace(d.LaneOutcome), "\n")
		laneH = 10.0 + float64(len(laneLines))*16.0
		if laneH < 42 {
			laneH = 42
		}
	}

	maxRows := len(d.RadiantPlayers)
	if len(d.DirePlayers) > maxRows {
		maxRows = len(d.DirePlayers)
	}
	hasPlayers := maxRows > 0

	// Count section separators (each adds 2px gap)
	gaps := 3.0 // hero→stats, stats→detail, detail→next
	h := heroH + statsH + detailH
	if laneH > 0 {
		h += laneH
		gaps++
	}
	if d.HeroDamage > 0 || d.TowerDamage > 0 || d.HeroHealing > 0 {
		h += dmgH
		gaps++
	}
	if hasPlayers {
		h += plHeaderH + float64(maxRows)*plRowH
		gaps++
	}
	h += footH + gaps*2

	dc := gg.NewContext(int(w), int(h))
	g.drawBackground(dc)

	resultColor := colorGreen
	resultText := "VICTORIA"
	if !d.IsWin {
		resultColor = colorRed
		resultText = "DERROTA"
	}

	// Match outcome icon from Stratz AnalysisOutcome
	outcomeIcon := ""
	switch strings.ToUpper(d.AnalysisOutcome) {
	case "STOMPED":
		outcomeIcon = " 💥"
	case "COMEBACK":
		outcomeIcon = " 🔄"
	case "CLOSE_GAME":
		outcomeIcon = " ⚔️"
	}

	y := 0.0

	// ── Hero header: square image left + dark bg right ────────────────────
	// Right-side dark background (full width first, hero image will overlay left)
	dc.SetColor(colorPanel)
	dc.DrawRectangle(0, y, w, heroH)
	dc.Fill()

	// Hero image — square, cover-crop to heroSquareW × heroH
	if len(d.HeroImgBytes) > 0 {
		if heroImg, _, err := image.Decode(bytes.NewReader(d.HeroImgBytes)); err == nil {
			sq := scaleToSquare(heroImg, int(heroSquareW), int(heroH))
			dc.DrawImage(sq, 0, int(y))
		}
	} else {
		// Fallback: dark panel with hero name
		dc.SetColor(colorBG)
		dc.DrawRectangle(0, y, heroSquareW, heroH)
		dc.Fill()
		g.loadFont(dc, 12)
		dc.SetColor(colorGold)
		dc.DrawStringAnchored(truncateStr(d.HeroName, 10), heroSquareW/2, y+heroH/2, 0.5, 0.5)
	}
	// Gold border around hero square (right + bottom edges)
	dc.SetColor(colorGold)
	dc.SetLineWidth(1.5)
	dc.DrawRectangle(0.75, y+0.75, heroSquareW-0.75, heroH-1.5)
	dc.Stroke()

	// Result color accent strip at bottom of header
	dc.SetColor(resultColor)
	dc.DrawRectangle(0, y+heroH-3, w, 3)
	dc.Fill()

	// Result badge (top-right of header)
	const badgeW = 190.0
	const badgeH = 56.0
	badgeX := w - badgeW - paddingL
	badgeY := y + (heroH-badgeH)/2
	dc.SetColor(resultColor)
	dc.DrawRoundedRectangle(badgeX, badgeY, badgeW, badgeH, 8)
	dc.Fill()
	g.loadFont(dc, 21)
	dc.SetColor(colorBG)
	if d.AnalysisOutcome != "" {
		dc.DrawStringAnchored(resultText, badgeX+badgeW/2, badgeY+19, 0.5, 0.5)
		g.loadFont(dc, 13)
		dc.DrawStringAnchored(d.AnalysisOutcome, badgeX+badgeW/2, badgeY+40, 0.5, 0.5)
	} else {
		dc.DrawStringAnchored(resultText, badgeX+badgeW/2, badgeY+badgeH/2, 0.5, 0.5)
	}

	// Player name + rank (right area, left-aligned after hero square)
	textX := heroSquareW + paddingL
	g.loadFont(dc, 21)
	dc.SetColor(colorGold)
	playerLine := d.PlayerName
	if d.RankBracket != "" {
		playerLine = fmt.Sprintf("%s  [%s]", d.PlayerName, d.RankBracket)
	}
	dc.DrawStringAnchored(playerLine, textX, y+55, 0, 0.5)

	// Hero · GameMode
	g.loadFont(dc, 14)
	dc.SetColor(colorWhite)
	dc.DrawStringAnchored(fmt.Sprintf("%s  ·  %s", d.HeroName, d.GameMode), textX, y+95, 0, 0.5)

	y += heroH + 2

	// ── Stats bar (avatar circle + 5 stat cells) ──────────────────────────
	dc.SetColor(colorPanel)
	dc.DrawRectangle(0, y, w, statsH)
	dc.Fill()

	const avatarR = 26.0
	const avatarX = paddingL + avatarR
	avatarCY := y + statsH/2

	if len(d.AvatarBytes) > 0 {
		if avatarImg, _, err := image.Decode(bytes.NewReader(d.AvatarBytes)); err == nil {
			dc.DrawCircle(avatarX, avatarCY, avatarR)
			dc.Clip()
			scaled := scaleImage(avatarImg, int(avatarR*2), int(avatarR*2))
			dc.DrawImageAnchored(scaled, int(avatarX), int(avatarCY), 0.5, 0.5)
			dc.ResetClip()
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
	// Gold avatar border
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

	// ── Details row: Score | Hero record | Lane ────────────────────────────
	dc.SetColor(colorRowAlt)
	dc.DrawRectangle(0, y, w, detailH)
	dc.Fill()

	score := fmt.Sprintf("Radiant %d — %d Dire%s", d.RadiantScore, d.DireScore, outcomeIcon)
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

	// ── Lane outcome (optional, multiline) ────────────────────────────────
	if laneH > 0 {
		dc.SetColor(colorPanel)
		dc.DrawRectangle(0, y, w, laneH)
		dc.Fill()
		g.loadFont(dc, 12)
		lineStep := 16.0
		baseY := y + (laneH-float64(len(laneLines))*lineStep)/2 + lineStep*0.5
		rawOutcomes := []string{d.TopLaneOutcome, d.MidLaneOutcome, d.BottomLaneOutcome}
		for i, line := range laneLines {
			lineY := baseY + float64(i)*lineStep
			if idx := strings.Index(line, ": "); idx >= 0 {
				label := line[:idx+2]
				dc.SetColor(colorGray)
				lw, _ := dc.MeasureString(label)
				dc.DrawStringAnchored(label, paddingL*2, lineY, 0, 0.5)
				raw := ""
				if i < len(rawOutcomes) {
					raw = rawOutcomes[i]
				}
				dc.SetColor(laneOutcomeColor(raw))
				dc.DrawStringAnchored(laneOutcomeValue(raw), paddingL*2+lw, lineY, 0, 0.5)
			} else {
				dc.SetColor(colorWhite)
				dc.DrawStringAnchored(line, paddingL*2, lineY, 0, 0.5)
			}
		}
		y += laneH + 2
	}

	// ── Damage row (optional) ─────────────────────────────────────────────
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

	// ── 5v5 Player list ───────────────────────────────────────────────────
	if hasPlayers {
		// Winner = bright, loser = dim
		radiantAlpha, direAlpha := 0.70, 0.30
		if !d.RadiantWon {
			radiantAlpha, direAlpha = 0.30, 0.70
		}

		// Radiant header
		dc.SetRGBA(0.08, 0.42, 0.12, radiantAlpha)
		dc.DrawRectangle(0, y, w/2, plHeaderH)
		dc.Fill()
		// Dire header
		dc.SetRGBA(0.50, 0.10, 0.10, direAlpha)
		dc.DrawRectangle(w/2, y, w/2, plHeaderH)
		dc.Fill()

		radiantLabel := "☀  RADIANT"
		direLabel := "🌙  DIRE"
		if outcomeIcon != "" {
			if d.RadiantWon {
				radiantLabel += outcomeIcon
			} else {
				direLabel += outcomeIcon
			}
		}

		g.loadFont(dc, 12)
		dc.SetColor(colorWhite)
		dc.DrawStringAnchored(radiantLabel, w/4, y+plHeaderH/2, 0.5, 0.5)
		dc.DrawStringAnchored(direLabel, w*3/4, y+plHeaderH/2, 0.5, 0.5)

		// Divider line
		dc.SetColor(colorGold)
		dc.SetLineWidth(0.5)
		dc.DrawLine(w/2, y, w/2, y+plHeaderH+float64(maxRows)*plRowH)
		dc.Stroke()

		y += plHeaderH

		for i := 0; i < maxRows; i++ {
			rowY := y + float64(i)*plRowH
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

	// ── Footer ────────────────────────────────────────────────────────────
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

	// Gold border — inset by half stroke width so it's fully inside the canvas
	const borderW = 1.5
	dc.SetColor(colorGold)
	dc.SetLineWidth(borderW)
	dc.DrawRectangle(borderW/2, borderW/2, w-borderW, float64(dc.Height())-borderW)
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

	if p.IsMain {
		dc.SetRGBA(200.0/255, 170.0/255, 110.0/255, 0.12)
		dc.DrawRectangle(startX, rowY, colW, rowH)
		dc.Fill()
		dc.SetColor(colorGold)
		dc.SetLineWidth(0.8)
		dc.DrawRectangle(startX+0.4, rowY+0.4, colW-0.8, rowH-0.8)
		dc.Stroke()
	}

	// Hero name
	g.loadFont(dc, 12)
	if p.IsMain {
		dc.SetColor(colorGold)
	} else {
		dc.SetColor(colorWhite)
	}
	dc.DrawStringAnchored(truncateStr(p.HeroName, 11), startX+8, cy, 0, 0.5)

	// Award pill badge (dedicated column)
	award := strings.ToUpper(p.Award)
	if award != "" && award != "NONE" {
		drawAwardPill(g, dc, award, startX+colW*0.31, cy)
	}

	// Player name
	g.loadFont(dc, 11)
	dc.SetColor(colorGray)
	dc.DrawStringAnchored(truncateStr(p.PlayerName, 12), startX+colW*0.56, cy, 0.5, 0.5)

	// IMP on far right
	g.loadFont(dc, 11)
	wrRight := startX + colW - 6
	if p.ShowIMP {
		impStr := fmt.Sprintf("%+d", p.IMP)
		switch {
		case p.IMP > 0:
			dc.SetColor(colorGreen)
		case p.IMP < 0:
			dc.SetColor(colorRed)
		default:
			dc.SetColor(colorGray)
		}
		dc.DrawStringAnchored(impStr, wrRight, cy, 1, 0.5)
		wrRight -= 40
	}
	// Win rate
	if p.WinRate >= 0 {
		dc.SetColor(winPctColor(p.WinRate))
		wr := fmt.Sprintf("%.0f%%(%d-%d)", p.WinRate, p.Wins, p.Losses)
		dc.DrawStringAnchored(wr, wrRight, cy, 1, 0.5)
	} else {
		dc.SetColor(colorGray)
		dc.DrawStringAnchored("—", wrRight, cy, 1, 0.5)
	}
}

// drawAwardPill draws a colored pill badge (MVP/CORE/SUPPORT) centered at (cx, cy).
func drawAwardPill(g *ImageGenerator, dc *gg.Context, award string, cx, cy float64) {
	const pillW, pillH = 44.0, 15.0

	var bgColor color.RGBA
	var label string
	var textColor color.RGBA

	switch award {
	case "MVP":
		bgColor = color.RGBA{212, 172, 13, 255}
		label = "* MVP"
		textColor = color.RGBA{40, 28, 0, 255}
	case "TOP_CORE":
		bgColor = color.RGBA{210, 105, 30, 255}
		label = "* CORE"
		textColor = color.RGBA{255, 245, 225, 255}
	case "TOP_SUPPORT":
		bgColor = color.RGBA{40, 110, 210, 255}
		label = "* SUPP"
		textColor = color.RGBA{215, 232, 255, 255}
	default:
		return
	}

	dc.SetColor(bgColor)
	dc.DrawRoundedRectangle(cx-pillW/2, cy-pillH/2, pillW, pillH, pillH/2)
	dc.Fill()
	g.loadFont(dc, 8)
	dc.SetColor(textColor)
	dc.DrawStringAnchored(label, cx, cy+0.5, 0.5, 0.5)
}

// laneOutcomeColor returns the display color for a raw Stratz lane outcome.
func laneOutcomeColor(raw string) color.RGBA {
	switch strings.ToUpper(raw) {
	case "RADIANT_VICTORY", "RADIANT_STOMP":
		return colorGreen
	case "DIRE_VICTORY", "DIRE_STOMP":
		return colorRed
	default:
		return colorGray
	}
}

// laneOutcomeValue returns the display text for a raw Stratz lane outcome.
func laneOutcomeValue(raw string) string {
	switch strings.ToUpper(raw) {
	case "RADIANT_VICTORY":
		return "Victoria Rad."
	case "RADIANT_STOMP":
		return "Stomp Rad."
	case "DIRE_VICTORY":
		return "Victoria Dire"
	case "DIRE_STOMP":
		return "Stomp Dire"
	case "TIE":
		return "Empate"
	default:
		return "—"
	}
}

// scaleImage scales src to dstW×dstH using bilinear interpolation.
func scaleImage(src image.Image, dstW, dstH int) image.Image {
	dst := image.NewRGBA(image.Rect(0, 0, dstW, dstH))
	xdraw.BiLinear.Scale(dst, dst.Bounds(), src, src.Bounds(), xdraw.Over, nil)
	return dst
}

// scaleToSquare scales src to fill dstW×dstH preserving aspect ratio (cover crop).
// Returns an image with Bounds at (0,0) so gg.DrawImage positions it correctly.
func scaleToSquare(src image.Image, dstW, dstH int) image.Image {
	srcW := src.Bounds().Dx()
	srcH := src.Bounds().Dy()
	scaleX := float64(dstW) / float64(srcW)
	scaleY := float64(dstH) / float64(srcH)
	scale := scaleX
	if scaleY > scaleX {
		scale = scaleY
	}
	newW := int(float64(srcW)*scale + 0.5)
	newH := int(float64(srcH)*scale + 0.5)
	if newW < dstW {
		newW = dstW
	}
	if newH < dstH {
		newH = dstH
	}
	// Scale to intermediate size
	scaled := image.NewRGBA(image.Rect(0, 0, newW, newH))
	xdraw.BiLinear.Scale(scaled, scaled.Bounds(), src, src.Bounds(), xdraw.Over, nil)
	// Crop center: copy to a (0,0)-based dst so gg places it correctly
	offX := (newW - dstW) / 2
	offY := (newH - dstH) / 2
	dst := image.NewRGBA(image.Rect(0, 0, dstW, dstH))
	stdraw.Draw(dst, dst.Bounds(), scaled, image.Point{offX, offY}, stdraw.Src)
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
