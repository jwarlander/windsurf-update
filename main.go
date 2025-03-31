package main

import (
	"archive/tar"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	"github.com/schollz/progressbar/v3"
)

const (
	UpdateApiUrl        = "https://windsurf-stable.codeium.com/api/update/%s/stable/latest"
	DefaultDownloadPath = "~/Downloads"
	DefaultInstallPath  = "~/apps/windsurf"
	VersionMarker       = ".windsurf-release"
)

type ReleaseInfo struct {
	URL                string
	Name               string
	Version            string
	ProductVersion     string
	Hash               string
	Timestamp          int64
	SHA256Hash         string
	SupportsFastUpdate bool
	WindsurfVersion    string
}

// Map GOOS-GOARCH to Windsurf update API platform names:
var platformMap = map[string]string{
	"darwin-arm64":  "darwin-arm64-dmg",
	"darwin-amd64":  "darwin-x64-dmg",
	"linux-amd64":   "linux-x64",
	"windows-amd64": "win32-x64",
}

func main() {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error getting home directory: %v\n", err)
		os.Exit(1)
	}

	// Determine current & supported platforms
	goPlatform := fmt.Sprintf("%s-%s", runtime.GOOS, runtime.GOARCH)
	updatePlatform, platformSupported := platformMap[goPlatform]

	supportedPlatforms := make([]string, 0, len(platformMap))
	for platform := range platformMap {
		supportedPlatforms = append(supportedPlatforms, platform)
	}

	downloadPath := flag.String(
		"download-path",
		strings.Replace(DefaultDownloadPath, "~", homeDir, 1),
		fmt.Sprintf("Where to download the archive (default: %s)", DefaultDownloadPath),
	)
	installPath := flag.String(
		"install-path",
		strings.Replace(DefaultInstallPath, "~", homeDir, 1),
		fmt.Sprintf("Where to install Windsurf (default: %s)", DefaultInstallPath),
	)
	platform := flag.String(
		"platform",
		"",
		fmt.Sprintf(
			"Platform to download for (current: %s, supported: [%s])",
			updatePlatform,
			strings.Join(supportedPlatforms, ", "),
		),
	)
	forceUpdate := flag.Bool("force-update", false, "Force update even if already up to date")
	yes := flag.Bool("yes", false, "Assume yes to all prompts")
	flag.Parse()

	versionMarker := filepath.Join(*installPath, VersionMarker)

	// Validate platform choice
	if *platform != "" {
		updatePlatform, platformSupported = platformMap[*platform]
	}
	if !platformSupported {
		fmt.Fprintln(os.Stderr, "Platform is not supported, use --platform to specify a supported option:")
		for _, supportedPlatform := range supportedPlatforms {
			fmt.Fprintf(os.Stderr, "  %s", supportedPlatform)
		}
		os.Exit(1)
	}
	*platform = updatePlatform
	fmt.Printf("Selected platform: %s\n", *platform)

	// Get latest version info
	release, err := getLatestRelease(*platform)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error checking for updates: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("Found Windsurf version: %s\n", release.WindsurfVersion)

	// Compare with currently installed version, if available
	if _, err := os.Stat(*installPath); !os.IsNotExist(err) && !*forceUpdate {
		vsn, err := os.ReadFile(versionMarker)
		if err != nil {
			fmt.Printf("Unable to check installed version: %v", err)
		} else {
			existing := strings.TrimSpace(string(vsn))
			latest := release.WindsurfVersion
			if compareVersions(existing, latest) >= 0 {
				fmt.Printf("Already at %s, no need to upgrade!\n", existing)
				os.Exit(0)
			}
		}
	}

	// Abort if download directory doesn't exist
	if _, err := os.Stat(*downloadPath); os.IsNotExist(err) {
		fmt.Fprintf(os.Stderr, "Download directory %s does not exist\n", *downloadPath)
		os.Exit(1)
	}

	// Download the archive
	archivePath := filepath.Join(*downloadPath, fmt.Sprintf("windsurf-%s.tar.gz", release.WindsurfVersion))
	if _, err := os.Stat(archivePath); os.IsNotExist(err) {
		fmt.Printf("Downloading %s to %s:\n", release.WindsurfVersion, *downloadPath)
		if err := downloadFile(release.URL, archivePath); err != nil {
			fmt.Fprintf(os.Stderr, "Error downloading update: %v\n", err)
			os.Exit(1)
		}
	} else {
		fmt.Printf("Download skipped (file already exists)\n")
	}

	// Check archive integrity
	fmt.Printf("Checking integrity of %s: ", archivePath)
	archiveHash, err := calculateSHA256(archivePath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error calculating SHA256: %v\n", err)
		os.Exit(1)
	}
	if archiveHash != release.SHA256Hash {
		fmt.Fprintf(os.Stderr, "SHA256 mismatch: expected %s, got %s\n", release.SHA256Hash, archiveHash)
		os.Exit(1)
	}
	fmt.Println("OK")

	// Clean up existing installation & (re)create directory
	if _, err := os.Stat(*installPath); !os.IsNotExist(err) {
		if *yes {
			fmt.Printf("Removing existing directory %s\n", *installPath)
		} else {
			fmt.Printf("Removing existing directory %s, are you sure? [y/N] ", *installPath)
			var response string
			fmt.Scanln(&response)
			if len(response) < 1 || (response[0] != 'Y' && response[0] != 'y') {
				fmt.Printf("Installation was aborted\n")
				os.Exit(1)
			}
		}
		if err := os.RemoveAll(*installPath); err != nil {
			fmt.Fprintf(os.Stderr, "Error removing existing directory %s: %v\n", *installPath, err)
			os.Exit(1)
		}
	}
	os.MkdirAll(*installPath, 0755)

	// Extract the archive
	if strings.HasPrefix(*platform, "linux-") {
		if err := extractArchive(archivePath, *installPath); err != nil {
			fmt.Fprintf(os.Stderr, "Error extracting update: %v\n", err)
			os.Exit(1)
		}
		f, err := os.Create(versionMarker)
		if err != nil {
			fmt.Printf("WARNING: Unable to create %s: %v", versionMarker, err)
		}
		f.WriteString(release.WindsurfVersion)
		f.Close()
		fmt.Printf("Successfully updated Windsurf to version %s\n", release.WindsurfVersion)
		fmt.Println("Restart Windsurf to apply the update.")
	} else {
		fmt.Printf("Install your update manually, using the downloaded archive at %s\n", archivePath)
	}
}

