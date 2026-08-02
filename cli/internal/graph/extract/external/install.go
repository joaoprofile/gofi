package external

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

// MaxExtractorBytes caps what an install will read. An extractor is a compiled
// binary, not an archive of the world; a stream past this is a mistake or an
// attack, and either way should stop before it fills the disk.
const MaxExtractorBytes = 512 << 20

// InstallOptions describes one extractor installation.
type InstallOptions struct {
	ProjectRoot string       // where .gofi/graph/extractors/ lives
	Language    string       // "java", "rust", ...
	From        string       // local file path, or an https:// URL
	SHA256      string       // expected digest in hex; empty skips the check
	Client      *http.Client // nil uses http.DefaultClient
}

// Installation is where an extractor landed and what it hashes to.
type Installation struct {
	Path   string
	SHA256 string
	Bytes  int64
}

// Install places an extractor under the project's extractors directory.
//
// The file is streamed to a temp name and renamed only after the digest checks
// out, so an interrupted or tampered download can never leave a half-written
// binary that gofi would then execute.
func Install(ctx context.Context, opt InstallOptions) (*Installation, error) {
	lang, err := validLanguage(opt.Language)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(opt.From) == "" {
		return nil, errors.New("informe a origem do extractor com --from <caminho|url>")
	}

	src, err := openSource(ctx, opt)
	if err != nil {
		return nil, err
	}
	defer src.Close()

	dir := filepath.Join(opt.ProjectRoot, ExtractorsDir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	root, err := os.OpenRoot(dir)
	if err != nil {
		return nil, err
	}
	defer root.Close()

	name := BinaryName(lang)
	tmp := name + ".part"
	f, err := root.OpenFile(tmp, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o755)
	if err != nil {
		return nil, err
	}
	sum := sha256.New()
	n, copyErr := io.Copy(io.MultiWriter(f, sum), io.LimitReader(src, MaxExtractorBytes+1))
	closeErr := f.Close()
	if err := errors.Join(copyErr, closeErr); err != nil {
		_ = root.Remove(tmp)
		return nil, err
	}
	if n > MaxExtractorBytes {
		_ = root.Remove(tmp)
		return nil, fmt.Errorf("o extractor passou de %d bytes", int64(MaxExtractorBytes))
	}
	if n == 0 {
		_ = root.Remove(tmp)
		return nil, fmt.Errorf("a origem %s esta vazia", opt.From)
	}

	got := hex.EncodeToString(sum.Sum(nil))
	if want := strings.TrimSpace(opt.SHA256); want != "" && !strings.EqualFold(want, got) {
		_ = root.Remove(tmp)
		return nil, fmt.Errorf("sha256 nao confere: esperado %s, veio %s", want, got)
	}
	if err := root.Rename(tmp, name); err != nil {
		_ = root.Remove(tmp)
		return nil, err
	}
	// A copy preserves nothing about the source's mode, and a downloaded file
	// has none, so the executable bit is set explicitly.
	path := filepath.Join(dir, name)
	if err := os.Chmod(path, 0o755); err != nil {
		return nil, err
	}
	return &Installation{Path: path, SHA256: got, Bytes: n}, nil
}

func openSource(ctx context.Context, opt InstallOptions) (io.ReadCloser, error) {
	from := opt.From
	if strings.HasPrefix(from, "http://") {
		return nil, fmt.Errorf("origem insegura %q — use https", from)
	}
	if !strings.HasPrefix(from, "https://") {
		return os.Open(from)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, from, nil)
	if err != nil {
		return nil, err
	}
	client := opt.Client
	if client == nil {
		client = http.DefaultClient
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("nao foi possivel baixar %s: %w", from, err)
	}
	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return nil, fmt.Errorf("%s respondeu %s", from, resp.Status)
	}
	return resp.Body, nil
}

// validLanguage keeps the name to characters that can only ever name a file, so
// a language taken from the command line cannot steer the write elsewhere.
func validLanguage(language string) (string, error) {
	lang := strings.ToLower(strings.TrimSpace(language))
	if lang == "" {
		return "", errors.New("informe a linguagem do extractor")
	}
	for _, r := range lang {
		ok := r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r == '-' || r == '+'
		if !ok {
			return "", fmt.Errorf("linguagem invalida %q — use letras, digitos, '-' ou '+'", language)
		}
	}
	return lang, nil
}

// Uninstall removes the project's copy of an extractor. An extractor found on
// PATH is not gofi's to delete, so it is left alone.
func Uninstall(projectRoot, language string) error {
	lang, err := validLanguage(language)
	if err != nil {
		return err
	}
	path := filepath.Join(projectRoot, ExtractorsDir, BinaryName(lang))
	if err := os.Remove(path); err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("nenhum extractor de %s instalado neste projeto", lang)
		}
		return err
	}
	return nil
}
