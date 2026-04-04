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

// Minecraft resource_packs paths (UWP variants)
var mcResourcePackPaths = []string{
	filepath.Join("Packages", "Microsoft.MinecraftUWP_8wekyb3d8bbwe",
		"LocalState", "games", "com.mojang", "resource_packs"),
	filepath.Join("Packages", "Microsoft.Minecraft115_ekx664bjj63nr",
		"LocalState", "games", "com.mojang", "resource_packs"),
}

// Pack represents a single resource pack entry from packs.json.
type Pack struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	ImageURL    string `json:"image_url"`
	DownloadURL string `json:"download_url"`
}

// buildResourcePacksContent fetches packs.json and builds the scrollable grid UI.
func (app *App) buildResourcePacksContent() fyne.CanvasObject {
	loading := widget.NewLabel("Загрузка ресурспаков...")
	loading.Alignment = fyne.TextAlignCenter

	grid := container.NewGridWithColumns(2)
	scroll := container.NewVScroll(grid)

	go func() {
		packs, err := fetchPacks()
		if err != nil {
			fyne.Do(func() {
				loading.SetText("Ошибка загрузки: " + err.Error())
			})
			return
		}
		fyne.Do(func() {
			loading.Hide()
			for _, p := range packs {
				card := app.buildPackCard(p)
				grid.Add(card)
			}
			grid.Refresh()
			scroll.Refresh()
		})
	}()

	title := ctxt(app.t("Ресурспаки", "Resource Packs"), textPrimary, 18)
	title.TextStyle = fyne.TextStyle{Bold: true}

	titleAccent := canvas.NewRectangle(accentRed)
	titleAccent.SetMinSize(fyne.NewSize(36, 2))
	titleAccent.CornerRadius = 1

	header := container.NewVBox(
		container.New(layout.NewCustomPaddedLayout(16, 4, 14, 14), title),
		container.New(layout.NewCustomPaddedLayout(0, 10, 14, 14), titleAccent),
		container.New(layout.NewCustomPaddedLayout(4, 8, 14, 14), loading),
	)

	return container.NewBorder(header, nil, nil, nil, scroll)
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

// buildPackCard creates a compact UI card for one resource pack.
func (app *App) buildPackCard(p Pack) fyne.CanvasObject {
	// Fixed-size preview image
	img := canvas.NewImageFromImage(image.NewRGBA(image.Rect(0, 0, 1, 1)))
	img.FillMode = canvas.ImageFillContain
	img.SetMinSize(fyne.NewSize(0, 90))

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
		fyne.Do(func() {
			img.Image = src
			img.Refresh()
		})
	}()

	name := ctxt(p.Name, textPrimary, 12)
	name.TextStyle = fyne.TextStyle{Bold: true}

	statusLabel := ctxt("", textSecondary, 10)

	installBtn := widget.NewButtonWithIcon(app.t("Установить", "Install"), theme.DownloadIcon(), nil)
	installBtn.Importance = widget.LowImportance
	installBtn.OnTapped = func() {
		installBtn.Disable()
		fyne.Do(func() { statusLabel.Text = app.t("Загрузка...", "Downloading..."); statusLabel.Refresh() })
		go func() {
			err := installMcpack(p.Name, p.DownloadURL)
			fyne.Do(func() {
				if err != nil {
					statusLabel.Text = "✗ " + err.Error()
					statusLabel.Color = accentOrange
				} else {
					statusLabel.Text = "✓ " + app.t("Установлен", "Installed")
					statusLabel.Color = accentGreen
				}
				statusLabel.Refresh()
				installBtn.Enable()
			})
		}()
	}

	var rows []fyne.CanvasObject
	rows = append(rows, img)
	rows = append(rows, container.New(layout.NewCustomPaddedLayout(5, 2, 0, 0), name))
	if p.Description != "" {
		desc := ctxt(p.Description, textSecondary, 10)
		rows = append(rows, container.New(layout.NewCustomPaddedLayout(0, 4, 0, 0), desc))
	}
	rows = append(rows, container.New(layout.NewCustomPaddedLayout(4, 2, 0, 0), installBtn))
	rows = append(rows, statusLabel)

	cardBg := canvas.NewRectangle(bgCard)
	cardBg.CornerRadius = 8
	cardBorder := canvas.NewRectangle(borderRed)
	cardBorder.CornerRadius = 9

	inner := container.NewVBox(rows...)
	paddedInner := container.New(layout.NewCustomPaddedLayout(8, 8, 8, 8), inner)
	card := container.NewStack(cardBorder, cardBg, paddedInner)
	return container.New(layout.NewCustomPaddedLayout(4, 4, 6, 6), card)
}

// installMcpack downloads a .mcpack file and installs it into all found Minecraft resource_packs dirs.
func installMcpack(name, downloadURL string) error {
	localAppData := os.Getenv("LOCALAPPDATA")
	if localAppData == "" {
		return fmt.Errorf("LOCALAPPDATA не задан")
	}

	// Download once
	resp, err := http.Get(downloadURL)
	if err != nil {
		return fmt.Errorf("ошибка загрузки: %w", err)
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("ошибка чтения: %w", err)
	}

	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return fmt.Errorf("ошибка открытия архива: %w", err)
	}

	installed := 0
	var lastErr error

	for _, basePath := range mcResourcePackPaths {
		packDir := filepath.Join(localAppData, basePath, sanitizeName(name))

		// Only install if the parent resource_packs dir exists
		parentDir := filepath.Dir(packDir)
		if _, err := os.Stat(parentDir); os.IsNotExist(err) {
			continue
		}

		if err := os.MkdirAll(packDir, 0755); err != nil {
			lastErr = err
			continue
		}

		if err := extractZip(zr, packDir); err != nil {
			lastErr = err
			continue
		}
		installed++
	}

	if installed == 0 {
		if lastErr != nil {
			return lastErr
		}
		return fmt.Errorf("папка resource_packs не найдена")
	}
	return nil
}

// extractZip extracts a zip.Reader into destDir safely.
func extractZip(zr *zip.Reader, destDir string) error {
	for _, f := range zr.File {
		destPath := filepath.Join(destDir, f.Name)
		if !strings.HasPrefix(filepath.Clean(destPath), filepath.Clean(destDir)) {
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
