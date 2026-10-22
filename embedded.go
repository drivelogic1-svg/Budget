package budget

import (
	"embed"
	"io/fs"
	"strings"
)

// WebFS embeds the HTML/CSS/JS desktop GUI assets.
//
//go:embed web/*
var WebFS embed.FS

// DefaultXLSXFS embeds the repository's Excel workbook so the Windows .exe
// works out of the box even as a single standalone file.
//
//go:embed *.xlsx
var DefaultXLSXFS embed.FS

// ReadDefaultXLSX returns the filename and raw bytes of the embedded Excel workbook.
func ReadDefaultXLSX() (string, []byte, error) {
	entries, err := fs.ReadDir(DefaultXLSXFS, ".")
	if err != nil {
		return "", nil, err
	}
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(strings.ToLower(entry.Name()), ".xlsx") {
			data, err := DefaultXLSXFS.ReadFile(entry.Name())
			if err != nil {
				return "", nil, err
			}
			return entry.Name(), data, nil
		}
	}
	return "", nil, fs.ErrNotExist
}
