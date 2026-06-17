// Package dotenv loads environment variables from a .env file, mirroring the
// shell idiom `set -a; source .env; set +a`. Variables already present in the
// process environment are never overwritten, so explicit shell exports always
// win over the file.
package dotenv

import (
	"bufio"
	"os"
	"strings"
)

// Load reads the .env file at path and exports each KEY=VALUE pair into the
// process environment, skipping any key that is already set. A missing file is
// not an error (it returns nil). Returns the names of the keys it set.
//
// Supported syntax (a practical subset of the shell/dotenv conventions):
//   - blank lines and lines starting with '#' are ignored
//   - an optional leading "export " is stripped
//   - values may be wrapped in single or double quotes
//   - inline comments after an unquoted value (" # ...") are stripped
func Load(path string) ([]string, error) {
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	defer f.Close()

	var set []string
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		key, value, ok := parseLine(scanner.Text())
		if !ok {
			continue
		}
		if _, exists := os.LookupEnv(key); exists {
			continue
		}
		if err := os.Setenv(key, value); err != nil {
			return set, err
		}
		set = append(set, key)
	}
	if err := scanner.Err(); err != nil {
		return set, err
	}
	return set, nil
}

// parseLine extracts a key/value pair from a single .env line. The bool is
// false for blank lines, comments, and malformed entries.
func parseLine(line string) (key, value string, ok bool) {
	line = strings.TrimSpace(line)
	if line == "" || strings.HasPrefix(line, "#") {
		return "", "", false
	}
	line = strings.TrimPrefix(line, "export ")

	eq := strings.IndexByte(line, '=')
	if eq <= 0 {
		return "", "", false
	}
	key = strings.TrimSpace(line[:eq])
	if key == "" {
		return "", "", false
	}
	value = strings.TrimSpace(line[eq+1:])
	value = unquote(value)
	return key, value, true
}

// unquote strips matching surrounding quotes, or trims an inline comment from
// an unquoted value.
func unquote(value string) string {
	if len(value) >= 2 {
		switch value[0] {
		case '"', '\'':
			if value[len(value)-1] == value[0] {
				return value[1 : len(value)-1]
			}
		}
	}
	if i := strings.Index(value, " #"); i >= 0 {
		value = strings.TrimSpace(value[:i])
	}
	return value
}
