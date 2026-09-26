package workspace

import (
	"path"
	"strings"
)

// langOf names a file's language from its name, for highlighting and
// icons. YAML is the only one codec parses; the rest are for display.
func langOf(name string) string {
	lower := strings.ToLower(name)
	switch path.Ext(lower) {
	case ".yaml", ".yml":
		return "yaml"
	case ".json", ".jsonc":
		return "json"
	case ".md", ".markdown":
		return "markdown"
	case ".sh", ".bash", ".zsh":
		return "shell"
	case ".ps1", ".psm1":
		return "powershell"
	case ".py":
		return "python"
	case ".go":
		return "go"
	case ".tf", ".tfvars", ".hcl":
		return "terraform"
	case ".tpl", ".gotmpl", ".j2", ".jinja", ".jinja2":
		return "template"
	case ".toml", ".ini", ".cfg", ".conf", ".properties", ".env":
		return "config"
	case ".xml":
		return "xml"
	}
	if lower == "dockerfile" || strings.HasPrefix(lower, "dockerfile.") || strings.HasSuffix(lower, ".dockerfile") {
		return "dockerfile"
	}
	return "text"
}

// binaryExt lists extensions that are never text, so the tree can skip
// them without opening every file.
var binaryExt = map[string]bool{}

func init() {
	for _, e := range strings.Fields(`
		.png .jpg .jpeg .gif .bmp .ico .webp .tif .tiff .psd .heic
		.pdf .doc .docx .xls .xlsx .ppt .pptx .odt .ods
		.zip .gz .tgz .tar .bz2 .xz .zst .7z .rar .jar .war .ear .whl .nupkg
		.exe .dll .so .dylib .bin .o .a .lib .obj .class .pyc .pyo .wasm
		.msi .deb .rpm .dmg .iso .img .apk
		.woff .woff2 .ttf .otf .eot
		.mp3 .mp4 .mov .avi .mkv .wav .flac .ogg .webm
		.db .sqlite .sqlite3 .mdb
		.pfx .p12 .jks .keystore .der
	`) {
		binaryExt[e] = true
	}
}
