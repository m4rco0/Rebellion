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

func ScanAtivo(Resolved_urls string, outputFile string) error {
	runner.Log_scan_step("Httpx", func() error {
		return RunHttpx(Resolved_urls, outputFile)
	})

	return nil
}
