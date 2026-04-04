package app

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

const packsJSONURL = "https://raw.githubusercontent.com/qcountel/packs/main/packs.json"

// Pack represents a single resource pack entry from packs.json.
type Pack struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	ImageURL    string `json:"image_url"`
	DownloadURL string `json:"download_url"`
}

// createPacksContent fetches packs.json and builds the scrollable grid UI.
func (app *App) createPacksContent() fyne.CanvasObject {
	loading := widget.NewLabel("Загрузка ресурспаков...")
	loading.Alignment = fyne.TextAlignCenter

	grid := container.NewGridWithColumns(2)
	scroll := container.NewVScroll(grid)

	go func() {
		packs, err := fetchPacks()
		if err != nil {
			loading.SetText("Ошибка загрузки: " + err.Error())
			return
		}
		loading.Hide()
		for _, p := range packs {
			card := app.buildPackCard(p)
			grid.Add(card)
		}
		grid.Refresh()
		scroll.Refresh()
	}()

	title := widget.NewLabelWithStyle("Ресурспаки", fyne.TextAlignCenter, fyne.TextStyle{Bold: true})

	return container.NewBorder(
		container.NewVBox(title, widget.NewSeparator(), loading),
		nil, nil, nil,
		scroll,
	)
}

// fetchPacks downloads and parses packs.json from GitHub.
func fetchPacks() ([]Pack, error) {
	resp, err := http.Get(packsJSONURL)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var packs []Pack
	if err := json.NewDecoder(resp.Body).Decode(&packs); err != nil {
		return nil, err
	}
	return packs, nil
}

// buildPackCard creates a UI card for one resource pack.
func (app *App) buildPackCard(p Pack) fyne.CanvasObject {
	// Placeholder image
	img := canvas.NewImageFromImage(image.NewRGBA(image.Rect(0, 0, 1, 1)))
	img.FillMode = canvas.ImageFillContain
	img.SetMinSize(fyne.NewSize(160, 100))

	// Async image load
	go func() {
		resp, err := http.Get(p.ImageURL)
		if err != nil {
			return
		}
		defer resp.Body.Close()
		data, err := io.ReadAll(resp.Body)
		if err != nil {
			return
		}
		src, _, err := image.Decode(bytes.NewReader(data))
		if err != nil {
			return
		}
		img.Image = src
		img.Refresh()
	}()

	name := widget.NewLabelWithStyle(p.Name, fyne.TextAlignCenter, fyne.TextStyle{Bold: true})
	name.Wrapping = fyne.TextWrapWord

	var desc *widget.Label
	if p.Description != "" {
		desc = widget.NewLabel(p.Description)
		desc.Wrapping = fyne.TextWrapWord
		desc.Alignment = fyne.TextAlignCenter
	}

	statusLabel := widget.NewLabel("")
	statusLabel.Alignment = fyne.TextAlignCenter

	installBtn := widget.NewButtonWithIcon("Установить", theme.DownloadIcon(), nil)
	installBtn.OnTapped = func() {
		installBtn.Disable()
		statusLabel.SetText("Загрузка...")
		go func() {
			err := installMcpack(p.Name, p.DownloadURL)
			if err != nil {
				statusLabel.SetText("✗ " + err.Error())
			} else {
				statusLabel.SetText("✓ Установлен")
			}
			installBtn.Enable()
		}()
	}

	var cardItems []fyne.CanvasObject
	cardItems = append(cardItems, img)
	cardItems = append(cardItems, name)
	if desc != nil {
		cardItems = append(cardItems, desc)
	}
	cardItems = append(cardItems, layout.NewSpacer())
	cardItems = append(cardItems, installBtn)
	cardItems = append(cardItems, statusLabel)

	content := container.NewVBox(cardItems...)
	return widget.NewCard("", "", content)
}

// installMcpack downloads a .mcpack file and installs it into Minecraft's resource_packs directory.
// .mcpack files are ZIP archives — we extract their contents into a named subfolder.
func installMcpack(name, downloadURL string) error {
	localAppData := os.Getenv("LOCALAPPDATA")
	if localAppData == "" {
		return fmt.Errorf("LOCALAPPDATA не задан")
	}
	packDir := filepath.Join(
		localAppData,
		"Packages",
		"Microsoft.MinecraftUWP_8wekyb3d8bbwe",
		"LocalState",
		"games",
		"com.mojang",
		"resource_packs",
		sanitizeName(name),
	)
	if err := os.MkdirAll(packDir, 0755); err != nil {
		return fmt.Errorf("не удалось создать папку: %w", err)
	}

	resp, err := http.Get(downloadURL)
	if err != nil {
		return fmt.Errorf("ошибка загрузки: %w", err)
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("ошибка чтения: %w", err)
	}

	// .mcpack is a ZIP archive
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return fmt.Errorf("ошибка открытия архива: %w", err)
	}

	for _, f := range zr.File {
		destPath := filepath.Join(packDir, f.Name)
		if !strings.HasPrefix(filepath.Clean(destPath), filepath.Clean(packDir)) {
			continue // zip-slip protection
		}
		if f.FileInfo().IsDir() {
			os.MkdirAll(destPath, 0755)
			continue
		}
		if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
			return err
		}
		rc, err := f.Open()
		if err != nil {
			return err
		}
		out, err := os.Create(destPath)
		if err != nil {
			rc.Close()
			return err
		}
		_, err = io.Copy(out, rc)
		out.Close()
		rc.Close()
		if err != nil {
			return err
		}
	}

	return nil
}

// sanitizeName removes characters unsafe for directory names.
func sanitizeName(name string) string {
	replacer := strings.NewReplacer(
		"/", "_", "\\", "_", ":", "_", "*", "_",
		"?", "_", `"`, "_", "<", "_", ">", "_", "|", "_",
	)
	return replacer.Replace(name)
}
