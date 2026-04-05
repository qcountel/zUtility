package app

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
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

// tappableImage is a canvas.Image that responds to taps.
type tappableImage struct {
	widget.BaseWidget
	img   *canvas.Image
	onTap func()
}

func newTappableImage(img *canvas.Image, onTap func()) *tappableImage {
	t := &tappableImage{img: img, onTap: onTap}
	t.ExtendBaseWidget(t)
	return t
}

func (t *tappableImage) CreateRenderer() fyne.WidgetRenderer {
	return widget.NewSimpleRenderer(t.img)
}

func (t *tappableImage) Tapped(_ *fyne.PointEvent) {
	if t.onTap != nil {
		t.onTap()
	}
}

func (t *tappableImage) TappedSecondary(_ *fyne.PointEvent) {}

// showImageZoom displays the given image in a full-window overlay with fade-in animation.
func (app *App) showImageZoom(src image.Image) {
	if src == nil {
		return
	}

	overlay := app.win.Canvas().Overlays()

	// Dark background
	bg := canvas.NewRectangle(color.NRGBA{R: 0, G: 0, B: 0, A: 0})
	bg.CornerRadius = 0

	// Big image
	bigImg := canvas.NewImageFromImage(src)
	bigImg.FillMode = canvas.ImageFillContain
	bigImg.ScaleMode = canvas.ImageScaleSmooth

	// Inner image container with padding
	imgBox := container.New(layout.NewCustomPaddedLayout(40, 40, 40, 40), bigImg)

	// Close button
	var closeOverlay func()
	closeBtn := widget.NewButtonWithIcon("", theme.CancelIcon(), func() {
		closeOverlay()
	})
	closeBtn.Importance = widget.LowImportance

	closeBtnBox := container.New(layout.NewCustomPaddedLayout(12, 0, 0, 12), closeBtn)
	closeRow := container.NewBorder(nil, nil, nil, closeBtnBox)

	content := container.NewBorder(closeRow, nil, nil, nil, imgBox)

	// Tap on background closes overlay
	bgTap := newTappableSurface(func() { closeOverlay() })
	overlayWidget := container.NewStack(bgTap, bg, content)

	closeOverlay = func() {
		overlay.Remove(overlayWidget)
	}

	overlay.Add(overlayWidget)

	// Fade-in animation for background
	anim := canvas.NewColorRGBAAnimation(
		color.NRGBA{R: 0, G: 0, B: 0, A: 0},
		color.NRGBA{R: 0, G: 0, B: 0, A: 210},
		200*time.Millisecond,
		func(c color.Color) {
			bg.FillColor = c
			bg.Refresh()
		},
	)
	anim.Start()
}

// tappableSurface is an invisible tappable widget that covers its full area.
type tappableSurface struct {
	widget.BaseWidget
	onTap func()
}

func newTappableSurface(onTap func()) *tappableSurface {
	t := &tappableSurface{onTap: onTap}
	t.ExtendBaseWidget(t)
	return t
}

func (t *tappableSurface) CreateRenderer() fyne.WidgetRenderer {
	bg := canvas.NewRectangle(color.NRGBA{0, 0, 0, 0})
	return widget.NewSimpleRenderer(bg)
}

func (t *tappableSurface) Tapped(_ *fyne.PointEvent) {
	if t.onTap != nil {
		t.onTap()
	}
}

func (t *tappableSurface) TappedSecondary(_ *fyne.PointEvent) {}

// buildResourcePacksContent fetches packs.json and builds the scrollable grid UI with search.
func (app *App) buildResourcePacksContent() fyne.CanvasObject {
	loading := widget.NewLabel("Загрузка ресурспаков...")
	loading.Alignment = fyne.TextAlignCenter

	scroll := container.NewVScroll(widget.NewLabel(""))

	// allPacks stores the full list
	var allPacks []Pack

	// Grid rebuild function — filters allPacks by query and updates scroll
	var rebuildGrid func(query string)
	rebuildGrid = func(query string) {
		q := strings.ToLower(strings.TrimSpace(query))
		var cards []fyne.CanvasObject
		for _, p := range allPacks {
			if q == "" || strings.Contains(strings.ToLower(p.Name), q) {
				cards = append(cards, app.buildPackCard(p))
			}
		}
		if len(cards) == 0 {
			scroll.Content = container.New(layout.NewCustomPaddedLayout(20, 20, 20, 20),
				ctxt(app.t("Ничего не найдено", "Nothing found"), textSecondary, 12))
		} else {
			grid := container.NewGridWithColumns(2, cards...)
			scroll.Content = container.New(layout.NewCustomPaddedLayout(4, 4, 4, 4), grid)
		}
		scroll.Refresh()
	}

	// Search field
	searchEntry := widget.NewEntry()
	searchEntry.SetPlaceHolder(app.t("\U0001f50d  Поиск по названию...", "\U0001f50d  Search by name..."))
	searchEntry.OnChanged = func(q string) {
		rebuildGrid(q)
	}
	searchBox := container.New(layout.NewCustomPaddedLayout(0, 8, 14, 14), searchEntry)

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
				allPacks = packs
				rebuildGrid(searchEntry.Text)
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
		searchBox,
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

	// Store loaded image for zoom
	var loadedSrc image.Image

	// Tappable image wrapper — opens zoom overlay on click
	tapImg := newTappableImage(img, func() {
		if loadedSrc != nil {
			app.showImageZoom(loadedSrc)
		}
	})

	// Fixed-height wrapper — ширина тянется по карточке, высота строго 140px
	imgFixed := container.New(fixedHeightLayout{140}, tapImg)

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
			loadedSrc = src
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
					statusLabel.Text = "\u2717 " + err.Error()
					statusLabel.Color = accentOrange
				} else {
					statusLabel.Text = "\u2713 " + app.t("Установлен", "Installed")
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
				continue
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
