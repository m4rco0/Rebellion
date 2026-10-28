package storage

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// Probe é a representação da saída JSON do httpx (uma linha por host).
type Probe struct {
	Input         string   `json:"input"`
	URL           string   `json:"url"`
	StatusCode    int      `json:"status_code"`
	Title         string   `json:"title"`
	Webserver     string   `json:"webserver"`
	ContentType   string   `json:"content_type"`
	ContentLength int64    `json:"content_length"`
	Location      string   `json:"location"`
	Technologies  []string `json:"tech"`
	IPs           []string `json:"a"`
	CNAMEs        []string `json:"cname"`
	CDN           string   `json:"cdn_name"`
	ResponseTime  string   `json:"time"`
	Failed        bool     `json:"failed"`
	Error         string   `json:"error"`
}

// ParseLines lê uma saída de lista (subfinder, puredns, gau) e devolve as
// linhas normalizadas, sem espaços, sem vazias, sem comentários e sem
// duplicatas, preservando a ordem original.
func ParseLines(r io.Reader) ([]string, error) {
	seen := make(map[string]struct{})
	var lines []string

	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if _, ok := seen[line]; ok {
			continue
		}
		seen[line] = struct{}{}
		lines = append(lines, line)
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("ler linhas: %w", err)
	}

	return lines, nil
}

// ParseLinesFile é ParseLines aplicado a um arquivo.
func ParseLinesFile(path string) ([]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	return ParseLines(f)
}

// ParseHttpxJSON lê a saída do httpx com -json (JSON por linha).
func ParseHttpxJSON(r io.Reader) ([]Probe, error) {
	var probes []Probe

	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	lineNumber := 0
	for scanner.Scan() {
		lineNumber++
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		var probe Probe
		if err := json.Unmarshal([]byte(line), &probe); err != nil {
			return nil, fmt.Errorf("linha %d: JSON inválido: %w", lineNumber, err)
		}
		probes = append(probes, probe)
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("ler httpx: %w", err)
	}

	return probes, nil
}

// ParseHttpxFile é ParseHttpxJSON aplicado a um arquivo.
func ParseHttpxFile(path string) ([]Probe, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	return ParseHttpxJSON(f)
}

// WriteLines grava uma lista em disco (uma entrada por linha), removendo
// duplicatas e criando o diretório pai se necessário.
func WriteLines(path string, lines []string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}

	seen := make(map[string]struct{}, len(lines))
	var builder strings.Builder
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if _, ok := seen[line]; ok {
			continue
		}
		seen[line] = struct{}{}
		builder.WriteString(line)
		builder.WriteByte('\n')
	}

	if err := os.WriteFile(path, []byte(builder.String()), 0o644); err != nil {
		return err
	}

	return nil
}
