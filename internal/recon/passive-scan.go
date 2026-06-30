package recon

import (
	"fmt"
	"time"

	pathing "github.com/m4rco0/Rebellion/internal/pathing"
	"github.com/m4rco0/Rebellion/internal/runner"
)

func Scan_passivo(domain string, paths *pathing.ScanPaths) error {
	fmt.Println("==> Inicializando o scan passivo <==")
	err := runner.Log_scan_step("Subfinder", func() error {
		return BuscarSubdominios(domain, paths.SubdomainsRaw)
	})

	err = runner.Log_scan_step("PureDNS", func() error {
		return RunPureDNS(paths.SubdomainsRaw, "resolvers.txt", paths.Resolved)
	})

	return err
}

func BuscarSubdominios(hostname, outputFile string) error {
	_, err := runner.Run(
		15*time.Minute,
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
