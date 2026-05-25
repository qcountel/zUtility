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
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"github.com/nfnt/resize"
)

const packsJSONURL = "https://raw.githubusercontent.com/qcountel/packs/main/packs.json"

// Максимальные размеры загружаемых данных.
const (
	maxPacksJSONSize = 1 << 20  // 1 MB — список паков
	maxImageSize     = 10 << 20 // 10 MB — одна превью-картинка
	maxMcpackSize    = 50 << 20 // 50 MB — mcpack "архив"
)

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

// showImageZoom displays the given image in a full-screen modal popup.
//
// Animation strategy: PopUp is full-screen from the start and never resized.
// The canvas.Image animates from the thumbnail's screen position (origin/originSize)
// to fill the entire canvas — "hero" expand effect.
// Background overlay fades in in parallel.
//
// Fyne's animation runner calls Tick() every render frame (vsync-locked, ≥60fps).
// ImageScalePixels (GPU nearest-neighbour) is used during animation to avoid
// per-frame CPU resampling; ImageScaleSmooth is applied only when animation ends.
//
// Easing: easeOutQuart (fast start, smooth brake) for open;
// easeInQuad (smooth acceleration) for close. Both animate position + size together.
func (app *App) showImageZoom(src image.Image, origin fyne.Position, originSize fyne.Size) {
	if src == nil {
		return
	}

	// No animations: show full-size instantly without hero transition.
	if !app.animationsEnabled {
		fullSize := app.win.Canvas().Size()
		bigImg := canvas.NewImageFromImage(src)
		bigImg.FillMode = canvas.ImageFillContain
		bigImg.ScaleMode = canvas.ImageScaleSmooth
		bigImg.Resize(fullSize)
		bigImg.Move(fyne.NewPos(0, 0))

		bg := canvas.NewRectangle(color.NRGBA{R: 0, G: 0, B: 0, A: 200})
		bg.Resize(fullSize)
		bg.Move(fyne.NewPos(0, 0))
		imgLayer := container.NewWithoutLayout(bg, bigImg)

		var popup *widget.PopUp
		doClose := func() { fyne.Do(popup.Hide) }
		closeBtn := widget.NewButtonWithIcon("", theme.CancelIcon(), doClose)
		closeBtn.Importance = widget.LowImportance
		uiLayer := container.NewBorder(
			container.New(layout.NewCustomPaddedLayout(8, 0, 8, 0),
				container.NewHBox(layout.NewSpacer(), closeBtn)),
			nil, nil, nil,
		)
		content := container.New(layout.NewStackLayout(), imgLayer, uiLayer)
		popup = widget.NewModalPopUp(content, app.win.Canvas())
		popup.Resize(fullSize)
		popup.Move(fyne.NewPos(0, 0))
		popup.Show()
		return
	}

	const (
		animDur    = 380 * time.Millisecond // open duration
		closeDur   = 260 * time.Millisecond // close duration
		maxBgAlpha = uint8(200)
	)

	fullSize := app.win.Canvas().Size()
	targetPos := fyne.NewPos(0, 0) // full-screen target inside popup

	lerp := func(a, b, t float32) float32 { return a + (b-a)*t }
	lerpPos := func(a, b fyne.Position, t float32) fyne.Position {
		return fyne.NewPos(lerp(a.X, b.X, t), lerp(a.Y, b.Y, t))
	}

	// easeOutQuart: 1-(1-t)^4 — fast start, very smooth brake. No math imports needed.
	easeOutQuart := func(t float32) float32 {
		t1 := 1 - t
		return 1 - t1*t1*t1*t1
	}

	// easeInQuad: t^2 — smooth acceleration for close.
	easeInQuad := func(t float32) float32 {
		return t * t
	}

	// Dimming overlay: transparent → maxBgAlpha.
	bg := canvas.NewRectangle(color.NRGBA{R: 0, G: 0, B: 0, A: 0})
	bg.Resize(fullSize)
	bg.Move(fyne.NewPos(0, 0))

	imgW := float32(src.Bounds().Dx())
	imgH := float32(src.Bounds().Dy())

	calcContain := func(boxSize fyne.Size, boxPos fyne.Position) (fyne.Size, fyne.Position) {
		if imgW == 0 || imgH == 0 {
			return boxSize, boxPos
		}
		scaleX := boxSize.Width / imgW
		scaleY := boxSize.Height / imgH
		scale := scaleX
		if scaleY < scaleX {
			scale = scaleY
		}
		targetW := imgW * scale
		targetH := imgH * scale
		targetX := boxPos.X + (boxSize.Width-targetW)/2
		targetY := boxPos.Y + (boxSize.Height-targetH)/2
		return fyne.NewSize(targetW, targetH), fyne.NewPos(targetX, targetY)
	}

	startSize, startPos := calcContain(originSize, origin)
	endSize, endPos := calcContain(fullSize, targetPos)

	// bigImg starts at the thumbnail's screen position and size.
	bigImg := canvas.NewImageFromImage(src)
	bigImg.FillMode = canvas.ImageFillStretch
	bigImg.ScaleMode = canvas.ImageScalePixels
	bigImg.Resize(startSize)
	bigImg.Move(startPos)

	// imgLayer: free layout container redrawn in one pass per frame.
	imgLayer := container.NewWithoutLayout(bg, bigImg)

	// applyFrame updates geometry + overlay alpha and triggers one container refresh.
	applyFrame := func(sz fyne.Size, pos fyne.Position, bgAlpha uint8) {
		bg.FillColor = color.NRGBA{R: 0, G: 0, B: 0, A: bgAlpha}
		bigImg.Resize(sz)
		bigImg.Move(pos)
		canvas.Refresh(imgLayer)
	}

	var popup *widget.PopUp
	var openAnim *fyne.Animation
	closing := false

	doClose := func() {
		if closing {
			return
		}
		closing = true
		if openAnim != nil {
			openAnim.Stop()
		}
		bigImg.ScaleMode = canvas.ImageScalePixels
		snapSz := bigImg.Size()
		snapPos := bigImg.Position()
		snapAlpha := bg.FillColor.(color.NRGBA).A
		closeAnim := &fyne.Animation{
			Duration: closeDur,
			Curve:    fyne.AnimationLinear, // manual curve via easeInQuad
			Tick: func(p float32) {
				e := easeInQuad(p)
				alpha := uint8(float32(snapAlpha) * (1 - e))
				sz := fyne.NewSize(lerp(snapSz.Width, startSize.Width, e), lerp(snapSz.Height, startSize.Height, e))
				pos := lerpPos(snapPos, startPos, e)
				applyFrame(sz, pos, alpha)
				if p >= 1.0 {
					fyne.Do(popup.Hide)
				}
			},
		}
		closeAnim.Start()
	}

	closeBtn := widget.NewButtonWithIcon("", theme.CancelIcon(), doClose)
	closeBtn.Importance = widget.LowImportance

	uiLayer := container.NewBorder(
		container.New(layout.NewCustomPaddedLayout(8, 0, 8, 0),
			container.NewHBox(layout.NewSpacer(), closeBtn)),
		nil, nil, nil,
	)
	content := container.New(layout.NewStackLayout(), imgLayer, uiLayer)

	// PopUp is full-screen from the start — never resized after Show().
	popup = widget.NewModalPopUp(content, app.win.Canvas())
	popup.Resize(fullSize)
	popup.Move(fyne.NewPos(0, 0))
	popup.Show()

	// Open animation: hero expand from thumbnail origin → full canvas.
	// BG fades quickly (p*2) so it's visible before image finishes expanding.
	openAnim = &fyne.Animation{
		Duration: animDur,
		Curve:    fyne.AnimationLinear, // manual easing via easeOutQuart
		Tick: func(p float32) {
			e := easeOutQuart(p)
			alpha := uint8(float32(maxBgAlpha) * clampF32(p*2, 0, 1))
			sz := fyne.NewSize(lerp(startSize.Width, endSize.Width, e), lerp(startSize.Height, endSize.Height, e))
			pos := lerpPos(startPos, endPos, e)
			applyFrame(sz, pos, alpha)
			if p >= 1.0 {
				bigImg.ScaleMode = canvas.ImageScaleSmooth
				canvas.Refresh(imgLayer)
			}
		},
	}
	openAnim.Start()
}

