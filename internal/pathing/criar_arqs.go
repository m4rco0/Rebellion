package project

import (
	"os"
	"path/filepath"
	"strings"
	"time"
)

type ScanPaths struct {
	Base          string
	Passive       string
	DNS           string
	HTTP          string
	URLs          string
	Findings      string
	Logs          string
	SubdomainsRaw string
	Resolved      string
	Alive         string
	HTTPDetails   string
	KatanaURLs    string
	Report        string
}

func NewScan(domain string) (ScanPaths, error) {
	cleanDomain := sanitize(domain)

	timestamp := time.Now().Format("2006-01-02_15-04-05")

	base := filepath.Join("output", cleanDomain, timestamp)

	paths := ScanPaths{
		Base:     base,
		Passive:  filepath.Join(base, "01_passive"),
		DNS:      filepath.Join(base, "02_dns"),
		HTTP:     filepath.Join(base, "03_http"),
		URLs:     filepath.Join(base, "04_urls"),
		Findings: filepath.Join(base, "05_findings"),
		Logs:     filepath.Join(base, "logs"),
	}

	paths.SubdomainsRaw = filepath.Join(paths.Passive, "subdomains_raw.txt")
	paths.Resolved = filepath.Join(paths.DNS, "resolved.txt")
	paths.Alive = filepath.Join(paths.HTTP, "alive.txt")
	paths.HTTPDetails = filepath.Join(paths.HTTP, "httpx_details.txt")
	paths.KatanaURLs = filepath.Join(paths.URLs, "urls.txt")
	paths.Report = filepath.Join(base, "report.md")

	dirs := []string{
		paths.Passive,
		paths.DNS,
		paths.HTTP,
		paths.URLs,
		paths.Findings,
		paths.Logs,
	}

	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return paths, err
		}
	}

	return paths, nil
}

func sanitize(domain string) string {
	domain = strings.TrimSpace(domain)
	domain = strings.TrimPrefix(domain, "https://")
	domain = strings.TrimPrefix(domain, "http://")
	domain = strings.TrimPrefix(domain, "ftp://")
	domain = strings.TrimPrefix(domain, "ssh://")
	domain = strings.TrimSuffix(domain, "/")

	replacer := strings.NewReplacer(
		"/", "_",
		"\\", "_",
		":", "_",
		"*", "_",
		"?", "_",
		"\"", "_",
		"<", "_",
		">", "_",
		"|", "_",
	)

	return replacer.Replace(domain)
}
