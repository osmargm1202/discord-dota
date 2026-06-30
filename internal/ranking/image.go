package ranking

import (
	"bytes"
	"dota-discord-bot/fonts"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"image/color"
	"time"

	"github.com/fogleman/gg"
	"golang.org/x/image/font/opentype"
)

// decodeImage decodes PNG or JPEG bytes into an image.Image.
func decodeImage(data []byte) (image.Image, string, error) {
	return image.Decode(bytes.NewReader(data))
}

const (
	canvasW  = 900.0
	rowH     = 50.0
	headerH  = 68.0
	sectionH = 32.0
	footerH  = 28.0
	paddingL = 18.0
)

var (
	colorBG     = color.RGBA{13, 17, 23, 255}
	colorPanel  = color.RGBA{26, 35, 50, 255}
	colorGold   = color.RGBA{200, 170, 110, 255}
	colorGray   = color.RGBA{136, 153, 170, 255}
	colorGreen  = color.RGBA{39, 174, 96, 255}
	colorRed    = color.RGBA{192, 57, 43, 255}
	colorBlue   = color.RGBA{52, 152, 219, 255}
	colorYellow = color.RGBA{243, 156, 18, 255}
	colorWhite  = color.RGBA{232, 213, 183, 255}
	colorRowAlt = color.RGBA{17, 24, 39, 180}
)

// ImageGenerator renders ranking PNG images.
type ImageGenerator struct{}

func NewImageGenerator() *ImageGenerator { return &ImageGenerator{} }

func (g *ImageGenerator) loadFont(dc *gg.Context, size float64) {
	ft, err := opentype.Parse(fonts.Exo2Regular)
	if err != nil {
		return
	}
	face, err := opentype.NewFace(ft, &opentype.FaceOptions{Size: size, DPI: 96})
	if err != nil {
		return
	}
	dc.SetFontFace(face)
}

func (g *ImageGenerator) drawBackground(dc *gg.Context) {
	h := float64(dc.Height())
	w := float64(dc.Width())
	for y := 0.0; y < h; y++ {
		t := y / h
		var blend float64
		if t < 0.5 {
			blend = t * 2
		} else {
			blend = (1 - t) * 2
		}
		r := float64(colorBG.R) + (float64(colorPanel.R)-float64(colorBG.R))*blend
		gr := float64(colorBG.G) + (float64(colorPanel.G)-float64(colorBG.G))*blend
		b := float64(colorBG.B) + (float64(colorPanel.B)-float64(colorBG.B))*blend
		dc.SetRGBA255(int(r), int(gr), int(b), 255)
		dc.DrawLine(0, y, w, y)
		dc.Stroke()
	}
}

func (g *ImageGenerator) drawBorder(dc *gg.Context) {
	w := float64(dc.Width())
	h := float64(dc.Height())
	dc.SetColor(colorGold)
	dc.SetLineWidth(1.5)
	dc.DrawRectangle(0.5, 0.5, w-1, h-1)
	dc.Stroke()
	for i := 0; i < 3; i++ {
		alpha := 255 - i*80
		dc.SetRGBA255(200, 170, 110, alpha)
		dc.DrawLine(0, float64(i), w, float64(i))
		dc.Stroke()
	}
}

func (g *ImageGenerator) drawHeader(dc *gg.Context, title, subtitle string) {
	w := float64(dc.Width())
	dc.SetColor(colorPanel)
	dc.DrawRectangle(0, 0, w, headerH)
	dc.Fill()
	dc.SetColor(colorGold)
	dc.SetLineWidth(0.8)
	dc.DrawLine(0, headerH, w, headerH)
	dc.Stroke()
	g.loadFont(dc, 15)
	dc.SetColor(colorGold)
	dc.DrawStringAnchored(title, paddingL, headerH*0.38, 0, 0.5)
	g.loadFont(dc, 11)
	dc.SetColor(colorGray)
	dc.DrawStringAnchored(subtitle, paddingL, headerH*0.72, 0, 0.5)
}

func (g *ImageGenerator) drawSectionHeader(dc *gg.Context, y float64, text string) {
	w := float64(dc.Width())
	dc.SetColor(colorBG)
	dc.DrawRectangle(0, y, w, sectionH)
	dc.Fill()
	dc.SetColor(colorGold)
	dc.SetLineWidth(0.5)
	dc.DrawLine(0, y+sectionH, w, y+sectionH)
	dc.Stroke()
	g.loadFont(dc, 10)
	dc.SetColor(colorGold)
	dc.DrawStringAnchored(text, paddingL, y+sectionH/2, 0, 0.5)
}

