package cli 
import (
	"flag"
	"os"
	"fmt"
)

type Argumentos struct {
	ActiveScan bool
	PassiveScan bool
	Domain string
}

func ParserArgs() Argumentos{
	args := Argumentos{
		ActiveScan: false,
		PassiveScan: false,
		Domain: "",
	}
	opt_domain := flag.Arg(0)
	opt_scan_ativo := flag.Bool("active-scan", false, "Realiza o scan de forma ativa com o host")

	opt_scan_passivo := flag.Bool("passive-scan", false, "Realiza um scan de forma passiva")
	flag.Parse()

	if flag.NArg() < 1 {
		fmt.Println("Uso: rebellion <domínio> [--active-scan]")
		os.Exit(1)
	}


	fmt.Println(opt_domain)
	args.Domain = opt_domain
	args.ActiveScan = *opt_scan_ativo
	args.PassiveScan = *opt_scan_passivo


	return args
}
