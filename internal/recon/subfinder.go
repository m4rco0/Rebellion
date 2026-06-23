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
		"-sc", "-title", "-td", "-server",
		"-cl",  "-ct",  "-location", "-rt",
		"-ip", "-cname", "-cdn",
		"-timeout",  "10", "-retries", "1",
		"-t", "50", "-rl" , "100",
		"-o", outputFile,
	)

	return err
}