type colDef struct {
	label   string
	x       float64
	anchorX float64
}

var indivCols = []colDef{
	{"#", 22, 0.5},
	{"JUGADOR", 70, 0},
	{"T", 390, 0.5},
	{"G", 435, 0.5},
	{"P", 478, 0.5},
	{"WIN%", 533, 0.5},
	{"+/-", 595, 0.5},
	{"~MMR", 678, 0.5},
}

var teamCols = []colDef{
	{"EQUIPO", 70, 0},
	{"G", 510, 0.5},
	{"P", 555, 0.5},
	{"WIN%", 605, 0.5},
	{"+/-", 660, 0.5},
	{"~MMR", 735, 0.5},
}

func (g *ImageGenerator) drawColHeaders(dc *gg.Context, y float64, cols []colDef) {
	dc.SetColor(colorPanel)
	dc.DrawRectangle(0, y, float64(dc.Width()), 24)
	dc.Fill()
	g.loadFont(dc, 10)
	dc.SetColor(colorGray)
	for _, col := range cols {
		dc.DrawStringAnchored(col.label, col.x, y+12, col.anchorX, 0.5)
	}
}

func (g *ImageGenerator) drawIndivRow(dc *gg.Context, y float64, idx int, r PlayerRankRow) {
	w := float64(dc.Width())
	if idx%2 == 1 {
		dc.SetRGBA255(17, 24, 39, 120)
		dc.DrawRectangle(0, y, w, rowH)
		dc.Fill()
	}
	// rank
	g.loadFont(dc, 13)
	medals := []string{"1", "2", "3"}
	rankStr := fmt.Sprintf("%d", idx+1)
	if idx < 3 {
		rankStr = medals[idx]
	}
	dc.SetColor(colorGray)
	dc.DrawStringAnchored(rankStr, 22, y+rowH/2, 0.5, 0.5)

	// avatar circle: show image if available, initials otherwise
	const avatarCX, avatarCY2, avatarR2 = 56.0, 0.0, 18.0 // avatarCY2 = y+rowH/2 computed below
	circleCY := y + rowH/2
	if len(r.AvatarBytes) > 0 {
		if img, _, err := decodeImage(r.AvatarBytes); err == nil {
			dc.DrawCircle(avatarCX, circleCY, avatarR2)
			dc.Clip()
			scaled := scaleImage(img, int(avatarR2*2), int(avatarR2*2))
			dc.DrawImageAnchored(scaled, int(avatarCX), int(circleCY), 0.5, 0.5)
			dc.ResetClip()
		}
	} else {
		dc.SetColor(colorPanel)
		dc.DrawCircle(avatarCX, circleCY, avatarR2)
		dc.Fill()
		initial := "?"
		if len(r.DisplayName) > 0 {
			runes := []rune(r.DisplayName)
			initial = string(runes[0])
		}
		g.loadFont(dc, 14)
		dc.SetColor(colorGold)
		dc.DrawStringAnchored(initial, avatarCX, circleCY, 0.5, 0.5)
	}
	dc.SetColor(colorGold)
	dc.SetLineWidth(1)
	dc.DrawCircle(avatarCX, circleCY, avatarR2)
	dc.Stroke()

	// player name
	g.loadFont(dc, 13)
	dc.SetColor(colorWhite)
	dc.DrawStringAnchored(r.DisplayName, 82, y+rowH/2, 0, 0.5)

	// stats
	total := r.Wins + r.Losses
	dc.SetColor(colorGray)
	dc.DrawStringAnchored(fmt.Sprintf("%d", total), 390, y+rowH/2, 0.5, 0.5)
	dc.SetColor(colorGreen)
	dc.DrawStringAnchored(fmt.Sprintf("%d", r.Wins), 435, y+rowH/2, 0.5, 0.5)
	dc.SetColor(colorRed)
	dc.DrawStringAnchored(fmt.Sprintf("%d", r.Losses), 478, y+rowH/2, 0.5, 0.5)
	dc.SetColor(winPctColor(r.WinPct))
	dc.DrawStringAnchored(fmt.Sprintf("%.0f%%", r.WinPct), 533, y+rowH/2, 0.5, 0.5)
	dc.SetColor(signColor(r.Net))
	dc.DrawStringAnchored(signStr(r.Net), 595, y+rowH/2, 0.5, 0.5)
	dc.SetColor(signColor(r.MMRTotal))
	dc.DrawStringAnchored("~"+signStr(r.MMRTotal), 678, y+rowH/2, 0.5, 0.5)
}

