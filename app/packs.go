package app

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

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

// HidePacks animates back to main screen from the packs view.
func (app *App) HidePacks() {
	if app.win == nil {
		return
	}

	current := app.win.Content()
	overlay := canvas.NewRectangle(color.NRGBA{R: 0x08, G: 0x05, B: 0x05, A: 0x00})
	app.win.SetContent(container.NewStack(current, overlay))

	go func() {
		const dur1 = 180 * time.Millisecond
		start := time.Now()
		ticker := time.NewTicker(14 * time.Millisecond)
		for range ticker.C {
			p := math.Min(1.0, float64(time.Since(start))/float64(dur1))
			a := uint8(255 * (p * p))
			fyne.Do(func() {
				overlay.FillColor = color.NRGBA{R: 0x08, G: 0x05, B: 0x05, A: a}
				overlay.Refresh()
			})
			if p >= 1 {
				break
			}
		}
		ticker.Stop()

		app.showPacks = false
		fyne.Do(func() {
			nc, _, _ := app.createContent(app.tr.Process())
			newOverlay := canvas.NewRectangle(color.NRGBA{R: 0x08, G: 0x05, B: 0x05, A: 0xFF})
			app.win.SetContent(container.NewStack(nc, newOverlay))
			app.win.Resize(fyne.NewSize(520, 720))

			go func() {
				const dur2 = 220 * time.Millisecond
				start2 := time.Now()
				t2 := time.NewTicker(14 * time.Millisecond)
				defer t2.Stop()
				for range t2.C {
					p := math.Min(1.0, float64(time.Since(start2))/float64(dur2))
					ease := 1 - math.Pow(1-p, 3)
					a := uint8(255 * (1 - ease))
					fyne.Do(func() {
						newOverlay.FillColor = color.NRGBA{R: 0x08, G: 0x05, B: 0x05, A: a}
						newOverlay.Refresh()
						if p >= 1 {
							newOverlay.Hide()
						}
					})
					if p >= 1 {
						break
					}
				}
			}()
		})
	}()
}

// buildResourcePacksContent fetches packs.json and builds the scrollable grid UI.
func (app *App) buildResourcePacksContent() fyne.CanvasObject {
	backBtn := widget.NewButton(app.t("← Назад", "← Back"), func() {
		app.HidePacks()
	})
	backBtn.Importance = widget.LowImportance

	titleText := ctxt(app.t("Ресурспаки", "Resource Packs"), textPrimary, 26)
	titleText.TextStyle = fyne.TextStyle{Bold: true}

	titleAccent := canvas.NewRectangle(accentRed)
	titleAccent.SetMinSize(fyne.NewSize(40, 2))
	titleAccent.CornerRadius = 1

	loading := widget.NewLabel(app.t("Загрузка...", "Loading..."))
	loading.Alignment = fyne.TextAlignCenter

	headerInner := container.NewVBox(
		container.NewHBox(backBtn),
		container.New(layout.NewCustomPaddedLayout(4, 2, 0, 0), titleText),
		container.New(layout.NewCustomPaddedLayout(0, 6, 0, 0), titleAccent),
		container.New(layout.NewCustomPaddedLayout(4, 8, 14, 14), loading),
	)

	headerBg := canvas.NewRectangle(bgCard)
	sepLine := canvas.NewRectangle(borderRed)
	sepLine.SetMinSize(fyne.NewSize(0, 1))

	headerPadded := container.New(layout.NewCustomPaddedLayout(10, 10, 14, 14), headerInner)
	headerContent := container.NewBorder(nil, sepLine, nil, nil, headerPadded)
	header := container.NewStack(headerBg, headerContent)

	grid := container.NewGridWithColumns(2)
	scroll := container.NewVScroll(grid)

	go func() {
		packs, err := fetchPacks()
		if err != nil {
			fyne.Do(func() {
				loading.SetText(app.t("Ошибка: ", "Error: ") + err.Error())
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

	bg := canvas.NewRectangle(bgPrimary)
	body := container.NewBorder(header, nil, nil, nil, scroll)
	return container.NewStack(bg, body)
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
	// Fixed-size preview image placeholder
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
		fyne.Do(func() {
			statusLabel.Text = app.t("Загрузка...", "Downloading...")
			statusLabel.Refresh()
		})
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
