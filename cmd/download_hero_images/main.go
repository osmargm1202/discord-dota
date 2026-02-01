// download_hero_images descarga las imágenes de héroes de Dota 2 (Steam CDN)
// a dota/miniaturas/{slug}.png para uso local.
// Ejecutar desde la raíz del repo: go run ./cmd/download_hero_images
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	steamCDNRenders = "https://cdn.akamai.steamstatic.com/apps/dota2/videos/dota_react/heroes/renders"
	steamCDNIcons   = "https://cdn.akamai.steamstatic.com/apps/dota2/images/dota_react/heroes/icons"
)

type heroEntry struct {
	ID            int    `json:"id"`
	Name          string `json:"name"`
	LocalizedName string `json:"localized_name"`
}

func main() {
	heroesPath := "dota/heroes.json"
	if len(os.Args) > 1 {
		heroesPath = os.Args[1]
	}
	outDir := "dota/miniaturas"
	if len(os.Args) > 2 {
		outDir = os.Args[2]
	}

	data, err := os.ReadFile(heroesPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error leyendo %s: %v\n", heroesPath, err)
		os.Exit(1)
	}

	var raw map[string]heroEntry
	if err := json.Unmarshal(data, &raw); err != nil {
		fmt.Fprintf(os.Stderr, "Error parseando heroes.json: %v\n", err)
		os.Exit(1)
	}

	if err := os.MkdirAll(outDir, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "Error creando %s: %v\n", outDir, err)
		os.Exit(1)
	}

	client := &http.Client{Timeout: 30 * time.Second}
	var slugs []string
	for _, h := range raw {
		if h.Name == "" {
			continue
		}
		slug := strings.TrimPrefix(h.Name, "npc_dota_hero_")
		if slug == "" {
			continue
		}
		slugs = append(slugs, slug)
	}

	fmt.Printf("Descargando %d héroes en %s ...\n", len(slugs), outDir)
	ok, fail := 0, 0
	for _, slug := range slugs {
		// Render (imagen grande) -> {slug}.png
		renderURL := fmt.Sprintf("%s/%s.png", steamCDNRenders, slug)
		renderPath := filepath.Join(outDir, slug+".png")
		if err := download(client, renderURL, renderPath); err != nil {
			fmt.Fprintf(os.Stderr, "  %s: %v\n", slug, err)
			fail++
		} else {
			ok++
		}
		time.Sleep(100 * time.Millisecond)
		// Icono (miniatura) -> {slug}_icon.png
		iconURL := fmt.Sprintf("%s/%s.png", steamCDNIcons, slug)
		iconPath := filepath.Join(outDir, slug+"_icon.png")
		_ = download(client, iconURL, iconPath)
		time.Sleep(100 * time.Millisecond)
	}
	fmt.Printf("Listo: %d renders ok, %d fallos\n", ok, fail)
}

func download(client *http.Client, url, path string) error {
	resp, err := client.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = io.Copy(f, resp.Body)
	return err
}
