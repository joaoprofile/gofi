package scaffold

// InstallRust installs the Rust workspace scaffold (Cargo.toml, crates/<name>/, etc.)
// at projectRoot. The directory must already exist.
func InstallRust(projectRoot string, data TemplateData) ([]string, error) {
	return installEmbedded("rust", projectRoot, data)
}
