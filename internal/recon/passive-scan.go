package recon

import (
	"fmt"
	"time"

	pathing "github.com/m4rco0/Rebellion/internal/pathing"
	"github.com/m4rco0/Rebellion/internal/runner"
)

func Scan_passivo(domain string,resolver string,  paths *pathing.ScanPaths) error {
	fmt.Println("==> Inicializando o scan passivo <==")
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
		return RunPureDNS(paths.SubdomainsRaw, resolver , paths.Resolved)
	})

	if err != nil {
		return err
	}
	/*err = runner.Log_scan_step("Waybackurls", func() error {
		return BuscarWaybackurls(paths.Resolved,paths.Findings)
	})
	if err != nil {
		return err
	}
	err = runner.Log_scan_step("Gau", func() error {
		return BuscarGau(paths.Resolved, paths.Findings)
	})
	*/

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
	_, err := runner.Run(
		20 * time.Minute,
		"assetfinder", "-subs-only", domain,
		"|", "anew", outputFile,
	)

	return err
}

func BuscarWaybackurls(resolvedsFile, outputFile string) error {
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
	result, err := runner.RunPipeline(
		35*time.Minute,

		[]string{"cat", resolvedsFile},

		[]string{"gau"},

		[]string{"anew", outputFile},
	)

	fmt.Println(result.Stdout)

	return err
}
func RunPureDNS(inputFile , resolversFile, outputFile string) error {
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

