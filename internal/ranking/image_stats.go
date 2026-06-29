package ranking

import (
	"bytes"
	"fmt"
	"image/color"
	"math"

	"github.com/fogleman/gg"
)

// HeroStatRow is one hero's stats for a player.
type HeroStatRow struct {
	HeroName   string
	WinCount   int
	MatchCount int
}

// PlayerStatsRenderData holds data for one player's hero stats PNG.
type PlayerStatsRenderData struct {
	PlayerName  string
	AvatarBytes []byte
	Heroes      []HeroStatRow // sorted by WinPct desc
	TotalGames  int           // games analyzed
	MinGames    int           // min games per hero threshold
}

// RenderPlayerStats generates a PNG showing hero stats in 3 groups (red/yellow/green).
func (g *ImageGenerator) RenderPlayerStats(d PlayerStatsRenderData) ([]byte, error) {
	const w = canvasW

	// Split heroes into 3 groups
	type group struct {
		label string
		col   color.RGBA
		rows  []HeroStatRow
	}
	groups := [3]group{
		{label: "< 40%  ❌", col: colorRed},
		{label: "40 – 50%  ➖", col: color.RGBA{212, 172, 13, 255}},
		{label: "> 50%  ✅", col: colorGreen},
	}
	for _, h := range d.Heroes {
		pct := 0.0
		if h.MatchCount > 0 {
			pct = float64(h.WinCount) / float64(h.MatchCount) * 100
		}
		switch {
		case pct < 40:
			groups[0].rows = append(groups[0].rows, h)
		case pct <= 50:
			groups[1].rows = append(groups[1].rows, h)
		default:
			groups[2].rows = append(groups[2].rows, h)
		}
	}

	// Calculate height
	const (
		headerH  = 64.0
		colHeadH = 28.0
		heroRowH = 22.0
		footerH  = 28.0
		padV     = 10.0
		colW     = w / 3.0
	)
	maxRows := 0
	for _, grp := range groups {
		if len(grp.rows) > maxRows {
			maxRows = len(grp.rows)
		}
	}
	totalH := int(headerH + padV + colHeadH + float64(maxRows)*heroRowH + padV + footerH)

	dc := gg.NewContext(w, totalH)

	// Background
	dc.SetColor(colorBG)
	dc.Clear()

	// Header
	g.drawStatsHeader(dc, d, headerH)

	// 3 column groups
	y := headerH + padV
	for ci, grp := range groups {
		x := float64(ci) * colW
		g.drawStatsGroup(dc, grp.label, grp.col, grp.rows, x, colW, y, colHeadH, heroRowH)
	}

	// Footer
	fy := float64(totalH) - footerH
	dc.SetColor(colorPanel)
	dc.DrawRectangle(0, fy, w, footerH)
	dc.Fill()
	g.loadFont(dc, 10)
	dc.SetColor(colorGray)
	footer := fmt.Sprintf("%d partidas analizadas  •  ≥%d partidas por héroe  •  Stratz", d.TotalGames, d.MinGames)
	dc.DrawStringAnchored(footer, w/2, fy+footerH/2, 0.5, 0.5)

	var buf bytes.Buffer
	if err := dc.EncodePNG(&buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func (g *ImageGenerator) drawStatsHeader(dc *gg.Context, d PlayerStatsRenderData, h float64) {
	// Dark panel
	dc.SetColor(colorPanel)
	dc.DrawRectangle(0, 0, canvasW, h)
	dc.Fill()

	// Avatar circle
	const cr = 22.0
	cx, cy := 36.0, h/2

	if len(d.AvatarBytes) > 0 {
		if img, _, err := decodeImage(d.AvatarBytes); err == nil {
			dc.DrawCircle(cx, cy, cr)
			dc.Clip()
			scaled := scaleImage(img, int(cr*2), int(cr*2))
			dc.DrawImageAnchored(scaled, int(cx), int(cy), 0.5, 0.5)
			dc.ResetClip()
		}
	} else {
		dc.SetColor(colorGold)
		dc.DrawCircle(cx, cy, cr)
		dc.Fill()
		g.loadFont(dc, 16)
		dc.SetColor(colorBG)
		initial := "?"
		if len(d.PlayerName) > 0 {
			initial = string([]rune(d.PlayerName)[0])
		}
		dc.DrawStringAnchored(initial, cx, cy, 0.5, 0.5)
	}

	// Gold border on avatar
	dc.SetColor(colorGold)
	dc.SetLineWidth(1.5)
	dc.DrawCircle(cx, cy, cr)
	dc.Stroke()

	// Player name
	g.loadFont(dc, 18)
	dc.SetColor(colorGold)
	dc.DrawStringAnchored(d.PlayerName, cx+cr+12, cy-8, 0, 0.5)

	// Subtitle
	g.loadFont(dc, 11)
	dc.SetColor(colorGray)
	dc.DrawStringAnchored("Estadísticas por héroe", cx+cr+12, cy+10, 0, 0.5)

	// Gold bottom border
	dc.SetColor(colorGold)
	dc.SetLineWidth(1)
	dc.DrawLine(0, h-1, canvasW, h-1)
	dc.Stroke()
}

func (g *ImageGenerator) drawStatsGroup(dc *gg.Context, label string, col color.RGBA, rows []HeroStatRow, x, w, y, headH, rowH float64) {
	// Column divider
	dc.SetColor(colorPanel)
	dc.DrawRectangle(x, y, w, headH+float64(len(rows))*rowH)
	dc.Fill()

	// Header bar with color
	dc.SetColor(col)
	dc.SetLineWidth(2)
	dc.DrawLine(x+4, y+headH-2, x+w-4, y+headH-2)
	dc.Stroke()

	g.loadFont(dc, 12)
	dc.SetColor(colorWhite)
	dc.DrawStringAnchored(label, x+w/2, y+headH/2, 0.5, 0.5)

	// Vertical separator
	if x > 0 {
		dc.SetColor(colorGray)
		dc.SetLineWidth(0.5)
		dc.DrawLine(x, y, x, y+headH+float64(len(rows))*rowH)
		dc.Stroke()
	}

	// Hero rows
	for ri, h := range rows {
		ry := y + headH + float64(ri)*rowH
		if ri%2 == 1 {
			dc.SetRGBA255(255, 255, 255, 8)
			dc.DrawRectangle(x, ry, w, rowH)
			dc.Fill()
		}
		pct := 0.0
		if h.MatchCount > 0 {
			pct = float64(h.WinCount) / float64(h.MatchCount) * 100
		}
		losses := h.MatchCount - h.WinCount

		// Thin colored bar on left proportional to win%
		barW := math.Max(2, w*pct/100*0.18)
		dc.SetColor(col)
		dc.SetLineWidth(1)
		dc.DrawLine(x+1, ry+rowH/2, x+1+barW, ry+rowH/2)
		dc.Stroke()

		g.loadFont(dc, 11)
		dc.SetColor(colorWhite)
		dc.DrawStringAnchored(truncateStr(h.HeroName, 13), x+6, ry+rowH/2, 0, 0.5)

		// W-L
		dc.SetColor(colorGray)
		wl := fmt.Sprintf("%d-%d", h.WinCount, losses)
		dc.DrawStringAnchored(wl, x+w*0.72, ry+rowH/2, 0.5, 0.5)

		// Win%
		dc.SetColor(winPctColor(pct))
		dc.DrawStringAnchored(fmt.Sprintf("%.0f%%", pct), x+w-5, ry+rowH/2, 1, 0.5)
	}
}