func (g *ImageGenerator) drawTeam2Row(dc *gg.Context, y float64, idx int, r Team2Row) {
	w := float64(dc.Width())
	if idx%2 == 1 {
		dc.SetColor(colorRowAlt)
		dc.DrawRectangle(0, y, w, rowH)
		dc.Fill()
	}
	g.loadFont(dc, 12)
	dc.SetColor(colorWhite)
	dc.DrawStringAnchored(r.Name1+" + "+r.Name2, 70, y+rowH/2, 0, 0.5)
	g.loadFont(dc, 13)
	dc.SetColor(colorGreen)
	dc.DrawStringAnchored(fmt.Sprintf("%d", r.Wins), 510, y+rowH/2, 0.5, 0.5)
	dc.SetColor(colorRed)
	dc.DrawStringAnchored(fmt.Sprintf("%d", r.Losses), 555, y+rowH/2, 0.5, 0.5)
	dc.SetColor(winPctColor(r.WinPct))
	dc.DrawStringAnchored(fmt.Sprintf("%.0f%%", r.WinPct), 605, y+rowH/2, 0.5, 0.5)
	dc.SetColor(signColor(r.Net))
	dc.DrawStringAnchored(signStr(r.Net), 660, y+rowH/2, 0.5, 0.5)
	dc.SetColor(signColor(r.MMRTogether))
	dc.DrawStringAnchored("~"+signStr(r.MMRTogether), 735, y+rowH/2, 0.5, 0.5)
}

func (g *ImageGenerator) drawTeam3Row(dc *gg.Context, y float64, idx int, r Team3Row) {
	w := float64(dc.Width())
	if idx%2 == 1 {
		dc.SetColor(colorRowAlt)
		dc.DrawRectangle(0, y, w, rowH)
		dc.Fill()
	}
	g.loadFont(dc, 11)
	dc.SetColor(colorWhite)
	dc.DrawStringAnchored(r.Name1+" + "+r.Name2+" + "+r.Name3, 70, y+rowH/2, 0, 0.5)
	g.loadFont(dc, 13)
	dc.SetColor(colorGreen)
	dc.DrawStringAnchored(fmt.Sprintf("%d", r.Wins), 510, y+rowH/2, 0.5, 0.5)
	dc.SetColor(colorRed)
	dc.DrawStringAnchored(fmt.Sprintf("%d", r.Losses), 555, y+rowH/2, 0.5, 0.5)
	dc.SetColor(winPctColor(r.WinPct))
	dc.DrawStringAnchored(fmt.Sprintf("%.0f%%", r.WinPct), 605, y+rowH/2, 0.5, 0.5)
	dc.SetColor(signColor(r.Net))
	dc.DrawStringAnchored(signStr(r.Net), 660, y+rowH/2, 0.5, 0.5)
	dc.SetColor(signColor(r.MMRTogether))
	dc.DrawStringAnchored("~"+signStr(r.MMRTogether), 735, y+rowH/2, 0.5, 0.5)
}

func (g *ImageGenerator) drawFooter(dc *gg.Context, y float64) {
	w := float64(dc.Width())
	dc.SetColor(colorBG)
	dc.DrawRectangle(0, y, w, footerH)
	dc.Fill()
	dc.SetColor(colorGold)
	dc.SetLineWidth(0.5)
	dc.DrawLine(0, y, w, y)
	dc.Stroke()
	g.loadFont(dc, 10)
	dc.SetColor(colorGray)
	ts := time.Now().UTC().Format("Mon Jan 02, 2006 03:04 PM")
	dc.DrawStringAnchored("Actualizado: "+ts+" UTC  ·  ~MMR estimado ±25/partida", paddingL, y+footerH/2, 0, 0.5)
	dc.DrawStringAnchored("Stratz API", w-paddingL, y+footerH/2, 1, 0.5)
}

