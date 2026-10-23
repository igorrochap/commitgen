// Package updatecheck checks whether a newer commitgen version is available.
package updatecheck

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"runtime/debug"
	"strconv"
	"strings"
	"time"
)

const (
	// FallbackVersion is used when the binary has no release version metadata.
	FallbackVersion  = "1.1.0"
	latestVersionURL = "https://proxy.golang.org/github.com/igorrochap/commitgen/@latest"
)

type latestVersionResponse struct {
	// Version is the version field returned by the Go module proxy.
	Version string `json:"Version"`
	// TagName is the version field used by GitHub release responses.
	TagName string `json:"tag_name"`
}

// Result contains the current and latest versions and whether an update exists.
type Result struct {
	// Current is the version of the running commitgen binary.
	Current string
	// Latest is the latest version returned by the version service.
	Latest string
	// Newer reports whether Latest is newer than Current.
	Newer bool
}

// Check looks up the latest version using ctx and compares it with the running version.
func Check(ctx context.Context) (Result, error) {
	current := CurrentVersion()
	latest, err := LatestVersion(ctx, http.DefaultClient, latestVersionURL)
	if err != nil {
		return Result{Current: current}, err
	}

	return Result{
		Current: current,
		Latest:  latest,
		Newer:   CompareVersions(latest, current) > 0,
	}, nil
}

// CheckWithTimeout checks for an update using timeout for the request.
func CheckWithTimeout(timeout time.Duration) (Result, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	return Check(ctx)
}

// CurrentVersion returns the version of the running binary or FallbackVersion.
func CurrentVersion() string {
	if info, ok := debug.ReadBuildInfo(); ok && info.Main.Version != "" && info.Main.Version != "(devel)" {
		return info.Main.Version
	}
	return FallbackVersion
}

// LatestVersion fetches and decodes the latest version from url.
func LatestVersion(ctx context.Context, client *http.Client, url string) (version string, err error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "commitgen")

	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			closeErr = fmt.Errorf("close latest version response: %w", closeErr)
			if err == nil {
				err = closeErr
				return
			}
			err = errors.Join(err, closeErr)
		}
	}()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("latest version returned status %s", resp.Status)
	}

	var latest latestVersionResponse
	if err := json.NewDecoder(resp.Body).Decode(&latest); err != nil {
		return "", err
	}
	version = latest.Version
	if version == "" {
		version = latest.TagName
	}
	if strings.TrimSpace(version) == "" {
		return "", fmt.Errorf("latest version response missing version")
	}
	return version, nil
}

// CompareVersions compares semantic version prefixes and returns -1, 0, or 1.
func CompareVersions(left, right string) int {
	leftParts := versionParts(left)
	rightParts := versionParts(right)

	for i := 0; i < 3; i++ {
		if leftParts[i] > rightParts[i] {
			return 1
		}
		if leftParts[i] < rightParts[i] {
			return -1
		}
	}
	return 0
}

func versionParts(version string) [3]int {
	version = strings.TrimSpace(version)
	version = strings.TrimPrefix(version, "v")
	version = strings.Split(version, "-")[0]

	var parts [3]int
	for i, part := range strings.Split(version, ".") {
		if i >= len(parts) {
			break
		}
		n, err := strconv.Atoi(part)
		if err != nil {
			continue
		}
		parts[i] = n
	}
	return parts
}
