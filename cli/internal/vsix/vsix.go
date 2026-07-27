// Package vsix packages a VSCode extension source tree into a .vsix.
//
// It is built here, in Go, rather than by `vsce package`: a .vsix is a plain
// OPC zip, and building it ourselves means `gofi install extensions` works on
// a machine with no Node toolchain at all — which is the whole point of
// shipping the extension inside the CLI binary.
//
// The package deliberately does not import the embedded artefact: the code
// generator that produces it needs the builder, so the builder must not need
// the artefact.
package vsix

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"io/fs"
	"path"
	"sort"
	"strings"
	"time"
)

// GofiAIDir is the folder holding the GOFI AI sources, relative to the repo
// root. The generator and the freshness test both resolve against it.
const GofiAIDir = "gofi-ai"

// zipEpoch is a fixed timestamp for every entry. Real modification times would
// make the artefact differ on every checkout, and the staleness test compares
// bytes.
var zipEpoch = time.Date(2020, time.January, 1, 0, 0, 0, 0, time.UTC)

// Manifest is the slice of the extension's package.json the .vsix packaging
// needs. Everything else in package.json is consumed by VSCode at runtime, not
// at install time.
type Manifest struct {
	Name        string   `json:"name"`
	DisplayName string   `json:"displayName"`
	Description string   `json:"description"`
	Version     string   `json:"version"`
	Publisher   string   `json:"publisher"`
	License     string   `json:"license"`
	Icon        string   `json:"icon"`
	Categories  []string `json:"categories"`
	Keywords    []string `json:"keywords"`
	Engines     struct {
		VSCode string `json:"vscode"`
	} `json:"engines"`
	Repository struct {
		URL string `json:"url"`
	} `json:"repository"`
}

// ID is the `publisher.name` identifier VSCode installs the extension under.
func (m Manifest) ID() string { return m.Publisher + "." + m.Name }

// VSIXName is the conventional artefact filename.
func (m Manifest) VSIXName() string {
	return fmt.Sprintf("%s-%s-%s.vsix", m.Publisher, m.Name, m.Version)
}

// excludedNames are entries never packaged, matched on any path segment. This
// is the .vscodeignore in code form: the list is short and static, and reading
// the file would mean implementing its glob dialect for no gain. Keep it in
// step with the .vscodeignore of the extensions this packages — the file is
// what a human reads, this is what actually decides.
var excludedNames = map[string]bool{
	"node_modules":  true,
	".git":          true,
	".vscode":       true,
	".vscodeignore": true,
	".gitignore":    true,
	".DS_Store":     true,
	// The extension's own test suite is for the repo, not for users.
	"test": true,
}

func excluded(name string) bool {
	if excludedNames[name] {
		return true
	}
	return strings.HasSuffix(name, ".vsix")
}

// ReadManifest parses the extension's package.json out of srcFS.
func ReadManifest(srcFS fs.FS) (Manifest, error) {
	var m Manifest
	data, err := fs.ReadFile(srcFS, "package.json")
	if err != nil {
		return m, fmt.Errorf("read package.json: %w", err)
	}
	if err := json.Unmarshal(data, &m); err != nil {
		return m, fmt.Errorf("parse package.json: %w", err)
	}
	if m.Name == "" || m.Publisher == "" || m.Version == "" {
		return m, fmt.Errorf("package.json: name, publisher and version are all required")
	}
	return m, nil
}