// clampF32 clamps v to [lo, hi].
func clampF32(v, lo, hi float32) float32 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

// buildOnlinePacksView fetches packs.json and builds the scrollable grid UI with search.
func (app *App) buildOnlinePacksView() fyne.CanvasObject {
	loading := widget.NewLabel("Загрузка ресурспаков...")
	loading.Alignment = fyne.TextAlignCenter

	scroll := container.NewVScroll(widget.NewLabel(""))

	// allPacks stores the full list; filtered by search query
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
				ctxt("Ничего не найдено", textSecondary, 12))
		} else {
			// Build rows manually — each row is its own 2-column grid.
			// This guarantees the last odd card never stretches to full width.
			var rows []fyne.CanvasObject
			for i := 0; i < len(cards); i += 2 {
				if i+1 < len(cards) {
					rows = append(rows, container.NewGridWithColumns(2, cards[i], cards[i+1]))
				} else {
					empty := canvas.NewRectangle(color.Transparent)
					rows = append(rows, container.NewGridWithColumns(2, cards[i], empty))
				}
			}
			scroll.Content = container.New(layout.NewCustomPaddedLayout(4, 4, 4, 4),
				container.NewVBox(rows...))
		}
		scroll.Refresh()
	}

	// Search field — no emoji icon to avoid rendering issues on Windows
	searchEntry := widget.NewEntry()
	searchEntry.SetPlaceHolder("Поиск по названию...")
	searchEntry.OnChanged = func(q string) {
		rebuildGrid(q)
	}
	searchBox := container.New(layout.NewCustomPaddedLayout(0, 8, 14, 14), searchEntry)

	loadPacks := func() {
		loading.Show()
		loading.SetText("Загрузка ресурспаков...")
		scroll.Content = widget.NewLabel("")
		scroll.Refresh()

		go func() {
			packs, err := fetchPacks()
			fyne.Do(func() {
				loading.Hide()
				if err != nil {
					loading.SetText("Ошибка загрузки: " + err.Error())
					loading.Show()
					return
				}
				allPacks = packs
				rebuildGrid(searchEntry.Text)
			})
		}()
	}

	header := container.NewVBox(
		searchBox,
		container.New(layout.NewCustomPaddedLayout(4, 8, 14, 14), loading),
	)

	loadPacks()

	return container.NewBorder(header, nil, nil, nil, scroll)
}

