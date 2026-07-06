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
	Total  int
	// Lane outcome counts contrast the build against how often the
	// player's own lane was won/lost/tied (independent of match result).
	LaneWins   int
	LaneLosses int
	LaneTies   int
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
	myStatsHeaderH   = 76.0
	myStatsGroupLblH = 14.0
	myStatsColHeadH  = 26.0
	myStatsRowH      = 26.0
	myStatsFooterH   = 28.0
	myStatsPadV      = 10.0

	// Hero portrait box: widened to roughly match Steam's hero image
	// aspect ratio (~16:9) so the cover-crop below doesn't need to trim
	// much off the sides.
	myStatsHeroW = 92.0
	myStatsHeroH = 52.0
)

// RenderMyHeroStats generates a PNG showing win/loss record grouped by
// ability-point allocation (Q-W-E-R) at a given hero level, contrasted
// against lane outcome (won/lost/tied).
func (g *ImageGenerator) RenderMyHeroStats(d MyHeroStatsRenderData) ([]byte, error) {
	const w = canvasW
	colHeadTotalH := myStatsGroupLblH + myStatsColHeadH
	totalH := int(myStatsHeaderH + myStatsPadV + colHeadTotalH + float64(len(d.Groups))*myStatsRowH + myStatsPadV + myStatsFooterH)

	dc := gg.NewContext(w, totalH)
	dc.SetColor(colorBG)
	dc.Clear()

	g.drawMyStatsHeader(dc, d, myStatsHeaderH)
	g.drawBuildGroupTable(dc, d.Groups, myStatsHeaderH+myStatsPadV, myStatsRowH)

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

	hx, hy := cx+cr+20, cy-myStatsHeroH/2
	if len(d.HeroImageBytes) > 0 {
		if img, _, err := decodeImage(d.HeroImageBytes); err == nil {
			// scaleToSquare cover-crops preserving aspect ratio (no
			// stretch); the box width above is chosen to closely match
			// Steam's hero-image aspect ratio so the crop is minimal.
			scaled := scaleToSquare(img, int(myStatsHeroW), int(myStatsHeroH))
			dc.DrawImage(scaled, int(hx), int(hy))
		}
	}
	dc.SetColor(colorGold)
	dc.SetLineWidth(1)
	dc.DrawRectangle(hx, hy, myStatsHeroW, myStatsHeroH)
	dc.Stroke()

	tx := hx + myStatsHeroW + 16
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

// Column x-fractions (of canvasW) for the build-group table.
const (
	colBuildX   = 0.03
	colGameX    = 0.26
	colWinX     = 0.33
	colLossX    = 0.40
	colPctX     = 0.47
	colLaneWX   = 0.62
	colLaneLX   = 0.69
	colLaneEX   = 0.76
	colLanePctX = 0.85
	grpMatchX   = 0.36 // "PARTIDA" group label, centered over W/L/%
	grpLaneX    = 0.71 // "LÍNEA" group label, centered over LnW/LnL/LnE/%
)

func (g *ImageGenerator) drawBuildGroupTable(dc *gg.Context, rows []BuildGroupRow, y, rH float64) {
	const w = canvasW
	groupLblH := myStatsGroupLblH
	colHeadH := myStatsColHeadH
	headH := groupLblH + colHeadH

	dc.SetColor(colorPanel)
	dc.DrawRectangle(0, y, w, headH)
	dc.Fill()

	g.loadFont(dc, 9)
	dc.SetColor(colorGray)
	dc.DrawStringAnchored("PARTIDA", w*grpMatchX, y+groupLblH/2, 0.5, 0.5)
	dc.DrawStringAnchored("LÍNEA", w*grpLaneX, y+groupLblH/2, 0.5, 0.5)

	labelY := y + groupLblH + colHeadH/2
	g.loadFont(dc, 11)
	dc.SetColor(colorGray)
	dc.DrawStringAnchored("Build (Q-W-E-R)", w*colBuildX, y+headH/2, 0, 0.5)
	dc.DrawStringAnchored("G", w*colGameX, labelY, 0.5, 0.5)
	dc.DrawStringAnchored("W", w*colWinX, labelY, 0.5, 0.5)
	dc.DrawStringAnchored("L", w*colLossX, labelY, 0.5, 0.5)
	dc.DrawStringAnchored("%", w*colPctX, labelY, 0.5, 0.5)
	dc.DrawStringAnchored("W", w*colLaneWX, labelY, 0.5, 0.5)
	dc.DrawStringAnchored("L", w*colLaneLX, labelY, 0.5, 0.5)
	dc.DrawStringAnchored("E", w*colLaneEX, labelY, 0.5, 0.5)
	dc.DrawStringAnchored("%", w*colLanePctX, labelY, 0.5, 0.5)

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
		laneTotal := row.LaneWins + row.LaneLosses + row.LaneTies
		lanePct := 0.0
		if laneTotal > 0 {
			lanePct = float64(row.LaneWins) / float64(laneTotal) * 100
		}
		cy := ry + rH/2

		g.loadFont(dc, 13)
		dc.SetColor(colorWhite)
		dc.DrawStringAnchored(row.Label, w*colBuildX, cy, 0, 0.5)
		dc.DrawStringAnchored(fmt.Sprintf("%d", row.Total), w*colGameX, cy, 0.5, 0.5)
		dc.SetColor(colorGreen)
		dc.DrawStringAnchored(fmt.Sprintf("%d", row.Wins), w*colWinX, cy, 0.5, 0.5)
		dc.SetColor(colorRed)
		dc.DrawStringAnchored(fmt.Sprintf("%d", row.Losses), w*colLossX, cy, 0.5, 0.5)
		dc.SetColor(colorGold)
		dc.DrawStringAnchored(fmt.Sprintf("%.0f%%", pct), w*colPctX, cy, 0.5, 0.5)

		dc.SetColor(colorGreen)
		dc.DrawStringAnchored(fmt.Sprintf("%d", row.LaneWins), w*colLaneWX, cy, 0.5, 0.5)
		dc.SetColor(colorRed)
		dc.DrawStringAnchored(fmt.Sprintf("%d", row.LaneLosses), w*colLaneLX, cy, 0.5, 0.5)
		dc.SetColor(colorGray)
		dc.DrawStringAnchored(fmt.Sprintf("%d", row.LaneTies), w*colLaneEX, cy, 0.5, 0.5)
		dc.SetColor(colorGold)
		dc.DrawStringAnchored(fmt.Sprintf("%.0f%%", lanePct), w*colLanePctX, cy, 0.5, 0.5)

		ry += rH
	}
}