func emptyMsg(dc *gg.Context, y float64, msg string) {
	g := &ImageGenerator{}
	g.loadFont(dc, 12)
	dc.SetColor(colorGray)
	dc.DrawStringAnchored(msg, canvasW/2, y+30, 0.5, 0.5)
}

// RenderIndividual generates the individual ranking PNG.
func (g *ImageGenerator) RenderIndividual(players []PlayerRankRow, weekLabel string) ([]byte, error) {
	colH := 24.0
	minRows := 1
	nRows := len(players)
	if nRows < minRows {
		nRows = minRows
	}
	h := headerH + sectionH + colH + float64(nRows)*rowH + footerH + 4
	if len(players) == 0 {
		h = headerH + sectionH + 60 + footerH
	}
	dc := gg.NewContext(int(canvasW), int(h))
	g.drawBackground(dc)
	g.drawHeader(dc, "RANKING INDIVIDUAL · Dota 2", weekLabel)
	y := headerH
	g.drawSectionHeader(dc, y, "RANKING INDIVIDUAL")
	y += sectionH
	if len(players) == 0 {
		emptyMsg(dc, y, "Sin partidas en este periodo")
		y += 60
	} else {
		g.drawColHeaders(dc, y, indivCols)
		y += colH
		for i, r := range players {
			g.drawIndivRow(dc, y, i, r)
			y += rowH
		}
	}
	g.drawFooter(dc, y)
	g.drawBorder(dc)
	var buf bytes.Buffer
	if err := dc.EncodePNG(&buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// RenderTeam2 generates the 2-player combo ranking PNG.
func (g *ImageGenerator) RenderTeam2(combos []Team2Row, weekLabel string) ([]byte, error) {
	colH := 24.0
	h := headerH + sectionH + colH + float64(len(combos))*rowH + footerH + 4
	if len(combos) == 0 {
		h = headerH + sectionH + 60 + footerH
	}
	dc := gg.NewContext(int(canvasW), int(h))
	g.drawBackground(dc)
	g.drawHeader(dc, "RANKING EN EQUIPO · Dota 2", weekLabel)
	y := headerH
	g.drawSectionHeader(dc, y, "COMBOS — 2 JUGADORES")
	y += sectionH
	if len(combos) == 0 {
		emptyMsg(dc, y, "Sin partidas en equipo este periodo")
		y += 60
	} else {
		g.drawColHeaders(dc, y, teamCols)
		y += colH
		for i, r := range combos {
			g.drawTeam2Row(dc, y, i, r)
			y += rowH
		}
	}
	g.drawFooter(dc, y)
	g.drawBorder(dc)
	var buf bytes.Buffer
	if err := dc.EncodePNG(&buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// RenderTeam3 generates the 3-player combo ranking PNG.
func (g *ImageGenerator) RenderTeam3(combos []Team3Row, weekLabel string) ([]byte, error) {
	colH := 24.0
	h := headerH + sectionH + colH + float64(len(combos))*rowH + footerH + 4
	if len(combos) == 0 {
		h = headerH + sectionH + 60 + footerH
	}
	dc := gg.NewContext(int(canvasW), int(h))
	g.drawBackground(dc)
	g.drawHeader(dc, "RANKING EN EQUIPO · Dota 2", weekLabel)
	y := headerH
	g.drawSectionHeader(dc, y, "COMBOS — 3 JUGADORES")
	y += sectionH
	if len(combos) == 0 {
		emptyMsg(dc, y, "Sin partidas en equipo este periodo")
		y += 60
	} else {
		g.drawColHeaders(dc, y, teamCols)
		y += colH
		for i, r := range combos {
			g.drawTeam3Row(dc, y, i, r)
			y += rowH
		}
	}
	g.drawFooter(dc, y)
	g.drawBorder(dc)
	var buf bytes.Buffer
	if err := dc.EncodePNG(&buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func winPctColor(pct float64) color.Color {
	if pct >= 50 {
		return colorGreen
	}
	if pct >= 40 {
		return colorYellow
	}
	return colorRed
}

func signColor(n int) color.Color {
	if n > 0 {
		return colorBlue
	}
	if n < 0 {
		return colorRed
	}
	return colorGray
}

func signStr(n int) string {
	if n > 0 {
		return fmt.Sprintf("+%d", n)
	}
	return fmt.Sprintf("%d", n)
}
