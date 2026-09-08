package recon

import (
	"time"

	"github.com/m4rco0/Rebellion/internal/runner"
)

func RunHttpx(Resolved_urls string, outputFile string) error {

	_, err := runner.Run(
		30*time.Minute,
		"httpx",
		"-l", Resolved_urls,
		"-sc", "-title", "-td", "-server",
		"-cl", "-ct", "-location", "-rt",
		"-ip", "-cname", "-cdn",
		"-timeout", "10", "-retries", "1",
		"-t", "50", "-rl", "100",
		"-o", outputFile,
	)

	return err
}

func RunGoBuster(path_url, outputFile string) error {
	_, err := runner.Run(
		30*time.Minute,
		"gobuster", "dir",
		"-u", path_url,
		"-w", "/usr/share/seclists/Discovery/Web-Content/common.txt",
	)

	return err
}

func RunNaabu(alvos_ip, outputFile string) error {
	_, err := runner.Run(
		30*time.Minute,
		"naabu",
		"-list", alvos_ip,
		"-top-ports", "1000",
		"-silent", "-o", outputFile,
	)

	return err
}

func ScanAtivo(Resolved_urls, outputFile string) error {
	runner.Log_scan_step("Httpx", func() error {
		return RunHttpx(Resolved_urls, outputFile)
	})

	runner.Log_scan_step("Gobuster", func() error {
		return RunGoBuster(Resolved_urls, outputFile)
	})
	return nil
}
