package app

import (
	"archive/zip"
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
	"fyne.io/fyne/v2/widget"
)

// packsJSONURL — прямая ссылка на packs.json в репозитории.
const packsJSONURL = "https://raw.githubusercontent.com/qcountel/zUtility/main/packs.json"

// Pack описывает один ресурспак из packs.json.
type Pack struct {
	Name        string `json:"name"`
	ImageURL    string `json:"image_url"`
	DownloadURL string `json:"download_url"`
}

// buildResourcePacksContent создаёт вкладку со списком ресурспаков.
func (app *App) buildResourcePacksContent() fyne.CanvasObject {
	title := ctxt(app.t("Ресурспаки", "Resource Packs"), textPrimary, 18)
	title.TextStyle = fyne.TextStyle{Bold: true}

	titleAccent := canvas.NewRectangle(accentRed)
	titleAccent.SetMinSize(fyne.NewSize(36, 2))
	titleAccent.CornerRadius = 1

	grid := container.NewGridWithColumns(2)

	loadingLabel := ctxt(app.t("Загрузка паков...", "Loading packs..."), textSecondary, 13)
	grid.Add(container.New(layout.NewCustomPaddedLayout(10, 10, 10, 10), loadingLabel))

	go func() {
		packs, err := fetchPacks()
		fyne.Do(func() {
			grid.RemoveAll()
			if err != nil {
				errLabel := ctxt(fmt.Sprintf("Ошибка: %v", err), accentRedBright, 12)
				grid.Add(container.New(layout.NewCustomPaddedLayout(10, 10, 10, 10), errLabel))
				grid.Refresh()
				return
			}
			if len(packs) == 0 {
				emptyLabel := ctxt(app.t("Паки не найдены", "No packs found"), textSecondary, 12)
				grid.Add(container.New(layout.NewCustomPaddedLayout(10, 10, 10, 10), emptyLabel))
				grid.Refresh()
				return
			}
			for _, p := range packs {
				pack := p
				grid.Add(app.buildPackCard(pack))
			}
			grid.Refresh()
		})
	}()

	content := container.NewVBox(
		container.New(layout.NewCustomPaddedLayout(16, 4, 14, 14), title),
		container.New(layout.NewCustomPaddedLayout(0, 10, 14, 14),
			container.New(layout.NewCustomPaddedLayout(0, 0, 0, 0), titleAccent)),
		container.New(layout.NewCustomPaddedLayout(0, 8, 8, 8), grid),
	)

	return container.NewVScroll(content)
}

// fetchPacks скачивает и парсит packs.json.
func fetchPacks() ([]Pack, error) {
	resp, err := http.Get(packsJSONURL)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	var packs []Pack
	if err := json.NewDecoder(resp.Body).Decode(&packs); err != nil {
		return nil, err
	}
	return packs, nil
}

