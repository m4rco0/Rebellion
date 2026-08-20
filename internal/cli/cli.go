package cli

import (
	"flag"
	"fmt"
	"os"
)

type Argumentos struct {
	ActiveScan  bool
	PassiveScan bool
	Domain      string
	ResolverDNS string
}

func ParserArgs() Argumentos {
	if len(os.Args) < 2 {
		fmt.Println("Uso: rebellion <domínio> [--active-scan]")
		os.Exit(1)
	}

	args := Argumentos{
		ActiveScan:  false,
		PassiveScan: false,
		Domain:      "",
	}
	opt_domain := os.Args[1]
	opt_scan_ativo := flag.Bool("active-scan", false, "Realiza o scan de forma ativa com o host")

	opt_scan_passivo := flag.Bool("passive-scan", false, "Realiza um scan de forma passiva")

	opt_resolved_dns := flag.String("resolver", "./resolved.txt", "Utilize para setar um resolver de DNS")
	flag.Parse()

	for i := 1; i < len(os.Args); i++ {
		if os.Args[i] == "--active-scan" {
			*opt_scan_ativo = true
		}
		if os.Args[i] == "--passive-scan" {
			*opt_scan_passivo = true
		}
	}

	if flag.NArg() < 1 {
		fmt.Println("Uso: rebellion <domínio> [--active-scan]")
		os.Exit(1)
	}

	fmt.Println(opt_domain)
	args.Domain = opt_domain
	args.ActiveScan = *opt_scan_ativo
	args.PassiveScan = *opt_scan_passivo
	args.ResolverDNS = *opt_resolved_dns

	fmt.Println("Argumentos parseados: ", args)

	return args
}
