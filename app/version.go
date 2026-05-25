package app

import (
	"strconv"
	"time"
)

const (
	CurrentVersion         = "3.1.1"
	GitHubRepoOwner        = "qcountel"
	GitHubRepoName         = "zutilityupdates"
	GitHubReleasesURL      = "https://github.com/" + GitHubRepoOwner + "/" + GitHubRepoName + "/releases"
	GitHubLatestReleaseAPI = "https://api.github.com/repos/" + GitHubRepoOwner + "/" + GitHubRepoName + "/releases/latest"
)

func (app *App) SessionUptimeText() string {
	startedAt := app.startedAt
	if startedAt.IsZero() {
		startedAt = time.Now()
	}
	return formatDuration(time.Since(startedAt))
}

func formatDuration(duration time.Duration) string {
	if duration < 0 {
		duration = 0
	}
	totalSeconds := int(duration.Round(time.Second) / time.Second)
	hours := totalSeconds / 3600
	minutes := (totalSeconds % 3600) / 60
	seconds := totalSeconds % 60
	if hours > 0 {
		return strconv.Itoa(hours) + " ч " + strconv.Itoa(minutes) + " мин " + strconv.Itoa(seconds) + " сек"
	}
	return strconv.Itoa(minutes) + " мин " + strconv.Itoa(seconds) + " сек"
}
