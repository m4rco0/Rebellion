package project

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNewScan(t *testing.T) {
	tmpDir := t.TempDir()
	t.Chdir(tmpDir)

	domain := "http://example.com"
	paths, err := NewScan(domain)

	if err != nil {
		t.Fatalf("Erro ao criar ScanPaths: %v", err)
	}

	if !strings.HasPrefix(paths.Base, filepath.Join("output", "example.com")) {
		t.Errorf("Camino base esparado '%s', mas obteve '%s'", filepath.Join("output", "example.com"), paths.Base)
	}

	exceptedDirs := map[string]string{
		"SubdomainsRaw": paths.Passive,
		"Resolved":      paths.DNS,
		"Alive":         paths.HTTP,
		"HTTPDetails":   paths.HTTP,
		"KatanaURLs":    paths.URLs,
		"Report":        paths.Base,
	}

	for key, dir := range exceptedDirs {
		if !strings.HasPrefix(dir, paths.Base) {
			t.Errorf("Caminho para %s esparado '%s', mas obteve '%s'", key, paths.Base, dir)
		}
	}

	//criação dos diretorios

	expectedDirs := []string{
		paths.Passive,
		paths.DNS,
		paths.HTTP,
		paths.URLs,
		paths.Findings,
		paths.Logs,
	}

	for _, dir := range expectedDirs {
		if _, err := os.Stat(dir); os.IsNotExist(err) {
			t.Errorf("Diretório esperado '%s' não foi criado", dir)
		}

	}

	// removendo diretorios de tests
	os.RemoveAll(tmpDir)
}

func TestSanitize(t *testing.T) {
	testCases := []struct {
		input    string
		expected string
	}{
		{"http://example.com", "example.com"},
		{"https://google.com", "google.com"},
		{"   example.org   ", "example.org"},
		{"example.net", "example.net"},
		{"ftp://example.io", "example.io"},
		{"ssh://example.io", "example.io"},
		{"", ""},
	}

	for _, tc := range testCases {
		result := sanitize(tc.input)

		if result != tc.expected {
			t.Errorf("sanitize(%q) = %q; esparado %q", tc.input, result, tc.expected)
		}

	}
}
