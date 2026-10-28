package recon

import (
	"fmt"
	"os"
	"time"

	pathing "github.com/m4rco0/Rebellion/internal/pathing"
	"github.com/m4rco0/Rebellion/internal/runner"
)

func Scan_passivo(domain string, resolver string, paths *pathing.ScanPaths) error {
	err := runner.Log_scan_step("Subfinder", func() error {
		return BuscarSubfinder(domain, paths.SubdomainsRaw)
	})
	if err != nil {
		return err
	}

	err = runner.Log_scan_step("Assetfinder", func() error {
		return BuscarAssetfinder(domain, paths.SubdomainsRaw)
	})
	if err != nil {
		return err
	}
	err = runner.Log_scan_step("PureDNS", func() error {
		return RunPureDNS(paths.SubdomainsRaw, resolver, paths.Resolved)
	})

	if err != nil {
		return err
	}
	if _, err := os.Stat(paths.Resolved); err != nil {
		return fmt.Errorf(
			"subdomínios resolvidos não encontrados em %s: habilite as etapas de subfinder/puredns antes da coleta de URLs",
			paths.Resolved,
		)
	}

	err = runner.Log_scan_step("Waybackurls", func() error {
		return BuscarWaybackurls(paths.Resolved, paths.KatanaURLs)
	})
	if err != nil {
		return err
	}
	err = runner.Log_scan_step("Gau", func() error {
		return BuscarGau(paths.Resolved, paths.KatanaURLs)
	})

	return err
}

func BuscarSubfinder(hostname, outputFile string) error {
	fmt.Println(hostname, outputFile)
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

func BuscarAssetfinder(domain, outputFile string) error {
	// assetfinder só escreve em stdout; o anew acrescenta ao arquivo sem
	// repetir linhas. Passar "|" como argumento (como era feito antes) fazia o
	// assetfinder ignorar a saída e o arquivo nunca era atualizado.
	_, err := runner.RunPipeline(
		20*time.Minute,
		[]string{"assetfinder", "-subs-only", domain},
		[]string{"anew", outputFile},
	)

	return err
}

func BuscarWaybackurls(resolvedsFile, outputFile string) error {
	fmt.Println("\n\nEntrando no waybackurls: ", resolvedsFile, outputFile)
	result, err := runner.RunPipeline(
		35*time.Minute,

		[]string{"cat", resolvedsFile},

		[]string{"waybackurls"},

		[]string{"anew", outputFile},
	)

	fmt.Println(result.Stdout)

	return err
}

func BuscarGau(resolvedsFile, outputFile string) error {
	fmt.Println("\n\nEntrando no Gau:", resolvedsFile, outputFile)
	result, err := runner.RunPipeline(
		35*time.Minute,

		[]string{"cat", resolvedsFile},

		[]string{"gau"},

		[]string{"anew", outputFile},
	)

	fmt.Println(result.Stdout)

	return err
}
func RunPureDNS(inputFile, resolversFile, outputFile string) error {
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
