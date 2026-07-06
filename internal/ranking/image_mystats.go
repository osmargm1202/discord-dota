package ranking

import (
	"bytes"
	"fmt"

	"github.com/fogleman/gg"
)

// BuildGroupRow is one ability-build group's record for a player+hero+level combo.
type BuildGroupRow struct {
	Label  string // e.g. "1-1-3-1"
	Wins   int
	Losses int
	Draws  int
	Total  int
}

// MyHeroStatsRenderData holds data for the /dota mystats PNG card.
type MyHeroStatsRenderData struct {
	PlayerName     string
	AvatarBytes    []byte
	HeroName       string
	HeroImageBytes []byte
	Level          int
	Groups         []BuildGroupRow // sorted by Total desc
	TotalGames     int
}

const (
	myStatsHeaderH  = 76.0
	myStatsColHeadH = 28.0
	myStatsRowH     = 26.0
	myStatsFooterH  = 28.0
	myStatsPadV     = 10.0
)

// RenderMyHeroStats generates a PNG showing win/loss record grouped by
// ability-point allocation (Q-W-E-R) at a given hero level.
func (g *ImageGenerator) RenderMyHeroStats(d MyHeroStatsRenderData) ([]byte, error) {
	const w = canvasW
	totalH := int(myStatsHeaderH + myStatsPadV + myStatsColHeadH + float64(len(d.Groups))*myStatsRowH + myStatsPadV + myStatsFooterH)

	dc := gg.NewContext(w, totalH)
	dc.SetColor(colorBG)
	dc.Clear()

	g.drawMyStatsHeader(dc, d, myStatsHeaderH)
	g.drawBuildGroupTable(dc, d.Groups, myStatsHeaderH+myStatsPadV, myStatsColHeadH, myStatsRowH)

	fy := float64(totalH) - myStatsFooterH
	dc.SetColor(colorPanel)
	dc.DrawRectangle(0, fy, w, myStatsFooterH)
	dc.Fill()
	g.loadFont(dc, 10)
	dc.SetColor(colorGray)
	footer := fmt.Sprintf("%d partidas analizadas  •  %s  •  Nivel %d  •  Stratz", d.TotalGames, d.HeroName, d.Level)
	dc.DrawStringAnchored(footer, w/2, fy+myStatsFooterH/2, 0.5, 0.5)

	var buf bytes.Buffer
	if err := dc.EncodePNG(&buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func (g *ImageGenerator) drawMyStatsHeader(dc *gg.Context, d MyHeroStatsRenderData, h float64) {
	dc.SetColor(colorPanel)
	dc.DrawRectangle(0, 0, canvasW, h)
	dc.Fill()

	const cr = 26.0
	cx, cy := 40.0, h/2

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
	}
	dc.SetColor(colorGold)
	dc.SetLineWidth(1.5)
	dc.DrawCircle(cx, cy, cr)
	dc.Stroke()

	const hs = 52.0
	hx, hy := cx+cr+20, cy-hs/2
	if len(d.HeroImageBytes) > 0 {
		if img, _, err := decodeImage(d.HeroImageBytes); err == nil {
			scaled := scaleImage(img, int(hs), int(hs))
			dc.DrawImageAnchored(scaled, int(hx+hs/2), int(cy), 0.5, 0.5)
		}
	}
	dc.SetColor(colorGold)
	dc.SetLineWidth(1)
	dc.DrawRectangle(hx, hy, hs, hs)
	dc.Stroke()

	tx := hx + hs + 16
	g.loadFont(dc, 18)
	dc.SetColor(colorGold)
	dc.DrawStringAnchored(fmt.Sprintf("%s — %s", d.PlayerName, d.HeroName), tx, cy-8, 0, 0.5)

	g.loadFont(dc, 11)
	dc.SetColor(colorGray)
	dc.DrawStringAnchored(fmt.Sprintf("Build de habilidades en nivel %d", d.Level), tx, cy+10, 0, 0.5)

	dc.SetColor(colorGold)
	dc.SetLineWidth(1)
	dc.DrawLine(0, h-1, canvasW, h-1)
	dc.Stroke()
}

func (g *ImageGenerator) drawBuildGroupTable(dc *gg.Context, rows []BuildGroupRow, y, headH, rH float64) {
	const w = canvasW

	dc.SetColor(colorPanel)
	dc.DrawRectangle(0, y, w, headH)
	dc.Fill()
	g.loadFont(dc, 11)
	dc.SetColor(colorGray)
	dc.DrawStringAnchored("Build (Q-W-E-R)", w*0.06, y+headH/2, 0, 0.5)
	dc.DrawStringAnchored("G", w*0.45, y+headH/2, 0.5, 0.5)
	dc.DrawStringAnchored("W", w*0.58, y+headH/2, 0.5, 0.5)
	dc.DrawStringAnchored("L", w*0.68, y+headH/2, 0.5, 0.5)
	dc.DrawStringAnchored("E", w*0.78, y+headH/2, 0.5, 0.5)
	dc.DrawStringAnchored("%", w*0.90, y+headH/2, 0.5, 0.5)

	ry := y + headH
	for idx, row := range rows {
		if idx%2 == 1 {
			dc.SetColor(colorRowAlt)
			dc.DrawRectangle(0, ry, w, rH)
			dc.Fill()
		}
		pct := 0.0
		if row.Total > 0 {
			pct = float64(row.Wins) / float64(row.Total) * 100
		}
		g.loadFont(dc, 13)
		dc.SetColor(colorWhite)
		dc.DrawStringAnchored(row.Label, w*0.06, ry+rH/2, 0, 0.5)
		dc.DrawStringAnchored(fmt.Sprintf("%d", row.Total), w*0.45, ry+rH/2, 0.5, 0.5)
		dc.SetColor(colorGreen)
		dc.DrawStringAnchored(fmt.Sprintf("%d", row.Wins), w*0.58, ry+rH/2, 0.5, 0.5)
		dc.SetColor(colorRed)
		dc.DrawStringAnchored(fmt.Sprintf("%d", row.Losses), w*0.68, ry+rH/2, 0.5, 0.5)
		dc.SetColor(colorGray)
		dc.DrawStringAnchored(fmt.Sprintf("%d", row.Draws), w*0.78, ry+rH/2, 0.5, 0.5)
		dc.SetColor(colorGold)
		dc.DrawStringAnchored(fmt.Sprintf("%.0f%%", pct), w*0.90, ry+rH/2, 0.5, 0.5)
		ry += rH
	}
}