// buildResourcePacksContent builds the main tab layout with ONLINE / INSTALLED sub-tabs.
func (app *App) buildResourcePacksContent() fyne.CanvasObject {
	subTabs := []string{"ONLINE", "INSTALLED"}
	activeSubTab := "ONLINE"

	mainContainer := container.NewStack()

	var rebuildPacksContent func()

	onlineView := app.buildOnlinePacksView()

	if app.installedView != nil {
		mainContainer.Objects = []fyne.CanvasObject{onlineView, app.installedView}
	} else {
		mainContainer.Objects = []fyne.CanvasObject{onlineView}
	}

	rebuildPacksContent = func() {
		if activeSubTab == "ONLINE" {
			onlineView.Show()
			if app.installedView != nil {
				app.installedView.Hide()
			}
		} else {
			onlineView.Hide()
			if app.installedView == nil || app.installedPacksDirty {
				app.installedPacksDirty = false
				if app.installedView != nil {
					mainContainer.Remove(app.installedView)
				}
				app.installedView, app.refreshInstalled = app.buildInstalledPacksView(func() {
					app.installedPacksDirty = true
					rebuildPacksContent()
				})
				mainContainer.Add(app.installedView)
			}
			app.installedView.Show()
		}
		mainContainer.Refresh()
	}

	rebuildPacksContent()

	type subTabBtn struct {
		text *canvas.Text
		line *canvas.Rectangle
	}
	btns := make([]subTabBtn, len(subTabs))

	updateActiveSubTab := func(selected string) {
		for i, name := range subTabs {
			if name == selected {
				btns[i].text.Color = accentRed
				btns[i].line.FillColor = accentRed
			} else {
				btns[i].text.Color = chex(0x44, 0x44, 0x44)
				btns[i].line.FillColor = color.Transparent
			}
			btns[i].text.Refresh()
			btns[i].line.Refresh()
		}
	}

	var tabItems []fyne.CanvasObject
	for i, name := range subTabs {
		idx := i
		tabName := name

		lbl := ctxt(tabName, chex(0x44, 0x44, 0x44), 14)
		lbl.TextStyle = fyne.TextStyle{Monospace: true, Bold: true}

		underline := canvas.NewRectangle(color.Transparent)
		underline.SetMinSize(fyne.NewSize(0, 2))

		if tabName == activeSubTab {
			lbl.Color = accentRed
			underline.FillColor = accentRed
		}

		btns[idx] = subTabBtn{text: lbl, line: underline}

		lblPadded := container.New(layout.NewCustomPaddedLayout(10, 8, 14, 14), lbl)
		btnCol := container.NewVBox(lblPadded, underline)

		tap := newNavTapArea(func() {
			activeSubTab = tabName
			updateActiveSubTab(tabName)
			rebuildPacksContent()
		})

		tabItems = append(tabItems, container.NewStack(btnCol, tap))
	}

	subToolbar := container.NewHBox(tabItems...)
	subBorder := canvas.NewRectangle(borderRow)
	subBorder.SetMinSize(fyne.NewSize(0, 1))

	headerWithBorder := container.NewVBox(
		container.New(layout.NewCustomPaddedLayout(4, 4, 4, 4), subToolbar),
		subBorder,
	)

	return container.NewBorder(headerWithBorder, nil, nil, nil, mainContainer)
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

	limited := io.LimitReader(resp.Body, maxPacksJSONSize+1)
	var packs []Pack
	if err := json.NewDecoder(limited).Decode(&packs); err != nil {
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

	// Tappable image wrapper — opens zoom modal on click.
	// tapImg is declared first so the closure can reference it legally in Go.
	var tapImg *tappableImage
	tapImg = newTappableImage(img, func() {
		if loadedSrc != nil {
			// Get the thumbnail's absolute canvas position so the hero animation
			// starts exactly where the image is on screen.
			origin := fyne.CurrentApp().Driver().AbsolutePositionForObject(tapImg)
			originSize := tapImg.Size()
			app.showImageZoom(loadedSrc, origin, originSize)
		}
	})

	// Fixed-height wrapper — width stretches with card, height is 140px
	imgFixed := container.New(fixedHeightLayout{140}, tapImg)

	// Async image load with size limit
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
		data, err := io.ReadAll(io.LimitReader(resp.Body, maxImageSize))
		if err != nil {
			return
		}
		src, _, err := image.Decode(bytes.NewReader(data))
		if err != nil {
			return
		}

		// Downscale huge images to max 1280x720 to ensure 60fps animations
		src = resize.Thumbnail(1280, 720, src, resize.Bilinear)

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

	installBtn := widget.NewButtonWithIcon("Установить", theme.DownloadIcon(), nil)
	installBtn.Importance = widget.LowImportance
	installBtn.OnTapped = func() {
		installBtn.Disable()
		fyne.Do(func() { statusLabel.Text = "Загрузка..."; statusLabel.Refresh() })
		go func() {
			err := installMcpack(p.Name, p.DownloadURL, app.useClonedMinecraft)
			fyne.Do(func() {
				if err != nil {
					statusLabel.Text = "✗ " + err.Error()
					statusLabel.Color = accentOrange
				} else {
					statusLabel.Text = "✓ Установлен"
					statusLabel.Color = accentGreen
					app.installedPacksDirty = true
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
	cardBg.CornerRadius = 3
	cardBorder := canvas.NewRectangle(borderRed)
	cardBorder.CornerRadius = 3

	inner := container.NewVBox(cardItems...)
	paddedInner := container.New(layout.NewCustomPaddedLayout(10, 10, 10, 10), inner)
	card := container.NewStack(cardBorder, cardBg, paddedInner)
	return container.New(layout.NewCustomPaddedLayout(4, 4, 6, 6), card)
}

// minecraftPackPaths returns all existing Minecraft resource_packs directories.
func minecraftPackPaths(name string, clonedOnly bool) []string {
	localAppData := os.Getenv("LOCALAPPDATA")
	if localAppData == "" {
		return nil
	}

	var packages []string
	if clonedOnly {
		// Только клонированный Minecraft
		packages = []string{
			filepath.Join("Packages", "Microsoft.Minecraft115_ekx664bjj63nr", "LocalState", "games", "com.mojang", "resource_packs"),
		}
	} else {
		// Оба пути (стандартный и клонированный)
		packages = []string{
			filepath.Join("Packages", "Microsoft.MinecraftUWP_8wekyb3d8bbwe", "LocalState", "games", "com.mojang", "resource_packs"),
			filepath.Join("Packages", "Microsoft.Minecraft115_ekx664bjj63nr", "LocalState", "games", "com.mojang", "resource_packs"),
		}
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
func installMcpack(name, downloadURL string, clonedOnly bool) error {
	dirs := minecraftPackPaths(name, clonedOnly)
	if len(dirs) == 0 {
		return fmt.Errorf("minecraft folders not found")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, downloadURL, nil)
	if err != nil {
		return fmt.Errorf("download request: %w", err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("download: %w", err)
	}
	defer resp.Body.Close()

	// Limit download size to prevent OOM from malicious/huge files.
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxMcpackSize+1))
	if err != nil {
		return fmt.Errorf("read response: %w", err)
	}
	if int64(len(data)) > maxMcpackSize {
		return fmt.Errorf("pack file too large (> %d MB)", maxMcpackSize>>20)
	}

	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return fmt.Errorf("open archive: %w", err)
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

type InstalledPack struct {
	FolderName  string
	FullPath    string
	DisplayName string
	Description string
	IconPath    string
}

func getInstalledPacksDirs(clonedOnly bool) []string {
	localAppData := os.Getenv("LOCALAPPDATA")
	if localAppData == "" {
		return nil
	}

	var packages []string
	if clonedOnly {
		packages = []string{
			filepath.Join("Packages", "Microsoft.Minecraft115_ekx664bjj63nr", "LocalState", "games", "com.mojang", "resource_packs"),
		}
	} else {
		packages = []string{
			filepath.Join("Packages", "Microsoft.MinecraftUWP_8wekyb3d8bbwe", "LocalState", "games", "com.mojang", "resource_packs"),
			filepath.Join("Packages", "Microsoft.Minecraft115_ekx664bjj63nr", "LocalState", "games", "com.mojang", "resource_packs"),
		}
	}

	var result []string
	for _, pkg := range packages {
		base := filepath.Join(localAppData, pkg)
		if _, err := os.Stat(base); err == nil {
			result = append(result, base)
		}
	}
	return result
}

func scanInstalledPack(folderPath string) InstalledPack {
	folderName := filepath.Base(folderPath)
	pack := InstalledPack{
		FolderName:  folderName,
		FullPath:    folderPath,
		DisplayName: folderName,
	}

	manifestPaths := []string{
		filepath.Join(folderPath, "pack_manifest.json"),
		filepath.Join(folderPath, "manifest.json"),
	}

	for _, mp := range manifestPaths {
		if data, err := os.ReadFile(mp); err == nil {
			type Manifest struct {
				Header struct {
					Name        string `json:"name"`
					Description string `json:"description"`
				} `json:"header"`
			}
			var m Manifest
			if err := json.Unmarshal(data, &m); err == nil {
				if m.Header.Name != "" {
					pack.DisplayName = m.Header.Name
				}
				pack.Description = m.Header.Description
				break
			}
		}
	}

	iconPath := filepath.Join(folderPath, "pack_icon.png")
	if _, err := os.Stat(iconPath); err == nil {
		pack.IconPath = iconPath
	}

	return pack
}

func (app *App) getInstalledPacks() []InstalledPack {
	dirs := getInstalledPacksDirs(app.useClonedMinecraft)
	var packs []InstalledPack
	seen := make(map[string]bool)

	for _, d := range dirs {
		entries, err := os.ReadDir(d)
		if err != nil {
			continue
		}
		for _, entry := range entries {
			if entry.IsDir() {
				path := filepath.Join(d, entry.Name())
				pack := scanInstalledPack(path)
				if !seen[pack.FolderName] {
					seen[pack.FolderName] = true
					packs = append(packs, pack)
				}
			}
		}
	}
	return packs
}

func zipFolder(srcFolder, destZipPath string) error {
	zipFile, err := os.Create(destZipPath)
	if err != nil {
		return err
	}
	defer zipFile.Close()

	archive := zip.NewWriter(zipFile)
	defer archive.Close()

	err = filepath.Walk(srcFolder, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		relPath, err := filepath.Rel(srcFolder, path)
		if err != nil {
			return err
		}
		if relPath == "." {
			return nil
		}

		header, err := zip.FileInfoHeader(info)
		if err != nil {
			return err
		}

		header.Name = filepath.ToSlash(relPath)

		if info.IsDir() {
			header.Name += "/"
		} else {
			header.Method = zip.Deflate
		}

		writer, err := archive.CreateHeader(header)
		if err != nil {
			return err
		}

		if info.IsDir() {
			return nil
		}

		file, err := os.Open(path)
		if err != nil {
			return err
		}
		defer file.Close()

		_, err = io.Copy(writer, file)
		return err
	})

	return err
}

func (app *App) buildInstalledPackRow(pack InstalledPack, refreshFn func()) fyne.CanvasObject {
	var img fyne.CanvasObject
	if pack.IconPath != "" {
		srcImg := canvas.NewImageFromFile(pack.IconPath)
		srcImg.FillMode = canvas.ImageFillContain
		srcImg.ScaleMode = canvas.ImageScaleSmooth
		srcImg.SetMinSize(fyne.NewSize(54, 54))
		img = srcImg
	} else {
		rect := canvas.NewRectangle(bgCard)
		rect.StrokeWidth = 1
		rect.StrokeColor = borderRed
		rect.SetMinSize(fyne.NewSize(54, 54))
		img = rect
	}
	imgContainer := container.NewStack(img)

	title := ctxt(pack.DisplayName, textPrimary, 14)
	title.TextStyle = fyne.TextStyle{Bold: true}

	desc := widget.NewLabel(pack.Description)
	desc.Wrapping = fyne.TextWrapWord

	textCol := container.NewVBox(
		title,
		container.New(layout.NewCustomPaddedLayout(2, 0, 0, 0), desc),
	)

	backupBtn := widget.NewButtonWithIcon("Бэкап", theme.DocumentSaveIcon(), func() {
		saveDlg := dialog.NewFileSave(func(writer fyne.URIWriteCloser, err error) {
			if err != nil || writer == nil {
				return
			}
			writer.Close()

			destPath := writer.URI().Path()
			if !strings.HasSuffix(strings.ToLower(destPath), ".mcpack") {
				_ = os.Remove(destPath)
				destPath = destPath + ".mcpack"
			}

			err = zipFolder(pack.FullPath, destPath)
			if err != nil {
				dialog.ShowError(err, app.win)
			} else {
				dialog.ShowInformation("Успешно", "Бэкап ресурспака успешно создан!", app.win)
			}
		}, app.win)

		saveDlg.SetFileName(pack.DisplayName + ".mcpack")
		saveDlg.Show()
	})
	backupBtn.Importance = widget.LowImportance

	deleteBtn := widget.NewButtonWithIcon("Удалить", theme.DeleteIcon(), func() {
		dialog.ShowConfirm("Удаление", "Вы действительно хотите удалить ресурспак "+pack.DisplayName+"?", func(confirmed bool) {
			if confirmed {
				err := os.RemoveAll(pack.FullPath)
				if err != nil {
					dialog.ShowError(err, app.win)
				} else {
					refreshFn()
				}
			}
		}, app.win)
	})
	deleteBtn.Importance = widget.LowImportance

	rowContent := container.NewBorder(
		nil, nil,
		imgContainer,
		container.NewHBox(backupBtn, deleteBtn),
		container.New(layout.NewCustomPaddedLayout(0, 0, 12, 12), textCol),
	)

	rowBg := canvas.NewRectangle(bgCard)
	rowBg.CornerRadius = 0

	rowBorder := canvas.NewRectangle(color.Transparent)
	rowBorder.StrokeWidth = 1
	rowBorder.StrokeColor = borderSubtle
	rowBorder.CornerRadius = 0

	paddedContent := container.New(layout.NewCustomPaddedLayout(10, 10, 12, 12), rowContent)
	return container.NewStack(rowBg, paddedContent, rowBorder)
}

func (app *App) buildInstalledPacksView(refreshFn func()) (fyne.CanvasObject, func()) {
	loading := widget.NewLabel("Сканирование установленных паков...")
	loading.Alignment = fyne.TextAlignCenter

	mainStack := container.NewStack(container.NewCenter(loading))

	refresh := func() {
		go func() {
			packs := app.getInstalledPacks()
			fyne.Do(func() {
				if len(packs) == 0 {
					mainStack.Objects = []fyne.CanvasObject{
						container.New(layout.NewCustomPaddedLayout(20, 20, 20, 20),
							container.NewCenter(ctxt("У вас нет установленных ресурспаков", textSecondary, 14))),
					}
				} else {
					var rows []fyne.CanvasObject
					for _, p := range packs {
						rows = append(rows, app.buildInstalledPackRow(p, refreshFn))
					}
					mainStack.Objects = []fyne.CanvasObject{
						container.NewVScroll(container.NewVBox(rows...)),
					}
				}
				mainStack.Refresh()
			})
		}()
	}

	refresh()

	return mainStack, refresh
}
