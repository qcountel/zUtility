package app

import (
	"archive/zip"
	"bytes"
	"context"
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
	"time"

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

// buildResourcePacksContent fetches packs.json and builds the scrollable grid UI.
func (app *App) buildResourcePacksContent() fyne.CanvasObject {
	loading := widget.NewLabel("Загрузка ресурспаков...")
	loading.Alignment = fyne.TextAlignCenter

	scroll := container.NewVScroll(widget.NewLabel(""))

	loadPacks := func() {
		loading.Show()
		loading.SetText(app.t("Загрузка ресурспаков...", "Loading resource packs..."))
		scroll.Content = widget.NewLabel("")
		scroll.Refresh()

		go func() {
			packs, err := fetchPacks()
			fyne.Do(func() {
				loading.Hide()
				if err != nil {
					loading.SetText(app.t("Ошибка загрузки: ", "Load error: ") + err.Error())
					loading.Show()
					return
				}
				var cards []fyne.CanvasObject
				for _, p := range packs {
					cards = append(cards, app.buildPackCard(p))
				}
				if len(cards) > 0 {
					grid := container.NewGridWithColumns(2, cards...)
					scroll.Content = container.New(layout.NewCustomPaddedLayout(4, 4, 4, 4), grid)
				}
				scroll.Refresh()
			})
		}()
	}

	title := ctxt(app.t("Ресурспаки", "Resource Packs"), textPrimary, 18)
	title.TextStyle = fyne.TextStyle{Bold: true}

	titleAccent := canvas.NewRectangle(accentRed)
	titleAccent.SetMinSize(fyne.NewSize(36, 2))
	titleAccent.CornerRadius = 1

	titleRow := container.New(layout.NewCustomPaddedLayout(16, 4, 14, 14), title)

	header := container.NewVBox(
		titleRow,
		container.New(layout.NewCustomPaddedLayout(0, 10, 14, 14), titleAccent),
		container.New(layout.NewCustomPaddedLayout(4, 8, 14, 14), loading),
	)

	loadPacks()

	return container.NewBorder(header, nil, nil, nil, scroll)
}

// fetchPacks downloads and parses packs.json from GitHub.
func fetchPacks() ([]Pack, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, packsJSONURL, nil)
	if err != nil {
		return nil, err
	}
	resp, err := http.DefaultClient.Do(req)
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

// fixedSizeLayout is a Fyne layout that always reports a fixed size,
// preventing images from expanding cards when loaded asynchronously.
type fixedSizeLayout struct{ size fyne.Size }

func (f fixedSizeLayout) Layout(objs []fyne.CanvasObject, _ fyne.Size) {
	for _, o := range objs {
		o.Move(fyne.NewPos(0, 0))
		o.Resize(f.size)
	}
}
func (f fixedSizeLayout) MinSize(_ []fyne.CanvasObject) fyne.Size { return f.size }

// fixedHeightLayout locks height to a constant but lets width be flexible.
type fixedHeightLayout struct{ h float32 }

func (f fixedHeightLayout) Layout(objs []fyne.CanvasObject, size fyne.Size) {
	for _, o := range objs {
		o.Move(fyne.NewPos(0, 0))
		o.Resize(fyne.NewSize(size.Width, f.h))
	}
}
func (f fixedHeightLayout) MinSize(_ []fyne.CanvasObject) fyne.Size {
	return fyne.NewSize(0, f.h)
}

// buildPackCard creates a compact UI card for one resource pack.
func (app *App) buildPackCard(p Pack) fyne.CanvasObject {
	// Placeholder image
	img := canvas.NewImageFromImage(image.NewRGBA(image.Rect(0, 0, 1, 1)))
	img.FillMode = canvas.ImageFillContain
	img.ScaleMode = canvas.ImageScaleSmooth

	// Fixed-height wrapper — ширина тянется по карточке, высота строго 140px
	imgFixed := container.New(fixedHeightLayout{140}, img)

	// Async image load
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, p.ImageURL, nil)
		if err != nil {
			return
		}
		resp, err := http.DefaultClient.Do(req)
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

	var descObj fyne.CanvasObject
	if p.Description != "" {
		desc := ctxt(p.Description, textSecondary, 10)
		descObj = container.New(layout.NewCustomPaddedLayout(2, 0, 0, 0), desc)
	}

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

	var cardItems []fyne.CanvasObject
	cardItems = append(cardItems, imgFixed)
	cardItems = append(cardItems, container.New(layout.NewCustomPaddedLayout(4, 2, 0, 0), name))
	if descObj != nil {
		cardItems = append(cardItems, descObj)
	}
	cardItems = append(cardItems, container.New(layout.NewCustomPaddedLayout(4, 0, 0, 0), installBtn))
	cardItems = append(cardItems, statusLabel)

	cardBg := canvas.NewRectangle(bgCard)
	cardBg.CornerRadius = 8
	cardBorder := canvas.NewRectangle(borderRed)
	cardBorder.CornerRadius = 9

	inner := container.NewVBox(cardItems...)
	paddedInner := container.New(layout.NewCustomPaddedLayout(10, 10, 10, 10), inner)
	card := container.NewStack(cardBorder, cardBg, paddedInner)
	return container.New(layout.NewCustomPaddedLayout(4, 4, 6, 6), card)
}

// minecraftPackPaths returns all existing Minecraft resource_packs directories.
func minecraftPackPaths(name string) []string {
	localAppData := os.Getenv("LOCALAPPDATA")
	if localAppData == "" {
		return nil
	}

	packages := []string{
		filepath.Join("Packages", "Microsoft.MinecraftUWP_8wekyb3d8bbwe", "LocalState", "games", "com.mojang", "resource_packs"),
		filepath.Join("Packages", "Microsoft.Minecraft115_ekx664bjj63nr", "LocalState", "games", "com.mojang", "resource_packs"),
	}

	var result []string
	for _, pkg := range packages {
		base := filepath.Join(localAppData, pkg)
		// check parent exists (resource_packs may not exist yet but com.mojang should)
		parent := filepath.Dir(base)
		if _, err := os.Stat(parent); err == nil {
			result = append(result, filepath.Join(base, sanitizeName(name)))
		}
	}
	return result
}

// installMcpack downloads a .mcpack file and installs it into all existing Minecraft resource_packs directories.
func installMcpack(name, downloadURL string) error {
	dirs := minecraftPackPaths(name)
	if len(dirs) == 0 {
		return fmt.Errorf("папки Minecraft не найдены")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, downloadURL, nil)
	if err != nil {
		return fmt.Errorf("ошибка загрузки: %w", err)
	}
	resp, err := http.DefaultClient.Do(req)
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

	for _, packDir := range dirs {
		if err := os.MkdirAll(packDir, 0755); err != nil {
			continue
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