// Build packages the extension rooted at srcFS into .vsix bytes.
//
// The output is byte-for-byte reproducible for a given source tree, so a test
// can rebuild it and compare against the committed artefact to catch drift.
func Build(srcFS fs.FS) ([]byte, Manifest, error) {
	manifest, err := ReadManifest(srcFS)
	if err != nil {
		return nil, manifest, err
	}

	files, err := collect(srcFS)
	if err != nil {
		return nil, manifest, err
	}

	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)

	// OPC metadata first, then the payload under extension/ — the layout
	// `code --install-extension` expects.
	vsixManifest, err := renderVSIXManifest(manifest)
	if err != nil {
		return nil, manifest, err
	}
	if err := writeEntry(zw, "extension.vsixmanifest", vsixManifest); err != nil {
		return nil, manifest, err
	}
	contentTypes, err := renderContentTypes(files)
	if err != nil {
		return nil, manifest, err
	}
	if err := writeEntry(zw, "[Content_Types].xml", contentTypes); err != nil {
		return nil, manifest, err
	}

	for _, name := range files {
		data, err := fs.ReadFile(srcFS, name)
		if err != nil {
			return nil, manifest, fmt.Errorf("read %s: %w", name, err)
		}
		if err := writeEntry(zw, "extension/"+name, data); err != nil {
			return nil, manifest, err
		}
	}

	if err := zw.Close(); err != nil {
		return nil, manifest, fmt.Errorf("close zip: %w", err)
	}
	return buf.Bytes(), manifest, nil
}

// ManifestFromVSIX reads extension/package.json back out of a packaged .vsix,
// so callers report the version they are actually shipping rather than one
// recorded separately and able to drift.
func ManifestFromVSIX(data []byte) (Manifest, error) {
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return Manifest{}, fmt.Errorf("open vsix: %w", err)
	}
	for _, f := range zr.File {
		if f.Name != "extension/package.json" {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			return Manifest{}, fmt.Errorf("open manifest in vsix: %w", err)
		}
		defer rc.Close()
		body, err := io.ReadAll(rc)
		if err != nil {
			return Manifest{}, fmt.Errorf("read manifest in vsix: %w", err)
		}
		var m Manifest
		if err := json.Unmarshal(body, &m); err != nil {
			return Manifest{}, fmt.Errorf("parse manifest in vsix: %w", err)
		}
		return m, nil
	}
	return Manifest{}, fmt.Errorf("vsix has no extension/package.json")
}

