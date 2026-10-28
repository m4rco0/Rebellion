package storage

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseLines(t *testing.T) {
	input := " example.com \n\n# comentário\napi.example.com\r\nexample.com\n  \n"

	lines, err := ParseLines(strings.NewReader(input))
	if err != nil {
		t.Fatalf("ParseLines retornou erro: %v", err)
	}

	expected := []string{"example.com", "api.example.com"}
	if len(lines) != len(expected) {
		t.Fatalf("esperado %d linhas, obteve %d: %v", len(expected), len(lines), lines)
	}
	for i, line := range expected {
		if lines[i] != line {
			t.Errorf("linha %d: esperado %q, obteve %q", i, line, lines[i])
		}
	}
}

func TestParseHttpxJSON(t *testing.T) {
	input := strings.Join([]string{
		`{"input":"https://example.com","url":"https://example.com/","status_code":200,"title":"Home","webserver":"nginx","tech":["nginx"],"a":["1.2.3.4"],"cname":["cdn.example.net"],"cdn_name":"cloudflare","content_length":123}`,
		"",
		`{"input":"https://api.example.com","url":"https://api.example.com/v1","status_code":404,"failed":true,"error":"timeout"}`,
	}, "\n")

	probes, err := ParseHttpxJSON(strings.NewReader(input))
	if err != nil {
		t.Fatalf("ParseHttpxJSON retornou erro: %v", err)
	}
	if len(probes) != 2 {
		t.Fatalf("esperado 2 probes, obteve %d", len(probes))
	}

	first := probes[0]
	if first.StatusCode != 200 || first.Title != "Home" || first.Webserver != "nginx" {
		t.Errorf("probe inesperado: %+v", first)
	}
	if len(first.Technologies) != 1 || first.Technologies[0] != "nginx" {
		t.Errorf("tecnologias inesperadas: %v", first.Technologies)
	}
	if len(first.IPs) != 1 || first.IPs[0] != "1.2.3.4" {
		t.Errorf("IPs inesperados: %v", first.IPs)
	}

	second := probes[1]
	if !second.Failed || second.Error != "timeout" || second.StatusCode != 404 {
		t.Errorf("probe inesperado: %+v", second)
	}
}

func TestParseHttpxJSONInvalid(t *testing.T) {
	_, err := ParseHttpxJSON(strings.NewReader("{not-json}"))
	if err == nil {
		t.Fatal("esperado erro para JSON inválido")
	}
	if !strings.Contains(err.Error(), "linha 1") {
		t.Errorf("erro deveria indicar a linha: %v", err)
	}
}

func TestWriteLines(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "urls.txt")

	if err := WriteLines(path, []string{"https://a.example.com", "https://a.example.com", " ", "https://b.example.com"}); err != nil {
		t.Fatalf("WriteLines retornou erro: %v", err)
	}

	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ler arquivo: %v", err)
	}

	expected := "https://a.example.com\nhttps://b.example.com\n"
	if string(content) != expected {
		t.Errorf("conteúdo esperado %q, obteve %q", expected, string(content))
	}
}

func TestParseLinesFileNotFound(t *testing.T) {
	_, err := ParseLinesFile(filepath.Join(t.TempDir(), "nao-existe.txt"))
	if err == nil {
		t.Fatal("esperado erro para arquivo inexistente")
	}
	if !os.IsNotExist(err) {
		t.Errorf("erro deveria ser IsNotExist: %v", err)
	}
}