func getLatestRelease(platform string) (*ReleaseInfo, error) {
	resp, err := http.Get(fmt.Sprintf(UpdateApiUrl, platform))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API returned status %d", resp.StatusCode)
	}

	var release ReleaseInfo
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return nil, err
	}

	return &release, nil
}

func downloadFile(url, filepath string) error {
	// Check if partial file exists and get its size
	partialPath := filepath + ".partial"
	var startOffset int64 = 0

	if stat, err := os.Stat(partialPath); err == nil {
		startOffset = stat.Size()
		fmt.Printf("Resuming download from offset %d bytes\n", startOffset)
	}

	// Create the request with proper headers for resuming if needed
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return fmt.Errorf("unable to create http request: %w", err)
	}

	// Set Range header if we're resuming a download
	if startOffset > 0 {
		req.Header.Set("Range", fmt.Sprintf("bytes=%d-", startOffset))
	}

	// Execute the request
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("unable to make http request: %w", err)
	}
	defer resp.Body.Close()

	// Check for appropriate status code
	if startOffset > 0 && resp.StatusCode != http.StatusPartialContent {
		// If server doesn't support range requests, start over
		startOffset = 0
		req.Header.Del("Range")
		resp.Body.Close()

		// Delete the partial file and start fresh
		os.Remove(partialPath)

		resp, err = http.DefaultClient.Do(req)
		if err != nil {
			return fmt.Errorf("unable to make http request: %w", err)
		}
		defer resp.Body.Close()
	}

	// Check final status code
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusPartialContent {
		return fmt.Errorf("http request returned status %d", resp.StatusCode)
	}

	// Open file in append mode if resuming, otherwise create new
	var out *os.File
	if startOffset > 0 {
		out, err = os.OpenFile(partialPath, os.O_APPEND|os.O_WRONLY, 0644)
	} else {
		out, err = os.Create(partialPath)
	}

	if err != nil {
		return fmt.Errorf("unable to create/open temporary file: %w", err)
	}
	defer out.Close()

	// Set up progress bar with appropriate content length
	contentLength := resp.ContentLength
	if contentLength > 0 {
		contentLength += startOffset
	}

	bar := progressbar.DefaultBytes(
		contentLength,
		"Downloading",
	)

	// Set progress bar's initial value if resuming
	if startOffset > 0 {
		bar.Set64(startOffset)
	}

	_, err = io.Copy(io.MultiWriter(out, bar), resp.Body)
	if err != nil {
		return fmt.Errorf("unable to copy to temporary file: %w", err)
	}

	err = os.Rename(partialPath, filepath)
	if err != nil {
		return fmt.Errorf("unable to rename temporary file: %w", err)
	}

	return nil
}

