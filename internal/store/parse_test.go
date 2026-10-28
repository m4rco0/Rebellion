package store

import (
	"testing"
)

func TestNormalizeDomain(t *testing.T) {
	cases := map[string]string{
		"https://Example.com/caminho": "example.com",
		"http://example.com/":         "example.com",
		"  EXAMPLE.COM.  ":            "example.com",
		"example.com":                 "example.com",
		"":                            "",
	}

	for input, expected := range cases {
		if got := NormalizeDomain(input); got != expected {
			t.Errorf("NormalizeDomain(%q) = %q; esperado %q", input, got, expected)
		}
	}
}

func TestInScope(t *testing.T) {
	cases := []struct {
		host     string
		domain   string
		expected bool
	}{
		{"example.com", "example.com", true},
		{"www.example.com", "example.com", true},
		{"a.b.example.com", "example.com", true},
		{"WWW.EXAMPLE.COM.", "example.com", true},
		{"notexample.com", "example.com", false},
		{"example.com.evil.com", "example.com", false},
		{"", "example.com", false},
	}

	for _, tc := range cases {
		if got := inScope(tc.host, tc.domain); got != tc.expected {
			t.Errorf("inScope(%q, %q) = %v; esperado %v", tc.host, tc.domain, got, tc.expected)
		}
	}
}

func TestParseURL(t *testing.T) {
	parsed, ok := parseURL("https://WWW.Example.com:8443/a/b?x=1", "example.com")
	if !ok {
		t.Fatal("URL deveria ser aceita")
	}
	if parsed.Host != "www.example.com" || parsed.Scheme != "https" || parsed.Path != "/a/b" || parsed.Query != "x=1" {
		t.Errorf("URL parseada incorretamente: %+v", parsed)
	}
	if parsed.Hash == "" {
		t.Error("hash não deveria ser vazio")
	}

	if _, ok := parseURL("https://evil.com/a", "example.com"); ok {
		t.Error("URL fora do escopo deveria ser rejeitada")
	}
	if _, ok := parseURL("ftp://www.example.com/a", "example.com"); ok {
		t.Error("esquema diferente de http/https deveria ser rejeitado")
	}
	if _, ok := parseURL("não é uma url", "example.com"); ok {
		t.Error("URL inválida deveria ser rejeitada")
	}
}

func TestParseURLsDeduplicates(t *testing.T) {
	urls := []string{
		"https://www.example.com/a",
		"https://www.example.com/a",
		"https://api.example.com/b",
		"https://evil.com/c",
	}

	if got := len(parseURLs(urls, "example.com")); got != 2 {
		t.Errorf("esperado 2 URLs únicas, obteve %d", got)
	}
}

func TestNormalizeIPs(t *testing.T) {
	got := normalizeIPs([]string{"1.2.3.4", " 1.2.3.4 ", "inválido", "::1", "2001:0db8::1"})
	expected := []string{"1.2.3.4", "::1", "2001:db8::1"}

	if len(got) != len(expected) {
		t.Fatalf("esperado %d IPs, obteve %d: %v", len(expected), len(got), got)
	}
	for i := range expected {
		if got[i] != expected[i] {
			t.Errorf("IP %d: esperado %q, obteve %q", i, expected[i], got[i])
		}
	}
}