// buildPackCard создаёт карточку пака: превью → название → кнопка Установить.
func (app *App) buildPackCard(p Pack) fyne.CanvasObject {
	imgPlaceholder := canvas.NewRectangle(bgElevated)
	imgPlaceholder.CornerRadius = 6
	imgPlaceholder.SetMinSize(fyne.NewSize(0, 110))

	imgStack := container.NewStack(imgPlaceholder)

	go func() {
		if p.ImageURL == "" {
			return
		}
		resp, err := http.Get(p.ImageURL)
		if err != nil {
			return
		}
		defer resp.Body.Close()

		img, _, err := image.Decode(resp.Body)
		if err != nil {
			return
		}

		fyneImg := canvas.NewImageFromImage(img)
		fyneImg.FillMode = canvas.ImageFillContain
		fyneImg.SetMinSize(fyne.NewSize(0, 110))

		fyne.Do(func() {
			imgStack.Objects = []fyne.CanvasObject{imgPlaceholder, fyneImg}
			imgStack.Refresh()
		})
	}()

	nameText := ctxt(p.Name, textPrimary, 12)
	nameText.TextStyle = fyne.TextStyle{Bold: true}

	statusLabel := ctxt("", textSecondary, 10)
	statusLabel.Hide()

	var installBtn *widget.Button
	installBtn = widget.NewButton(app.t("Установить", "Install"), func() {
		installBtn.Disable()
		fyne.Do(func() {
			statusLabel.Text = app.t("Загрузка...", "Downloading...")
			statusLabel.Color = textSecondary
			statusLabel.Show()
			statusLabel.Refresh()
		})
		go func() {
			err := app.installPackZip(p)
			fyne.Do(func() {
				if err != nil {
					statusLabel.Text = fmt.Sprintf("✗ %v", err)
					statusLabel.Color = accentRedBright
					installBtn.Enable()
				} else {
					statusLabel.Text = app.t("✓ Установлен", "✓ Installed")
					statusLabel.Color = accentGreen
				}
				statusLabel.Refresh()
			})
		}()
	})

	inner := container.NewVBox(
		imgStack,
		container.New(layout.NewCustomPaddedLayout(6, 2, 4, 4), nameText),
		container.New(layout.NewCustomPaddedLayout(0, 4, 4, 4), installBtn),
		container.New(layout.NewCustomPaddedLayout(0, 2, 4, 4), statusLabel),
	)

	cardBg := canvas.NewRectangle(bgCard)
	cardBg.CornerRadius = 8

	padded := container.New(layout.NewCustomPaddedLayout(8, 8, 8, 8), inner)
	card := container.NewStack(cardBg, padded)

	return container.New(layout.NewCustomPaddedLayout(4, 4, 4, 4), card)
}

// installPackZip скачивает .zip и распаковывает в папку ресурспаков Minecraft.
func (app *App) installPackZip(p Pack) error {
	resp, err := http.Get(p.DownloadURL)
	if err != nil {
		return fmt.Errorf("download: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download HTTP %d", resp.StatusCode)
	}

	tmp, err := os.CreateTemp("", "pack-*.zip")
	if err != nil {
		return fmt.Errorf("temp file: %w", err)
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)

	if _, err := io.Copy(tmp, resp.Body); err != nil {
		tmp.Close()
		return fmt.Errorf("write: %w", err)
	}
	tmp.Close()

	localAppData := os.Getenv("LOCALAPPDATA")
	destDir := filepath.Join(
		localAppData,
		`Packages\Microsoft.MinecraftUWP_8wekyb3d8bbwe\LocalState\games\com.mojang\resource_packs`,
		sanitizePackName(p.Name),
	)

	if err := os.MkdirAll(destDir, 0o755); err != nil {
		return fmt.Errorf("mkdir: %w", err)
	}

	if err := extractZipTo(tmpPath, destDir); err != nil {
		return fmt.Errorf("extract: %w", err)
	}

	app.conf.Logger.Info("resource pack installed", "name", p.Name, "dest", destDir)
	return nil
}

func sanitizePackName(name string) string {
	rep := strings.NewReplacer(
		" ", "_", "/", "_", "\\", "_",
		":", "_", "*", "_", "?", "_",
		"\"", "_", "<", "_", ">", "_", "|", "_",
	)
	return rep.Replace(name)
}

func extractZipTo(src, dest string) error {
	r, err := zip.OpenReader(src)
	if err != nil {
		return err
	}
	defer r.Close()

	cleanDest := filepath.Clean(dest) + string(os.PathSeparator)

	for _, f := range r.File {
		outPath := filepath.Join(dest, filepath.Clean(f.Name))
		if !strings.HasPrefix(outPath, cleanDest) {
			continue
		}
		if f.FileInfo().IsDir() {
			if err := os.MkdirAll(outPath, 0o755); err != nil {
				return err
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(outPath), 0o755); err != nil {
			return err
		}
		out, err := os.Create(outPath)
		if err != nil {
			return err
		}
		rc, err := f.Open()
		if err != nil {
			out.Close()
			return err
		}
		_, copyErr := io.Copy(out, rc)
		rc.Close()
		out.Close()
		if copyErr != nil {
			return copyErr
		}
	}
	return nil
}
