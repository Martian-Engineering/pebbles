package main

import (
	"archive/tar"
	"compress/gzip"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"
)

const selfUpdateRepo = "Martian-Engineering/pebbles"

// releaseInfo captures the fields we need from the GitHub releases API.
type releaseInfo struct {
	TagName string `json:"tag_name"`
	Body    string `json:"body"`
	HTMLURL string `json:"html_url"`
}

// selfUpdateOptions collects CLI flags for self-update.
type selfUpdateOptions struct {
	checkOnly bool
}

// updateStatus describes how the current build compares to the latest release.
type updateStatus struct {
	CurrentVersion  string
	LatestVersion   string
	ReleaseNotes    string
	ReleaseURL      string
	CurrentValid    bool
	UpdateAvailable bool
}

// semver holds a parsed vX.Y.Z version.
type semver struct {
	major int
	minor int
	patch int
}

// runSelfUpdate handles pb self-update.
func runSelfUpdate(_ string, args []string) {
	// Parse CLI flags before doing network work.
	options, err := parseSelfUpdateArgs(args)
	if err != nil {
		exitError(err)
	}
	// Fetch the latest release data from GitHub.
	release, err := fetchLatestRelease(selfUpdateRepo)
	if err != nil {
		exitError(err)
	}
	// Compare the current build version to the latest tag.
	status, err := buildUpdateStatus(buildVersion, release)
	if err != nil {
		exitError(err)
	}
	printUpdateStatus(status)
	if options.checkOnly {
		return
	}
	if !status.CurrentValid {
		exitError(fmt.Errorf("current version %q is not a release build", status.CurrentVersion))
	}
	if !status.UpdateAvailable {
		return
	}
	// Download and replace the binary when an update is available.
	if err := applySelfUpdate(release); err != nil {
		exitError(err)
	}
	fmt.Printf("Updated pb to %s\n", release.TagName)
}

// parseSelfUpdateArgs parses flags for the self-update command.
func parseSelfUpdateArgs(args []string) (selfUpdateOptions, error) {
	fs := flag.NewFlagSet("self-update", flag.ExitOnError)
	checkOnly := fs.Bool("check", false, "Check for updates without installing")
	_ = fs.Parse(args)
	if fs.NArg() != 0 {
		return selfUpdateOptions{}, fmt.Errorf("self-update takes no arguments")
	}
	return selfUpdateOptions{checkOnly: *checkOnly}, nil
}

// fetchLatestRelease loads the latest release metadata from GitHub.
func fetchLatestRelease(repo string) (releaseInfo, error) {
	client := &http.Client{Timeout: 15 * time.Second}
	url := fmt.Sprintf("https://api.github.com/repos/%s/releases/latest", repo)
	// Build the request with a stable User-Agent for GitHub API compliance.
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return releaseInfo{}, fmt.Errorf("build release request: %w", err)
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", fmt.Sprintf("pb/%s", buildVersion))
	resp, err := client.Do(req)
	if err != nil {
		return releaseInfo{}, fmt.Errorf("fetch latest release: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		message := strings.TrimSpace(string(body))
		if message == "" {
			message = resp.Status
		}
		return releaseInfo{}, fmt.Errorf("latest release request failed: %s", message)
	}
	var release releaseInfo
	// Decode the response JSON payload into the release summary.
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return releaseInfo{}, fmt.Errorf("decode latest release: %w", err)
	}
	if strings.TrimSpace(release.TagName) == "" {
		return releaseInfo{}, fmt.Errorf("latest release tag missing")
	}
	return release, nil
}

