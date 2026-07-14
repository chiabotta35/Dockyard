package webui

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
)

const (
	GitHubOwner = "dockyard"
	GitHubRepo  = "dockyard"
)

type UpdateInfo struct {
	Available   bool   `json:"available"`
	CurrentVer  string `json:"current_version"`
	LatestVer   string `json:"latest_version"`
	ReleaseURL  string `json:"release_url"`
	PublishedAt string `json:"published_at"`
	Body        string `json:"body"`
}

type GitHubRelease struct {
	TagName     string `json:"tag_name"`
	Name        string `json:"name"`
	Body        string `json:"body"`
	PublishedAt string `json:"published_at"`
	HTMLURL     string `json:"html_url"`
	Assets      []struct {
		Name               string `json:"name"`
		BrowserDownloadURL string `json:"browser_download_url"`
	} `json:"assets"`
}

var lastUpdateCheck *UpdateInfo
var lastCheckTime time.Time

func CheckForUpdate(currentVersion string) (*UpdateInfo, error) {
	if time.Since(lastCheckTime) < 5*time.Minute && lastUpdateCheck != nil {
		return lastUpdateCheck, nil
	}

	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/releases/latest", GitHubOwner, GitHubRepo)
	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to check GitHub: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == 404 {
		info := &UpdateInfo{
			Available:  false,
			CurrentVer: currentVersion,
			LatestVer:  currentVersion,
		}
		lastUpdateCheck = info
		lastCheckTime = time.Now()
		return info, nil
	}

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("GitHub API returned %d", resp.StatusCode)
	}

	var release GitHubRelease
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return nil, fmt.Errorf("failed to parse release: %w", err)
	}

	available := release.TagName != currentVersion && release.TagName != ""
	info := &UpdateInfo{
		Available:   available,
		CurrentVer:  currentVersion,
		LatestVer:   release.TagName,
		ReleaseURL:  release.HTMLURL,
		PublishedAt: release.PublishedAt,
		Body:        release.Body,
	}

	lastUpdateCheck = info
	lastCheckTime = time.Now()
	return info, nil
}

func PerformSelfUpdate(currentVersion string, events *EventHub) error {
	info, err := CheckForUpdate(currentVersion)
	if err != nil {
		return err
	}

	if !info.Available {
		return fmt.Errorf("already on latest version %s", currentVersion)
	}

	events.Broadcast(Event{
		Type:    EventUpdateStarted,
		Message: fmt.Sprintf("Updating Dockyard from %s to %s...", currentVersion, info.LatestVer),
	})

	newTag := strings.TrimPrefix(info.LatestVer, "v")
	arch := runtime.GOARCH

	binaryName := fmt.Sprintf("dockyard_%s_%s", newTag, arch)
	downloadURL := fmt.Sprintf(
		"https://github.com/%s/%s/releases/download/%s/%s",
		GitHubOwner, GitHubRepo, info.LatestVer, binaryName,
	)

	events.BroadcastLog("", "Downloading update...")

	tmpDir, err := os.MkdirTemp("", "dockyard-update-*")
	if err != nil {
		events.Broadcast(Event{Type: EventUpdateFailed, Message: "Failed to create temp directory"})
		return fmt.Errorf("failed to create temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	tmpFile := filepath.Join(tmpDir, "dockyard-new")
	if runtime.GOOS == "windows" {
		tmpFile = filepath.Join(tmpDir, "dockyard-new.exe")
	}

	if err := downloadFile(downloadURL, tmpFile); err != nil {
		events.Broadcast(Event{Type: EventUpdateFailed, Message: "Failed to download update"})
		return fmt.Errorf("download failed: %w", err)
	}

	events.BroadcastLog("", "Verifying download...")

	currentExe, err := os.Executable()
	if err != nil {
		events.Broadcast(Event{Type: EventUpdateFailed, Message: "Failed to determine current binary path"})
		return fmt.Errorf("cannot find current executable: %w", err)
	}
	currentExe, err = filepath.EvalSymlinks(currentExe)
	if err != nil {
		return fmt.Errorf("cannot resolve executable path: %w", err)
	}

	if err := os.Chmod(tmpFile, 0755); err != nil {
		events.Broadcast(Event{Type: EventUpdateFailed, Message: "Failed to set permissions on update"})
		return fmt.Errorf("chmod failed: %w", err)
	}

	backupPath := currentExe + ".bak"
	if err := copyFile(currentExe, backupPath); err != nil {
		logrus.WithError(err).Warn("Failed to backup current binary, continuing anyway")
	}

	if err := copyFile(tmpFile, currentExe); err != nil {
		if backupErr := copyFile(backupPath, currentExe); backupErr != nil {
			events.Broadcast(Event{Type: EventUpdateFailed, Message: "Update failed and rollback also failed"})
			return fmt.Errorf("update failed and rollback failed: %w", err)
		}
		events.Broadcast(Event{Type: EventUpdateFailed, Message: "Update failed, rolled back to previous version"})
		return fmt.Errorf("update failed, rolled back: %w", err)
	}

	os.Remove(backupPath)

	events.Broadcast(Event{
		Type:    EventUpdateComplete,
		Message: fmt.Sprintf("Updated to %s successfully! Restart to apply.", info.LatestVer),
	})

	return nil
}

func downloadFile(url, dest string) error {
	if !strings.HasPrefix(url, "https://") {
		return fmt.Errorf("invalid download URL scheme")
	}

	client := &http.Client{Timeout: 5 * time.Minute}
	resp, err := client.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	out, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer out.Close()

	hasher := sha256.New()
	writer := io.MultiWriter(out, hasher)

	if _, err := io.Copy(writer, resp.Body); err != nil {
		return err
	}

	checksum := hex.EncodeToString(hasher.Sum(nil))
	logrus.WithField("sha256", checksum).Info("Downloaded update checksum")

	return out.Close()
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	if _, err := io.Copy(out, in); err != nil {
		return err
	}

	info, err := in.Stat()
	if err == nil {
		os.Chmod(dst, info.Mode())
	}

	return out.Close()
}

func findDockerUpdateCommand(info *UpdateInfo) string {
	return fmt.Sprintf(
		"docker pull ghcr.io/%s/%s:%s && docker restart dockyard",
		GitHubOwner, GitHubRepo, info.LatestVer,
	)
}

func fetchReleaseNotes(url string) string {
	if !strings.HasPrefix(url, "https://") {
		return ""
	}
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return ""
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024*1024))
	return string(body)
}
