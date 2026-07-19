package webui

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	GitHubOwner = "chiabotta35"
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

type gitTagRef struct {
	Ref    string `json:"ref"`
	Object struct {
		SHA  string `json:"sha"`
		Type string `json:"type"`
		URL  string `json:"url"`
	} `json:"object"`
}

var (
	lastUpdateCheck *UpdateInfo
	lastCheckTime   time.Time
	updateCheckMu   sync.Mutex
)

// normalizeVersion strips a leading "v" prefix and whitespace so that
// "v0.1.1" and "0.1.1" compare equal.
func normalizeVersion(v string) string {
	return strings.TrimSpace(strings.TrimPrefix(v, "v"))
}

// parseVersion parses "0.1.4" into [0, 1, 4] for comparison.
func parseVersion(v string) []int {
	v = normalizeVersion(v)
	parts := strings.Split(v, ".")
	var nums []int
	for _, p := range parts {
		n, err := strconv.Atoi(p)
		if err != nil {
			return nil
		}
		nums = append(nums, n)
	}
	return nums
}

// versionLess returns true if a < b.
func versionLess(a, b []int) bool {
	for i := 0; i < len(a) && i < len(b); i++ {
		if a[i] < b[i] {
			return true
		}
		if a[i] > b[i] {
			return false
		}
	}
	return len(a) < len(b)
}

func CheckForUpdate(currentVersion string) (*UpdateInfo, error) {
	return CheckForUpdateForce(currentVersion, false)
}

func CheckForUpdateForce(currentVersion string, force bool) (*UpdateInfo, error) {
	updateCheckMu.Lock()
	defer updateCheckMu.Unlock()

	if !force && time.Since(lastCheckTime) < 60*time.Second && lastUpdateCheck != nil {
		return lastUpdateCheck, nil
	}

	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/git/refs/tags", GitHubOwner, GitHubRepo)
	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to check GitHub tags: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("GitHub API returned %d", resp.StatusCode)
	}

	var tags []gitTagRef
	if err := json.NewDecoder(resp.Body).Decode(&tags); err != nil {
		return nil, fmt.Errorf("failed to parse tags: %w", err)
	}

	// Collect all v* tags, parse their versions, and find the latest.
	curVer := parseVersion(currentVersion)
	var latestTag string
	var latestVer []int

	for _, t := range tags {
		tagName := strings.TrimPrefix(t.Ref, "refs/tags/")
		if !strings.HasPrefix(tagName, "v") {
			continue
		}
		ver := parseVersion(tagName)
		if ver == nil {
			continue
		}
		if latestVer == nil || versionLess(latestVer, ver) {
			latestTag = tagName
			latestVer = ver
		}
	}

	if latestTag == "" || latestVer == nil {
		info := &UpdateInfo{
			Available:  false,
			CurrentVer: currentVersion,
			LatestVer:  currentVersion,
		}
		lastUpdateCheck = info
		lastCheckTime = time.Now()
		return info, nil
	}

	available := curVer == nil || versionLess(curVer, latestVer)
	if available && !isImageReady(client, latestTag) {
		available = false
	}

	info := &UpdateInfo{
		Available:  available,
		CurrentVer: currentVersion,
		LatestVer:  latestTag,
		ReleaseURL: fmt.Sprintf("https://github.com/%s/%s/releases/tag/%s", GitHubOwner, GitHubRepo, latestTag),
	}

	lastUpdateCheck = info
	lastCheckTime = time.Now()
	return info, nil
}

// isImageReady checks whether the Docker image for the given tag has been
// pushed to ghcr.io. It authenticates with the registry, then does a HEAD
// request for the manifest. Returns false if the image isn't available yet.
func isImageReady(client *http.Client, tag string) bool {
	imageRef := fmt.Sprintf("%s/%s", GitHubOwner, GitHubRepo)

	// Get an anonymous pull token from the registry.
	tokenURL := fmt.Sprintf("https://ghcr.io/token?service=ghcr.io&scope=repository:%s:pull", imageRef)
	tokenResp, err := client.Get(tokenURL)
	if err != nil {
		return false
	}
	defer tokenResp.Body.Close()
	if tokenResp.StatusCode != 200 {
		return false
	}
	var tokenData struct {
		Token string `json:"token"`
	}
	if err := json.NewDecoder(tokenResp.Body).Decode(&tokenData); err != nil || tokenData.Token == "" {
		return false
	}

	// HEAD the manifest for the tag.
	manifestURL := fmt.Sprintf("https://ghcr.io/v2/%s/manifests/%s", imageRef, tag)
	req, err := http.NewRequest("HEAD", manifestURL, nil)
	if err != nil {
		return false
	}
	req.Header.Set("Authorization", "Bearer "+tokenData.Token)
	req.Header.Set("Accept", "application/vnd.oci.image.index.v1+json, application/vnd.docker.distribution.manifest.v2+json, application/vnd.docker.distribution.manifest.list.v2+json, application/vnd.oci.image.manifest.v1+json")

	manifestResp, err := client.Do(req)
	if err != nil {
		return false
	}
	defer manifestResp.Body.Close()

	return manifestResp.StatusCode == 200
}