// buildUpdateStatus compares the current build version to the latest release.
func buildUpdateStatus(currentVersion string, release releaseInfo) (updateStatus, error) {
	// Always validate the latest tag so comparisons are reliable.
	latest, err := parseSemver(release.TagName)
	if err != nil {
		return updateStatus{}, fmt.Errorf("latest release tag %q invalid: %w", release.TagName, err)
	}
	status := updateStatus{
		CurrentVersion: currentVersion,
		LatestVersion:  release.TagName,
		ReleaseNotes:   strings.TrimSpace(release.Body),
		ReleaseURL:     strings.TrimSpace(release.HTMLURL),
	}
	current, err := parseSemver(currentVersion)
	if err != nil {
		status.CurrentValid = false
		return status, nil
	}
	status.CurrentValid = true
	status.UpdateAvailable = compareSemver(current, latest) < 0
	return status, nil
}

// printUpdateStatus renders the current vs latest version details.
func printUpdateStatus(status updateStatus) {
	// Summarize current and latest versions first.
	fmt.Printf("Current version: %s\n", status.CurrentVersion)
	fmt.Printf("Latest version: %s\n", status.LatestVersion)
	if !status.CurrentValid {
		fmt.Println("Current version is not a release build; cannot compare.")
	} else if status.UpdateAvailable {
		fmt.Println("Update available.")
	} else {
		fmt.Println("pb is up to date.")
	}
	fmt.Println("")
	// Show release notes for the latest release.
	fmt.Printf("Release notes for %s:\n", status.LatestVersion)
	if status.ReleaseNotes == "" {
		fmt.Println("(no release notes)")
	} else {
		fmt.Println(status.ReleaseNotes)
	}
	if status.ReleaseURL != "" {
		fmt.Printf("\nRelease: %s\n", status.ReleaseURL)
	}
}

// applySelfUpdate downloads and installs the latest pb release.
func applySelfUpdate(release releaseInfo) error {
	// Resolve platform-specific asset names that match install.sh.
	osName, arch, err := resolveReleaseTarget()
	if err != nil {
		return err
	}
	downloadURL := releaseDownloadURL(release.TagName, osName, arch)
	// Resolve the current executable path so we can replace it.
	execPath, err := resolveExecutablePath()
	if err != nil {
		return err
	}
	tmpDir, err := os.MkdirTemp("", "pb-update-")
	if err != nil {
		return fmt.Errorf("create temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)
	archivePath := filepath.Join(tmpDir, "pb.tar.gz")
	if err := downloadToFile(downloadURL, archivePath); err != nil {
		return err
	}
	targetDir := filepath.Dir(execPath)
	tmpFile, err := os.CreateTemp(targetDir, "pb-update-")
	if err != nil {
		return permissionHint(err, execPath)
	}
	tmpPath := tmpFile.Name()
	if err := tmpFile.Close(); err != nil {
		return fmt.Errorf("prepare temp binary: %w", err)
	}
	defer os.Remove(tmpPath)
	// Extract the pb binary to a temp file in the target directory.
	if err := extractBinaryFromTarGz(archivePath, tmpPath); err != nil {
		return err
	}
	if err := os.Chmod(tmpPath, 0o755); err != nil {
		return fmt.Errorf("set permissions on %s: %w", tmpPath, err)
	}
	if err := os.Rename(tmpPath, execPath); err != nil {
		return permissionHint(err, execPath)
	}
	return nil
}

// resolveReleaseTarget maps the runtime platform to release asset identifiers.
func resolveReleaseTarget() (string, string, error) {
	osName := runtime.GOOS
	if osName != "darwin" && osName != "linux" {
		return "", "", fmt.Errorf("unsupported OS: %s", osName)
	}
	arch := runtime.GOARCH
	if arch != "amd64" && arch != "arm64" {
		return "", "", fmt.Errorf("unsupported architecture: %s", arch)
	}
	return osName, arch, nil
}

// resolveExecutablePath returns the resolved path to the running binary.
func resolveExecutablePath() (string, error) {
	execPath, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("resolve executable: %w", err)
	}
	resolved, err := filepath.EvalSymlinks(execPath)
	if err != nil {
		return "", fmt.Errorf("resolve symlink: %w", err)
	}
	return resolved, nil
}

// releaseDownloadURL builds the download URL for a release asset.
func releaseDownloadURL(tag, osName, arch string) string {
	return fmt.Sprintf(
		"https://github.com/%s/releases/download/%s/pb-%s-%s.tar.gz",
		selfUpdateRepo,
		tag,
		osName,
		arch,
	)
}

// downloadToFile fetches the URL contents and writes them to a file.
func downloadToFile(url, path string) error {
	client := &http.Client{Timeout: 60 * time.Second}
	// Create the output file before starting the download.
	out, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("create %s: %w", path, err)
	}
	defer out.Close()
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("build download request: %w", err)
	}
	req.Header.Set("User-Agent", fmt.Sprintf("pb/%s", buildVersion))
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("download %s: %w", url, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download failed: %s", resp.Status)
	}
	if _, err := io.Copy(out, resp.Body); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	return nil
}