// collect lists the packageable files in srcFS, sorted for reproducibility.
func collect(srcFS fs.FS) ([]string, error) {
	var files []string
	err := fs.WalkDir(srcFS, ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if p == "." {
			return nil
		}
		if excluded(path.Base(p)) {
			if d.IsDir() {
				return fs.SkipDir
			}
			return nil
		}
		if !d.IsDir() {
			files = append(files, p)
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("walk extension sources: %w", err)
	}
	sort.Strings(files)
	if len(files) == 0 {
		return nil, fmt.Errorf("extension sources are empty")
	}
	return files, nil
}

func writeEntry(zw *zip.Writer, name string, data []byte) error {
	header := &zip.FileHeader{Name: name, Method: zip.Deflate, Modified: zipEpoch}
	w, err := zw.CreateHeader(header)
	if err != nil {
		return fmt.Errorf("create %s: %w", name, err)
	}
	if _, err := w.Write(data); err != nil {
		return fmt.Errorf("write %s: %w", name, err)
	}
	return nil
}

// ── OPC metadata ────────────────────────────────────────────────────────────

type packageManifest struct {
	XMLName      xml.Name `xml:"PackageManifest"`
	Version      string   `xml:"Version,attr"`
	Xmlns        string   `xml:"xmlns,attr"`
	Metadata     pmMetadata
	Installation pmInstallation
	Dependencies string `xml:"Dependencies"`
	Assets       pmAssets
}

type pmMetadata struct {
	XMLName      xml.Name `xml:"Metadata"`
	Identity     pmIdentity
	DisplayName  string `xml:"DisplayName"`
	Description  string `xml:"Description"`
	Tags         string `xml:"Tags"`
	Categories   string `xml:"Categories"`
	GalleryFlags string `xml:"GalleryFlags"`
	Properties   pmProperties
}

type pmIdentity struct {
	XMLName   xml.Name `xml:"Identity"`
	Language  string   `xml:"Language,attr"`
	ID        string   `xml:"Id,attr"`
	Version   string   `xml:"Version,attr"`
	Publisher string   `xml:"Publisher,attr"`
}

type pmProperties struct {
	XMLName  xml.Name     `xml:"Properties"`
	Property []pmProperty `xml:"Property"`
}

type pmProperty struct {
	ID    string `xml:"Id,attr"`
	Value string `xml:"Value,attr"`
}

type pmInstallation struct {
	XMLName            xml.Name `xml:"Installation"`
	InstallationTarget struct {
		ID string `xml:"Id,attr"`
	} `xml:"InstallationTarget"`
}

type pmAssets struct {
	XMLName xml.Name  `xml:"Assets"`
	Asset   []pmAsset `xml:"Asset"`
}

type pmAsset struct {
	Type        string `xml:"Type,attr"`
	Path        string `xml:"Path,attr"`
	Addressable string `xml:"Addressable,attr"`
}

func renderVSIXManifest(m Manifest) ([]byte, error) {
	pm := packageManifest{
		Version: "2.0.0",
		Xmlns:   "http://schemas.microsoft.com/developer/vsx-schema/2011",
		Metadata: pmMetadata{
			Identity:     pmIdentity{Language: "en-US", ID: m.Name, Version: m.Version, Publisher: m.Publisher},
			DisplayName:  m.DisplayName,
			Description:  m.Description,
			Tags:         strings.Join(m.Keywords, ","),
			Categories:   strings.Join(m.Categories, ","),
			GalleryFlags: "Public",
			Properties: pmProperties{Property: []pmProperty{
				{ID: "Microsoft.VisualStudio.Code.Engine", Value: m.Engines.VSCode},
				{ID: "Microsoft.VisualStudio.Code.ExtensionDependencies", Value: ""},
				{ID: "Microsoft.VisualStudio.Code.ExtensionPack", Value: ""},
				{ID: "Microsoft.VisualStudio.Services.Links.Source", Value: m.Repository.URL},
			}},
		},
		Assets: pmAssets{Asset: []pmAsset{
			{Type: "Microsoft.VisualStudio.Code.Manifest", Path: "extension/package.json", Addressable: "true"},
		}},
	}
	pm.Installation.InstallationTarget.ID = "Microsoft.VisualStudio.Code"

	if m.Icon != "" {
		pm.Assets.Asset = append(pm.Assets.Asset, pmAsset{
			Type: "Microsoft.VisualStudio.Services.Icons.Default", Path: "extension/" + m.Icon, Addressable: "true",
		})
	}

	body, err := xml.MarshalIndent(pm, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("render vsixmanifest: %w", err)
	}
	return append([]byte(xml.Header), body...), nil
}

// contentTypeByExt maps the file types the extension actually ships. An
// unknown extension falls back to octet-stream, which VSCode accepts.
var contentTypeByExt = map[string]string{
	".json":         "application/json",
	".js":           "application/javascript",
	".css":          "text/css",
	".md":           "text/markdown",
	".txt":          "text/plain",
	".png":          "image/png",
	".svg":          "image/svg+xml",
	".vsixmanifest": "text/xml",
	".xml":          "text/xml",
}

type contentTypes struct {
	XMLName xml.Name          `xml:"Types"`
	Xmlns   string            `xml:"xmlns,attr"`
	Default []contentTypeItem `xml:"Default"`
}

type contentTypeItem struct {
	Extension   string `xml:"Extension,attr"`
	ContentType string `xml:"ContentType,attr"`
}

func renderContentTypes(files []string) ([]byte, error) {
	seen := map[string]bool{".vsixmanifest": true}
	for _, f := range files {
		if ext := path.Ext(f); ext != "" {
			seen[strings.ToLower(ext)] = true
		}
	}
	exts := make([]string, 0, len(seen))
	for ext := range seen {
		exts = append(exts, ext)
	}
	sort.Strings(exts)

	ct := contentTypes{Xmlns: "http://schemas.openxmlformats.org/package/2006/content-types"}
	for _, ext := range exts {
		mime, ok := contentTypeByExt[ext]
		if !ok {
			mime = "application/octet-stream"
		}
		ct.Default = append(ct.Default, contentTypeItem{Extension: ext, ContentType: mime})
	}

	body, err := xml.MarshalIndent(ct, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("render content types: %w", err)
	}
	return append([]byte(xml.Header), body...), nil
}
