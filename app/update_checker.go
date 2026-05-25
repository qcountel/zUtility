package app

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type UpdateStatus struct {
	Checking       bool
	Downloading    bool
	HasUpdate      bool
	CurrentVersion string
	LatestVersion  string
	ReleaseURL     string
	AssetName      string
	AssetURL       string
	DownloadedPath string
	CheckedAt      time.Time
	Error          string
}

type githubRelease struct {
	TagName string               `json:"tag_name"`
	HTMLURL string               `json:"html_url"`
	Name    string               `json:"name"`
	Assets  []githubReleaseAsset `json:"assets"`
}

type githubReleaseAsset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
}

func (app *App) UpdateStatus() UpdateStatus {
	app.updateMu.RLock()
	defer app.updateMu.RUnlock()
	return app.updateState
}

func (app *App) StartUpdateCheck(onDone func(UpdateStatus)) {
	app.updateMu.Lock()
	if app.updateState.Checking {
		status := app.updateState
		app.updateMu.Unlock()
		if onDone != nil {
			onDone(status)
		}
		return
	}
	app.updateState.Checking = true
	app.updateState.Downloading = false
	app.updateState.Error = ""
	app.updateState.CurrentVersion = CurrentVersion
	app.updateState.DownloadedPath = ""
	status := app.updateState
	app.updateMu.Unlock()

	if onDone != nil {
		onDone(status)
	}

	go func() {
		status := UpdateStatus{
			CurrentVersion: CurrentVersion,
			CheckedAt:      time.Now(),
		}

		release, err := fetchLatestRelease(app.ctx)
		if err != nil {
			status.Error = err.Error()
		} else {
			status.LatestVersion = normalizeVersion(release.TagName)
			status.ReleaseURL = release.HTMLURL
			status.AssetName, status.AssetURL = findWindowsAsset(release.Assets)
			status.HasUpdate = compareVersions(status.LatestVersion, CurrentVersion) > 0
		}

		app.updateMu.Lock()
		app.updateState = status
		app.updateState.Checking = false
		status = app.updateState
		app.updateMu.Unlock()

		if onDone != nil {
			onDone(status)
		}
	}()
}

func fetchLatestRelease(ctx context.Context) (githubRelease, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, GitHubLatestReleaseAPI, nil)
	if err != nil {
		return githubRelease{}, err
	}
	request.Header.Set("Accept", "application/vnd.github+json")
	request.Header.Set("User-Agent", "zUtility/"+CurrentVersion)

	client := &http.Client{Timeout: 8 * time.Second}
	response, err := client.Do(request)
	if err != nil {
		return githubRelease{}, err
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		return githubRelease{}, fmt.Errorf("github returned %s", response.Status)
	}

	var release githubRelease
	if err := json.NewDecoder(response.Body).Decode(&release); err != nil {
		return githubRelease{}, err
	}
	if release.TagName == "" {
		if release.Name == "" {
			return githubRelease{}, fmt.Errorf("empty release version")
		}
		release.TagName = release.Name
	}
	if release.HTMLURL == "" {
		release.HTMLURL = GitHubReleasesURL
	}
	return release, nil
}

func (app *App) OpenReleaseURL(rawURL string) error {
	if rawURL == "" {
		rawURL = GitHubReleasesURL
	}
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return err
	}
	return app.app.OpenURL(parsed)
}

func (app *App) DownloadLatestReleaseAsset(onDone func(UpdateStatus)) {
	app.updateMu.Lock()
	status := app.updateState
	if status.Checking || status.Downloading {
		app.updateMu.Unlock()
		if onDone != nil {
			onDone(status)
		}
		return
	}
	if status.AssetURL == "" {
		status.Error = "в последнем релизе не найден .exe asset"
		app.updateState = status
		app.updateMu.Unlock()
		if onDone != nil {
			onDone(status)
		}
		return
	}
	status.Downloading = true
	status.Error = ""
	status.DownloadedPath = ""
	app.updateState = status
	app.updateMu.Unlock()

	if onDone != nil {
		onDone(status)
	}

	go func() {
		path, err := downloadReleaseAsset(app.ctx, status.AssetName, status.AssetURL)

		app.updateMu.Lock()
		status = app.updateState
		status.Downloading = false
		if err != nil {
			status.Error = err.Error()
		} else {
			status.DownloadedPath = path
			if runErr := launchDownloadedUpdate(path); runErr != nil {
				status.Error = runErr.Error()
			} else {
				go func() {
					time.Sleep(700 * time.Millisecond)
					_ = app.Close(false)
				}()
			}
		}
		app.updateState = status
		app.updateMu.Unlock()

		if onDone != nil {
			onDone(status)
		}
	}()
}

func findWindowsAsset(assets []githubReleaseAsset) (string, string) {
	for _, asset := range assets {
		name := strings.ToLower(asset.Name)
		if strings.HasSuffix(name, ".exe") && asset.BrowserDownloadURL != "" {
			return asset.Name, asset.BrowserDownloadURL
		}
	}
	for _, asset := range assets {
		if asset.BrowserDownloadURL != "" {
			return asset.Name, asset.BrowserDownloadURL
		}
	}
	return "", ""
}

