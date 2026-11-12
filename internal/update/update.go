// Package update asks a release feed whether a newer version exists. It only
// reads: telling the user and opening the download page is the caller's job,
// and nothing here replaces the running program.
package update

import (
	"bufio"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

// Current is the version of this build. A test keeps it equal to viva.toml's.
const Current = "0.3.13"

// Feed is the GitHub release the project publishes its builds to.
const Feed = "https://api.github.com/repos/Alones-art/mottys/releases/latest"

// MirrorFeed is the copy of the newest release on the download host. Unlike
// Feed it names every file with its size and SHA-256, which is what a download
// is checked against. This fork checks GitHub only until its own mirror is
// up, so the mirror stays unused for now.
const MirrorFeed = "https://dl.motty.page/latest.json"

// ErrNoRelease means the feed has no published release, which is also what a
// private repository looks like to an anonymous request.
var ErrNoRelease = errors.New("update: no published release was found")

// Release is the newest published version.
type Release struct {
	Version string // without a leading v
	URL     string // the page to download it from
	Notes   string // the release notes as published (Markdown), possibly cut short
	// PlatformPackageChecked reports whether the answer included the feed's
	// package list for the platform that asked. A GitHub page fallback can
	// announce a release without proving whether that release has a package.
	PlatformPackageChecked bool
	// Asset is the package for the platform that asked, when the feed names one.
	// The zero value means there is none: either the feed checked and this
	// platform has none, or PlatformPackageChecked is false and it never said.
	Asset Asset
}

// Asset is one downloadable package, as the mirror describes it.
type Asset struct {
	Name   string // the file name of the versioned copy
	URL    string // https, on the host the feed came from
	SHA256 string // lower-case hex
	Size   int64
	// Signature is the base64 minisign signature over the package, naming its
	// version and file. An empty signature means the release cannot be
	// installed in place, only downloaded by hand.
	Signature string
}

// Has reports whether the release carries a package for the asking platform.
func (asset Asset) Has() bool { return asset.URL != "" }

// Signed reports whether the package can be checked against the release key.
func (asset Asset) Signed() bool { return asset.Signature != "" }

const (
	maxFeedBytes  = 1 << 20
	maxNotesBytes = 16 << 10
	// A minisign signature is a few hundred bytes; anything far past that is
	// not one, and the feed should not be allowed to make the app carry it.
	maxSignatureBytes = 4096
)

// fetchRelease reads the feed's newest release with the files it carries. The
// client decides timeouts and proxies; nil uses one with a 15 second timeout
// and the environment's proxy.
func fetchRelease(ctx context.Context, client *http.Client, feed string) (Release, []releaseAsset, error) {
	if client == nil {
		client = &http.Client{Timeout: 15 * time.Second}
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, feed, nil)
	if err != nil {
		return Release{}, nil, err
	}
	request.Header.Set("Accept", "application/vnd.github+json")
	request.Header.Set("User-Agent", "motty/"+Current)
	response, err := client.Do(request)
	if err != nil {
		return Release{}, nil, err
	}
	defer response.Body.Close()
	if response.StatusCode == http.StatusNotFound {
		return Release{}, nil, ErrNoRelease
	}
	if response.StatusCode != http.StatusOK {
		return Release{}, nil, fmt.Errorf("update: the release feed answered %s", response.Status)
	}
	var body struct {
		Tag        string         `json:"tag_name"`
		URL        string         `json:"html_url"`
		Notes      string         `json:"body"`
		Draft      bool           `json:"draft"`
		Prerelease bool           `json:"prerelease"`
		Assets     []releaseAsset `json:"assets"`
	}
	if err := json.NewDecoder(io.LimitReader(response.Body, maxFeedBytes)).Decode(&body); err != nil {
		return Release{}, nil, fmt.Errorf("update: reading the release feed: %w", err)
	}
	version := strings.TrimPrefix(strings.TrimSpace(body.Tag), "v")
	if _, ok := parse(version); !ok || body.Draft || body.Prerelease {
		return Release{}, nil, errors.New("update: the release feed has no usable version")
	}
	// The page is opened in the user's browser; only a web address is trusted.
	if !strings.HasPrefix(body.URL, "https://") {
		return Release{}, nil, errors.New("update: the release page is not an https address")
	}
	return Release{Version: version, URL: body.URL, Notes: cutUTF8(body.Notes, maxNotesBytes)}, body.Assets, nil
}

// releaseAsset is one file the release carries, as the GitHub feed describes it.
type releaseAsset struct {
	Name string `json:"name"`
	Size int64  `json:"size"`
	URL  string `json:"browser_download_url"`
}

// The two manifests a release needs before a package can install in place:
// SHA256SUMS holds one "<sha256>  <name>" line per file, SIGNATURES one
// "<name>\t<signature>" line per package.
const (
	checksumsName   = "SHA256SUMS"
	signaturesName  = "SIGNATURES"
	githubAssetHost = "github.com"
)

// Latest reads the feed. The client decides timeouts and proxies; nil uses one
// with a 15 second timeout and the environment's proxy.
func Latest(ctx context.Context, client *http.Client, feed string) (Release, error) {
	release, _, err := fetchRelease(ctx, client, feed)
	return release, err
}

// releaseName is the versioned name a GitHub release gives the package of a
// platform, or "" for one it does not build. The mirror keeps stable names, but
// a release stamps its version into every file it carries.
func releaseName(version, goos, goarch string, pkg Package) string {
	stable := StableNamePackage(goos, goarch, pkg)
	if stable == "" {
		return ""
	}
	if pkg == PackageDeb {
		arch := stable[strings.LastIndex(stable, "_")+1:]
		return "motty_" + version + "_" + arch + ".deb"
	}
	return "motty-" + version + "-" + strings.TrimPrefix(stable, "motty-")
}

// latestGitHubPackage turns the release's own assets into what the mirror would
// answer: the package for the asking platform, its checksum and its signature,
// all read from the release the feed names. The package's download address is
// built from the feed itself, never taken from the answer, so the response
// cannot send a download to another host. A release without the manifests still
// announces, it just cannot install in place.
func latestGitHubPackage(ctx context.Context, client *http.Client, feed, goos, goarch string, pkg Package) (Release, error) {
	release, assets, err := fetchRelease(ctx, client, feed)
	if err != nil {
		return Release{}, err
	}
	stable := releaseName(release.Version, goos, goarch, pkg)
	if stable == "" {
		return release, nil
	}
	var packageAsset, checksums, signatures *releaseAsset
	for index := range assets {
		switch assets[index].Name {
		case stable:
			packageAsset = &assets[index]
		case checksumsName:
			checksums = &assets[index]
		case signaturesName:
			signatures = &assets[index]
		}
	}
	if packageAsset == nil || checksums == nil || signatures == nil {
		return release, nil
	}
	if packageAsset.Size <= 0 || packageAsset.Size > maxAssetBytes {
		return release, nil
	}
	checksum, err := manifestEntry(ctx, client, checksums.URL, stable, checksumsName)
	if err != nil {
		return Release{}, err
	}
	signature, err := manifestEntry(ctx, client, signatures.URL, stable, signaturesName)
	if err != nil {
		return Release{}, err
	}
	address, err := pinGitHubAsset(feed, release.Version, stable)
	if err != nil {
		return Release{}, err
	}
	return Release{
		Version: release.Version, URL: release.URL, Notes: release.Notes,
		PlatformPackageChecked: true,
		Asset: Asset{
			Name: stable, URL: address, SHA256: checksum, Size: packageAsset.Size, Signature: signature,
		},
	}, nil
}

// pinGitHubAsset builds the download address of a release asset from the feed
// and the release tag. The feed's path is repos/<owner>/<repo>/releases/latest,
// so the answer names a file on the same repository's release, however the
// response described it.
func pinGitHubAsset(feed, version, name string) (string, error) {
	base, err := url.Parse(feed)
	if err != nil || base.Scheme != "https" || base.Host == "" {
		return "", errors.New("update: the release feed is not an https address")
	}
	parts := strings.Split(strings.Trim(base.Path, "/"), "/")
	if len(parts) < 3 || parts[0] != "repos" || parts[1] == "" || parts[2] == "" {
		return "", errors.New("update: the release feed is not a GitHub repository feed")
	}
	pinned := url.URL{
		Scheme: "https", Host: githubAssetHost,
		Path: "/" + parts[1] + "/" + parts[2] + "/releases/download/v" + version + "/" + name,
	}
	return pinned.String(), nil
}

// manifestEntry downloads one manifest of the release and returns the entry for
// name: its checksum from SHA256SUMS, or its signature from SIGNATURES. Only an
// https address is read, and the answer is bounded like the feed itself.
func manifestEntry(ctx context.Context, client *http.Client, address, name, file string) (string, error) {
	base, err := url.Parse(address)
	if err != nil || base.Scheme != "https" || base.Host == "" {
		return "", errors.New("update: the release manifest is not an https address")
	}
	if client == nil {
		client = &http.Client{Timeout: 15 * time.Second}
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, address, nil)
	if err != nil {
		return "", err
	}
	request.Header.Set("User-Agent", "motty/"+Current)
	response, err := client.Do(request)
	if err != nil {
		return "", err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return "", fmt.Errorf("update: %s answered %s", file, response.Status)
	}
	scanner := bufio.NewScanner(io.LimitReader(response.Body, maxFeedBytes))
	scanner.Buffer(make([]byte, 0, 64*1024), 4*1024)
	for scanner.Scan() {
		line := scanner.Text()
		switch file {
		case checksumsName:
			fields := strings.Fields(line)
			if len(fields) == 2 && fields[1] == name {
				if !sha256Pattern.MatchString(fields[0]) {
					return "", errors.New("update: SHA256SUMS holds a malformed checksum")
				}
				return fields[0], nil
			}
		case signaturesName:
			if entry, signature, ok := strings.Cut(line, "\t"); ok && entry == name {
				return signature, nil
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return "", fmt.Errorf("update: reading %s: %w", file, err)
	}
	return "", fmt.Errorf("update: %s does not carry %s", file, name)
}

// cutUTF8 keeps at most limit bytes of text, never splitting a character.
func cutUTF8(text string, limit int) string {
	if len(text) <= limit {
		return text
	}
	for limit > 0 && !utf8.RuneStart(text[limit]) {
		limit--
	}
	return text[:limit]
}

// Check asks the feed for the newest release with the package of the asking
// platform: the feed's own assets are the mirror, so the answer can install in
// place when the release carries its checksums and signatures.
func Check(ctx context.Context, client *http.Client, goos, goarch string) (Release, error) {
	return CheckPackage(ctx, client, goos, goarch, PackageArchive)
}

// Package identifies the installation format, not just the host operating system.
type Package uint8

const (
	PackageArchive Package = iota
	PackageDeb
	// PackageWindowsZip is the portable zip a portable copy updates with: the
	// program is replaced in place instead of running an installer.
	PackageWindowsZip
)

// CheckPackage selects the package matching the installation on this machine.
// The feed is GitHub and the release's own assets are the mirror: the package,
// its checksum and its signature all come from the release, so the answer can
// install in place.
func CheckPackage(ctx context.Context, client *http.Client, goos, goarch string, pkg Package) (Release, error) {
	return latestGitHubPackage(ctx, client, Feed, goos, goarch, pkg)
}

func checkFeeds(ctx context.Context, client *http.Client, mirrorFeed, githubFeed, goos, goarch string) (Release, error) {
	return checkFeedsPackage(ctx, client, mirrorFeed, githubFeed, goos, goarch, PackageArchive)
}

func checkFeedsPackage(ctx context.Context, client *http.Client, mirrorFeed, githubFeed, goos, goarch string, pkg Package) (Release, error) {
	release, err := latestMirrorPackage(ctx, client, mirrorFeed, goos, goarch, pkg)
	if err == nil {
		return release, nil
	}
	if fallback, fallbackErr := Latest(ctx, client, githubFeed); fallbackErr == nil || errors.Is(fallbackErr, ErrNoRelease) {
		return fallback, fallbackErr
	}
	return Release{}, err
}

// StableName is the name the mirror gives the package of a platform, or "" for
// a platform that has none: the three systems the release builds for, on the
// architectures it builds for.
func StableName(goos, goarch string) string {
	switch {
	case goos == "darwin" && (goarch == "arm64" || goarch == "amd64"):
		return "motty-darwin-" + goarch + ".zip"
	case goos == "linux" && (goarch == "arm64" || goarch == "amd64"):
		return "motty-linux-" + goarch + ".tar.gz"
	case goos == "windows" && (goarch == "arm64" || goarch == "amd64"):
		return "motty-windows-" + goarch + "-setup.exe"
	}
	return ""
}

// StableNamePackage names a DEB only for the architectures we publish.
func StableNamePackage(goos, goarch string, pkg Package) string {
	if pkg == PackageDeb {
		if goos == "linux" && (goarch == "arm64" || goarch == "amd64") {
			return "motty_" + goarch + ".deb"
		}
		return ""
	}
	if pkg == PackageWindowsZip {
		if goos == "windows" && (goarch == "arm64" || goarch == "amd64") {
			return "motty-windows-" + goarch + ".zip"
		}
		return ""
	}
	return StableName(goos, goarch)
}

const maxAssetBytes = 1 << 29

var (
	tagPattern    = regexp.MustCompile(`^v[0-9]+(\.[0-9]+){0,2}$`)
	sha256Pattern = regexp.MustCompile(`^[0-9a-f]{64}$`)
)

// LatestMirror reads the mirror's latest.json at feed. The package URL is built
// from the feed's own address and the release tag, never taken from the file,
// so the answer cannot send a download to another host.
func LatestMirror(ctx context.Context, client *http.Client, feed, goos, goarch string) (Release, error) {
	return latestMirrorPackage(ctx, client, feed, goos, goarch, PackageArchive)
}

func latestMirrorPackage(ctx context.Context, client *http.Client, feed, goos, goarch string, pkg Package) (Release, error) {
	if client == nil {
		client = &http.Client{Timeout: 15 * time.Second}
	}
	base, err := url.Parse(feed)
	if err != nil || base.Scheme != "https" || base.Host == "" {
		return Release{}, errors.New("update: the mirror feed is not an https address")
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, feed, nil)
	if err != nil {
		return Release{}, err
	}
	request.Header.Set("User-Agent", "motty/"+Current)
	response, err := client.Do(request)
	if err != nil {
		return Release{}, err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return Release{}, fmt.Errorf("update: the mirror answered %s", response.Status)
	}
	var body struct {
		Version string `json:"version"`
		Tag     string `json:"tag"`
		Source  string `json:"source"`
		Notes   string `json:"notes"`
		Files   []struct {
			Name      string `json:"name"`
			Size      int64  `json:"size"`
			SHA256    string `json:"sha256"`
			Signature string `json:"signature"`
		} `json:"files"`
	}
	if err := json.NewDecoder(io.LimitReader(response.Body, maxFeedBytes)).Decode(&body); err != nil {
		return Release{}, fmt.Errorf("update: reading the mirror feed: %w", err)
	}
	if _, ok := parse(body.Version); !ok || body.Tag != "v"+body.Version || !tagPattern.MatchString(body.Tag) {
		return Release{}, errors.New("update: the mirror feed has no usable version")
	}
	if !strings.HasPrefix(body.Source, "https://") {
		return Release{}, errors.New("update: the release page is not an https address")
	}
	release := Release{
		Version: body.Version, URL: body.Source, Notes: cutUTF8(body.Notes, maxNotesBytes),
		PlatformPackageChecked: true,
	}
	stable := StableNamePackage(goos, goarch, pkg)
	for _, file := range body.Files {
		if stable == "" || file.Name != stable {
			continue
		}
		if !sha256Pattern.MatchString(file.SHA256) || file.Size <= 0 || file.Size > maxAssetBytes {
			// The feed names this platform's file but nothing that could hold a
			// download to it. The list is there, the answer is not: calling the
			// release package-less would be the same wrong claim a bare page
			// fallback makes.
			release.PlatformPackageChecked = false
			break
		}
		name := strings.Replace(stable, "motty-", "motty-"+body.Version+"-", 1)
		if pkg == PackageDeb {
			name = "motty_" + body.Version + "_" + goarch + ".deb"
		}
		pinned := url.URL{Scheme: "https", Host: base.Host, Path: "/releases/" + body.Tag + "/" + name}
		release.Asset = Asset{Name: name, URL: pinned.String(), SHA256: file.SHA256, Size: file.Size, Signature: validSignature(file.Signature)}
		break
	}
	return release, nil
}

// validSignature keeps a signature the app can use and drops anything that is
// not a base64 string of a sane length: a broken one is the same as none, and
// the download page still works.
func validSignature(signature string) string {
	if signature == "" || len(signature) > maxSignatureBytes {
		return ""
	}
	if _, err := base64.StdEncoding.DecodeString(signature); err != nil {
		return ""
	}
	return signature
}

// Newer reports whether latest is a later version than current. Either that is
// not a plain dotted number is "no".
func Newer(current, latest string) bool {
	a, okA := parse(strings.TrimPrefix(current, "v"))
	b, okB := parse(strings.TrimPrefix(latest, "v"))
	if !okA || !okB {
		return false
	}
	for index := range a {
		if a[index] != b[index] {
			return b[index] > a[index]
		}
	}
	return false
}

func parse(version string) (parts [3]int, ok bool) {
	fields := strings.Split(version, ".")
	if len(fields) == 0 || len(fields) > 3 {
		return parts, false
	}
	for index, field := range fields {
		number, err := strconv.Atoi(field)
		if err != nil || number < 0 || number > 1<<20 {
			return parts, false
		}
		parts[index] = number
	}
	return parts, true
}