func calculateSHA256(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("failed to open file for SHA256 calculation: %w", err)
	}
	defer file.Close()

	hash := sha256.New()

	// Create a buffer for reading the file in chunks
	buf := make([]byte, 1024*1024) // 1MB buffer

	// Read and hash the file in chunks
	for {
		n, err := file.Read(buf)
		if n > 0 {
			hash.Write(buf[:n])
		}

		if err == io.EOF {
			break
		}

		if err != nil {
			return "", fmt.Errorf("error reading file for SHA256 calculation: %w", err)
		}
	}
	// Get the hash as a hex string
	hashSum := hash.Sum(nil)
	hashString := hex.EncodeToString(hashSum)

	return hashString, nil
}

func extractArchive(archivePath, destPath string) error {
	fmt.Printf("Extracting %s to %s\n", archivePath, destPath)

	file, err := os.Open(archivePath)
	if err != nil {
		return err
	}
	defer file.Close()

	gzr, err := gzip.NewReader(file)
	if err != nil {
		return err
	}
	defer gzr.Close()

	tr := tar.NewReader(gzr)

	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		// Avoid absolute paths and directory traversal outside the destination directory
		if !filepath.IsLocal(header.Name) {
			return fmt.Errorf("non-local path detected in tar file: %s", header.Name)
		}

		// Remove first directory component to allow for a fully custom install path
		header.Name = strings.TrimPrefix(header.Name, "Windsurf/")
		if header.Name == "" {
			continue
		}

		target := filepath.Join(destPath, header.Name)

		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0755); err != nil {
				return err
			}
		case tar.TypeReg:
			f, err := os.OpenFile(target, os.O_CREATE|os.O_RDWR, os.FileMode(header.Mode))
			if err != nil {
				return err
			}

			if _, err := io.Copy(f, tr); err != nil {
				f.Close()
				return err
			}
			f.Close()
		}
	}

	return nil
}

func compareVersions(v1, v2 string) int {
	// Split version strings into components
	v1Parts := strings.Split(v1, ".")
	v2Parts := strings.Split(v2, ".")

	// Compare each component
	maxLen := max(len(v1Parts), len(v2Parts))
	for i := 0; i < maxLen; i++ {
		// If v1 is shorter than v2, pad with "0"
		v1Val := 0
		if i < len(v1Parts) {
			v1Val, _ = strconv.Atoi(v1Parts[i])
		}

		// If v2 is shorter than v1, pad with "0"
		v2Val := 0
		if i < len(v2Parts) {
			v2Val, _ = strconv.Atoi(v2Parts[i])
		}

		// Compare the numeric values
		if v1Val < v2Val {
			return -1
		} else if v1Val > v2Val {
			return 1
		}
	}

	// Versions are equal
	return 0
}