func downloadReleaseAsset(ctx context.Context, assetName, assetURL string) (string, error) {
	if assetURL == "" {
		return "", fmt.Errorf("empty asset url")
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, assetURL, nil)
	if err != nil {
		return "", err
	}
	request.Header.Set("User-Agent", "zUtility/"+CurrentVersion)

	client := &http.Client{Timeout: 0}
	response, err := client.Do(request)
	if err != nil {
		return "", err
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		return "", fmt.Errorf("download failed: %s", response.Status)
	}

	currentExe, err := os.Executable()
	if err != nil {
		return "", err
	}
	currentExe, err = filepath.Abs(currentExe)
	if err != nil {
		return "", err
	}
	currentDir := filepath.Dir(currentExe)
	if err := os.MkdirAll(currentDir, 0755); err != nil {
		return "", err
	}
	if assetName == "" || !strings.HasSuffix(strings.ToLower(assetName), ".exe") {
		assetName = filepath.Base(currentExe)
	}
	filePath := filepath.Join(currentDir, filepath.Base(currentExe)+".new")
	tmpPath := filePath + ".download"

	_ = os.Remove(tmpPath)
	if _, err := os.Stat(filePath); err == nil {
		if err := os.Remove(filePath); err != nil {
			return "", err
		}
	}

	file, err := os.Create(tmpPath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	if _, err := io.Copy(file, response.Body); err != nil {
		_ = os.Remove(tmpPath)
		return "", err
	}
	if err := file.Close(); err != nil {
		_ = os.Remove(tmpPath)
		return "", err
	}
	if err := os.Rename(tmpPath, filePath); err != nil {
		_ = os.Remove(tmpPath)
		return "", err
	}
	return filePath, nil
}

func launchDownloadedUpdate(path string) error {
	if path == "" {
		return fmt.Errorf("empty update path")
	}
	currentExe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("resolve current exe: %w", err)
	}
	currentExe, err = filepath.Abs(currentExe)
	if err != nil {
		return fmt.Errorf("abs current exe: %w", err)
	}
	scriptPath, err := writeUpdateScript(currentExe, path)
	if err != nil {
		return err
	}
	cmd := exec.Command("cmd.exe", "/C", scriptPath)
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start updater script: %w", err)
	}
	return nil
}

func writeUpdateScript(currentExe, downloadedExe string) (string, error) {
	if currentExe == "" || downloadedExe == "" {
		return "", fmt.Errorf("empty updater paths")
	}
	scriptPath := filepath.Join(filepath.Dir(currentExe), "zutility_self_update.cmd")
	currentEscaped := escapeBatchPath(currentExe)
	downloadedEscaped := escapeBatchPath(downloadedExe)
	script := strings.Join([]string{
		"@echo off",
		"setlocal",
		":waitloop",
		"timeout /t 1 /nobreak >nul",
		fmt.Sprintf("del /f /q \"%s\" >nul 2>nul", currentEscaped),
		fmt.Sprintf("if exist \"%s\" goto waitloop", currentEscaped),
		fmt.Sprintf("move /y \"%s\" \"%s\" >nul", downloadedEscaped, currentEscaped),
		fmt.Sprintf("if not exist \"%s\" goto waitloop", currentEscaped),
		fmt.Sprintf("start \"\" \"%s\"", currentEscaped),
		fmt.Sprintf("del /f /q \"%s\" >nul 2>nul", escapeBatchPath(scriptPath)),
		"endlocal",
	}, "\r\n")
	if err := os.WriteFile(scriptPath, []byte(script), 0644); err != nil {
		return "", fmt.Errorf("write updater script: %w", err)
	}
	return scriptPath, nil
}

func escapeBatchPath(path string) string {
	return strings.ReplaceAll(path, "\"", "")
}

func normalizeVersion(version string) string {
	trimmed := strings.TrimSpace(strings.TrimPrefix(version, "v"))
	if trimmed == "" {
		return CurrentVersion
	}
	return trimmed
}

func compareVersions(left, right string) int {
	leftParts := versionParts(left)
	rightParts := versionParts(right)
	count := len(leftParts)
	if len(rightParts) > count {
		count = len(rightParts)
	}
	for i := 0; i < count; i++ {
		var l, r int
		if i < len(leftParts) {
			l = leftParts[i]
		}
		if i < len(rightParts) {
			r = rightParts[i]
		}
		if l > r {
			return 1
		}
		if l < r {
			return -1
		}
	}
	return 0
}

func versionParts(version string) []int {
	fields := strings.FieldsFunc(normalizeVersion(version), func(r rune) bool {
		return r < '0' || r > '9'
	})
	parts := make([]int, 0, len(fields))
	for _, field := range fields {
		if field == "" {
			continue
		}
		part, err := strconv.Atoi(field)
		if err != nil {
			continue
		}
		parts = append(parts, part)
	}
	if len(parts) == 0 {
		return []int{0}
	}
	return parts
}
