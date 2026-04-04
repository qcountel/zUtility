package app

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"image/color"
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
	"fyne.io/fyne/v2/widget"
)

const packsJSONURL = "https://raw.githubusercontent.com/qcountel/zUtility/main/packs.json"

// Pack describes a single resource pack entry from packs.json.
type Pack struct {
	Name        string `json:"name"`
	ImageURL    string `json:"image_url"`
	DownloadURL string `json:"download_url"`
}

// ---------------------------------------------------------------------------
// Navigation helpers (mirror of animateToSettings / HideSettings)
// ---------------------------------------------------------------------------

func (app *App) animateToPacks() {
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

		app.showPacks = true
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

// ---------------------------------------------------------------------------
// Packs screen
// ---------------------------------------------------------------------------

func (app *App) createPacksContent() fyne.CanvasObject {
	bg := canvas.NewRectangle(bgPrimary)

	// Header
	backBtn := widget.NewButton(app.t("← Назад", "← Back"), func() {
		app.HidePacks()
	})
	backBtn.Importance = widget.LowImportance

	titleText := ctxt(app.t("Ресурспаки", "Resource Packs"), textPrimary, 26)
	titleText.TextStyle = fyne.TextStyle{Bold: true}

	titleAccent := canvas.NewRectangle(accentRed)
	titleAccent.SetMinSize(fyne.NewSize(40, 2))
	titleAccent.CornerRadius = 1

	headerInner := container.NewVBox(
		container.NewHBox(backBtn),
		container.New(layout.NewCustomPaddedLayout(4, 2, 0, 0), titleText),
		container.New(layout.NewCustomPaddedLayout(0, 6, 0, 0), titleAccent),
	)
	headerBg := canvas.NewRectangle(bgCard)
	sepLine := canvas.NewRectangle(borderRed)
	sepLine.SetMinSize(fyne.NewSize(0, 1))
	headerPadded := container.New(layout.NewCustomPaddedLayout(10, 10, 14, 14), headerInner)
	headerContent := container.NewBorder(nil, sepLine, nil, nil, headerPadded)
	header := container.NewStack(headerBg, headerContent)

	// Loading placeholder
	loadingLabel := ctxt(app.t("Загрузка паков...", "Loading packs..."), textSecondary, 13)
	gridContainer := container.NewVBox(
		container.New(layout.NewCustomPaddedLayout(32, 32, 0, 0), container.NewCenter(loadingLabel)),
	)

	scroll := container.NewVScroll(gridContainer)
	body := container.NewBorder(header, nil, nil, nil, scroll)
	content := container.NewStack(bg, body)

	// Async: fetch list, then build cards
	go func() {
		packs, err := fetchPacks()
		fyne.Do(func() {
			gridContainer.Objects = nil

			if err != nil {
				errLabel := ctxt(
					fmt.Sprintf(app.t("Ошибка загрузки: %v", "Load error: %v"), err),
					accentRed, 13,
				)
				gridContainer.Objects = []fyne.CanvasObject{
					container.New(layout.NewCustomPaddedLayout(32, 32, 0, 0), container.NewCenter(errLabel)),
				}
				gridContainer.Refresh()
				return
			}
			if len(packs) == 0 {
				empty := ctxt(app.t("Нет доступных паков", "No packs available"), textSecondary, 13)
				gridContainer.Objects = []fyne.CanvasObject{
					container.New(layout.NewCustomPaddedLayout(32, 32, 0, 0), container.NewCenter(empty)),
				}
				gridContainer.Refresh()
				return
			}

			var cards []fyne.CanvasObject
			for _, p := range packs {
				cards = append(cards, app.buildPackCard(p))
			}
			grid := container.NewGridWithColumns(2, cards...)
			gridPadded := container.New(layout.NewCustomPaddedLayout(6, 6, 8, 8), grid)
			gridContainer.Objects = []fyne.CanvasObject{gridPadded}
			gridContainer.Refresh()
		})
	}()

	return content
}

// buildPackCard creates a card widget for a single pack.
// Image is loaded asynchronously after the card is rendered.
func (app *App) buildPackCard(p Pack) fyne.CanvasObject {
	// Placeholder image area
	placeholder := canvas.NewRectangle(bgElevated)
	placeholder.CornerRadius = 6
	placeholder.SetMinSize(fyne.NewSize(0, 110))
	imgSizer := canvas.NewRectangle(color.Transparent)
	imgSizer.SetMinSize(fyne.NewSize(0, 110))
	imgContainer := container.NewStack(placeholder, imgSizer)

	// Name
	nameLabel := ctxt(p.Name, textPrimary, 12)
	nameLabel.TextStyle = fyne.TextStyle{Bold: true}

	// Status widget.Label for proper show/hide
	statusLabel := widget.NewLabel("")
	statusLabel.TextStyle = fyne.TextStyle{}
	statusLabel.Hide()

	// Install button
	installBtn := widget.NewButton(app.t("Установить", "Install"), nil)
	installBtn.Importance = widget.HighImportance

	installBtn.OnTapped = func() {
		installBtn.Disable()
		statusLabel.Show()
		statusLabel.SetText(app.t("Установка...", "Installing..."))

		go func() {
			err := installPackZip(p.DownloadURL, p.Name)
			fyne.Do(func() {
				if err != nil {
					statusLabel.SetText(fmt.Sprintf("✗ %v", err))
				} else {
					statusLabel.SetText(app.t("✓ Установлен!", "✓ Installed!"))
				}
				installBtn.Enable()
			})
		}()
	}

	inner := container.NewVBox(
		imgContainer,
		container.New(layout.NewCustomPaddedLayout(4, 2, 2, 2), nameLabel),
		container.New(layout.NewCustomPaddedLayout(2, 2, 0, 0), installBtn),
		statusLabel,
	)

	cardBg := canvas.NewRectangle(bgCard)
	cardBg.CornerRadius = 8
	cardBorder := canvas.NewRectangle(borderRed)
	cardBorder.CornerRadius = 9

	paddedInner := container.New(layout.NewCustomPaddedLayout(10, 10, 10, 10), inner)
	card := container.NewStack(cardBorder, cardBg, paddedInner)
	result := container.New(layout.NewCustomPaddedLayout(3, 3, 4, 4), card)

	// Async image load — does not block render
	if p.ImageURL != "" {
		go func() {
			img, err := loadImageFromURL(p.ImageURL)
			if err != nil {
				return
			}
			fyneImg := canvas.NewImageFromImage(img)
			fyneImg.FillMode = canvas.ImageFillContain
			fyneImg.SetMinSize(fyne.NewSize(0, 110))
			fyne.Do(func() {
				imgContainer.Objects = []fyne.CanvasObject{fyneImg}
				imgContainer.Refresh()
			})
		}()
	}

	return result
}

// ---------------------------------------------------------------------------
// Network helpers
// ---------------------------------------------------------------------------

func fetchPacks() ([]Pack, error) {
	resp, err := http.Get(packsJSONURL)
	if err != nil {
		return nil, fmt.Errorf("fetch packs.json: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("packs.json: HTTP %d", resp.StatusCode)
	}
	var packs []Pack
	if err := json.NewDecoder(resp.Body).Decode(&packs); err != nil {
		return nil, fmt.Errorf("decode packs.json: %w", err)
	}
	return packs, nil
}

func loadImageFromURL(url string) (image.Image, error) {
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("image HTTP %d", resp.StatusCode)
	}
	img, _, err := image.Decode(resp.Body)
	return img, err
}

// ---------------------------------------------------------------------------
// Install helpers
// ---------------------------------------------------------------------------

// resourcePacksPath returns the Minecraft Win10 resource_packs directory.
func resourcePacksPath() string {
	localApp := os.Getenv("LOCALAPPDATA")
	return filepath.Join(
		localApp,
		"Packages",
		"Microsoft.MinecraftUWP_8wekyb3d8bbwe",
		"LocalState",
		"games",
		"com.mojang",
		"resource_packs",
	)
}

// installPackZip downloads a zip/mcpack and extracts it into resource_packs/<name>/.
func installPackZip(downloadURL, packName string) error {
	resp, err := http.Get(downloadURL)
	if err != nil {
		return fmt.Errorf("download: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download HTTP %d", resp.StatusCode)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read body: %w", err)
	}

	destDir := filepath.Join(resourcePacksPath(), sanitizeName(packName))
	if err := os.MkdirAll(destDir, 0o755); err != nil {
		return fmt.Errorf("create dir: %w", err)
	}

	// Try to unzip
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		// Not a zip — write raw file
		ext := ".mcpack"
		if strings.HasSuffix(strings.ToLower(downloadURL), ".zip") {
			ext = ".zip"
		}
		return os.WriteFile(filepath.Join(resourcePacksPath(), sanitizeName(packName)+ext), data, 0o644)
	}

	// Extract zip entries
	for _, f := range zr.File {
		// Strip the first component if the zip has a single root folder
		relPath := f.Name
		parts := strings.SplitN(relPath, "/", 2)
		if len(parts) == 2 {
			relPath = parts[1]
		}
		if relPath == "" {
			continue
		}

		target := filepath.Join(destDir, filepath.FromSlash(relPath))

		if f.FileInfo().IsDir() {
			if err := os.MkdirAll(target, 0o755); err != nil {
				return err
			}
			continue
		}

		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		rc, err := f.Open()
		if err != nil {
			return err
		}
		out, err := os.Create(target)
		if err != nil {
			rc.Close()
			return err
		}
		_, err = io.Copy(out, rc)
		rc.Close()
		out.Close()
		if err != nil {
			return err
		}
	}
	return nil
}

func sanitizeName(name string) string {
	replacer := strings.NewReplacer(
		" ", "_",
		"/", "-",
		"\\", "-",
		":", "-",
		"*", "-",
		"?", "-",
		"\"", "-",
		"<", "-",
		">", "-",
		"|", "-",
	)
	return replacer.Replace(name)
}
