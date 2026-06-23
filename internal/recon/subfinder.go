package recon

import (
	"time"

	"github.com/m4rco0/Rebellion/internal/runner"
)


func BuscarSubdominios(hostname , outputFile string) (error) {
	_, err :=   runner.Run(
		15 * time.Minute,
		"subfinder",
		"-silent",
		"-all",
		"-d", hostname,
		"-o", outputFile,
	)

	return err
}

func RunPureDNS(inputFile string, resolversFile string, outputFile string) error {
	_, err := runner.Run(
		15*time.Minute,
		"puredns",
		"resolve",
		inputFile,
		"-r", resolversFile,
		"-w", outputFile,
	)

	return err
}

func RunHttpx(Resolved_urls string, outputFile string) error {
	_, err := runner.Run(
		10 * time.Minute,
		"httpx",
		"-l", Resolved_urls,
		"-o", outputFile,
	)

	return err
}
