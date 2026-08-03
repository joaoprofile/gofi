package scaffold

// backendScaffolds maps a backend language to its embedded template tree.
// Language keys mirror config.Language* without importing config — the
// scaffold package stays dependency-light, and the pairing is locked by a test.
var backendScaffolds = map[string]string{
	"go":     "golang",
	"rust":   "rust",
	"java":   "java",
	"csharp": "csharp",
	"nodejs": "nodejs",
}

// HasBackendScaffold reports whether gofi can bootstrap a project skeleton for
// this language. Callers use it to say "not supported" once, up front, instead
// of discovering it halfway through the pipeline.
func HasBackendScaffold(language string) bool {
	_, ok := backendScaffolds[language]
	return ok
}

// InstallBackend writes the project skeleton for language at projectRoot: the
// build manifest, an entrypoint under <SourceRoot>/, and the empty domain/ and
// .migrations/ folders the gofi conventions expect. The directory must already
// exist.
//
// Existing files are never overwritten, so running this over a repository that
// already has code fills the gaps instead of replacing them.
func InstallBackend(language, projectRoot string, data TemplateData) ([]string, error) {
	dir, ok := backendScaffolds[language]
	if !ok {
		return nil, nil
	}
	return installFS(embeddedFS, "embedded/"+dir, projectRoot, data.WithModuleParts(), InstallOptions{KeepExisting: true})
}