// extractBinaryFromTarGz extracts the pb binary from a tar.gz archive.
func extractBinaryFromTarGz(archivePath, targetPath string) error {
	archive, err := os.Open(archivePath)
	if err != nil {
		return fmt.Errorf("open archive: %w", err)
	}
	defer archive.Close()
	gzipReader, err := gzip.NewReader(archive)
	if err != nil {
		return fmt.Errorf("read gzip: %w", err)
	}
	defer gzipReader.Close()
	tarReader := tar.NewReader(gzipReader)
	for {
		header, err := tarReader.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return fmt.Errorf("read archive: %w", err)
		}
		// Skip directories and unrelated files.
		if header.FileInfo().IsDir() {
			continue
		}
		if filepath.Base(header.Name) != "pb" {
			continue
		}
		out, err := os.OpenFile(targetPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o755)
		if err != nil {
			return fmt.Errorf("write %s: %w", targetPath, err)
		}
		if _, err := io.Copy(out, tarReader); err != nil {
			_ = out.Close()
			return fmt.Errorf("extract pb: %w", err)
		}
		if err := out.Close(); err != nil {
			return fmt.Errorf("close %s: %w", targetPath, err)
		}
		return nil
	}
	return fmt.Errorf("pb binary not found in archive")
}

// parseSemver parses a vX.Y.Z version string into numeric parts.
func parseSemver(input string) (semver, error) {
	trimmed := strings.TrimSpace(input)
	trimmed = strings.TrimPrefix(trimmed, "v")
	parts := strings.Split(trimmed, ".")
	if len(parts) != 3 {
		return semver{}, fmt.Errorf("expected vX.Y.Z")
	}
	major, err := strconv.Atoi(parts[0])
	if err != nil {
		return semver{}, fmt.Errorf("invalid major: %w", err)
	}
	minor, err := strconv.Atoi(parts[1])
	if err != nil {
		return semver{}, fmt.Errorf("invalid minor: %w", err)
	}
	patch, err := strconv.Atoi(parts[2])
	if err != nil {
		return semver{}, fmt.Errorf("invalid patch: %w", err)
	}
	return semver{major: major, minor: minor, patch: patch}, nil
}

// compareSemver returns -1, 0, or 1 based on version ordering.
func compareSemver(a, b semver) int {
	if a.major != b.major {
		if a.major < b.major {
			return -1
		}
		return 1
	}
	if a.minor != b.minor {
		if a.minor < b.minor {
			return -1
		}
		return 1
	}
	if a.patch < b.patch {
		return -1
	}
	if a.patch > b.patch {
		return 1
	}
	return 0
}

// permissionHint wraps permission errors with sudo guidance.
func permissionHint(err error, target string) error {
	if isPermissionError(err) {
		return fmt.Errorf("permission denied updating %s; try running with sudo", target)
	}
	return fmt.Errorf("update %s: %w", target, err)
}

// isPermissionError returns true when the error indicates permission problems.
func isPermissionError(err error) bool {
	if err == nil {
		return false
	}
	if os.IsPermission(err) {
		return true
	}
	return errors.Is(err, syscall.EACCES) || errors.Is(err, syscall.EPERM)
}
