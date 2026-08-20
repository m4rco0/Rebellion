package cli

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type Argumentos struct {
	ActiveScan  bool
	PassiveScan bool
	Domain      string
	ResolverDNS string
}

func ParserArgs() Argumentos {

	opt_scan_ativo := flag.Bool("active-scan", false, "Realiza o scan de forma ativa com o host")
	opt_scan_passivo := flag.Bool("passive-scan", false, "Realiza um scan de forma passiva")
	opt_resolved_dns := flag.String("resolver", "./resolved.txt", "Caminho para o arquivo de resolvers DNS")

	flag.Parse()

	if flag.NArg() < 1 {
		fmt.Println("Uso: rebellion [opções] <domínio>")
		flag.PrintDefaults()
		os.Exit(1)
	}

	opt_domain := flag.Arg(0)

	args := Argumentos{
		Domain:      opt_domain,
		ActiveScan:  *opt_scan_ativo,
		PassiveScan: *opt_scan_passivo,
		ResolverDNS: expandirTil(*opt_resolved_dns),
	}

	fmt.Println("Argumentos parseados: ", args)

	return args
}

func expandirTil(caminho string) string {
	if strings.HasPrefix(caminho, "~/") {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return caminho
		}
		return filepath.Join(homeDir, caminho[2:])
	}
	return caminho
}
