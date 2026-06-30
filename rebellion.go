package main

import (
	"flag"
	"fmt"
	"os"

	pathing "github.com/m4rco0/Rebellion/internal/pathing"
	"github.com/m4rco0/Rebellion/internal/recon"
)

func main() {
	activeScan := flag.Bool("active-scan", false, "Realiza o scan de forma ativa com o host")
	flag.Parse()

	if flag.NArg() < 1 {
		fmt.Println("Uso: rebellion <domínio> [--active-scan]")
		os.Exit(1)
	}

	domain := flag.Arg(0)

	fmt.Println("================================")
	fmt.Println("        Rebellion Scanner       ")
	fmt.Println("================================")
	fmt.Println("[+] Alvo:", domain)

	fmt.Println(flag.Arg(1))

	paths, err := pathing.NewScan(domain)
	if err != nil {
		fmt.Println("[!] Erro criando estrutura do projeto:", err)
		return
	}

	fmt.Println("[+] Pasta do scan:", paths.Base)
	err = recon.Scan_passivo(domain, &paths)
	if err != nil {
		fmt.Println("[!] Erro no scan passivo:", err)
		return
	}
	if *activeScan {
		err := recon.ScanAtivo(paths.Resolved, paths.Alive)
		if err != nil {
			fmt.Println("[!] Erro no scan ativo:", err)
			return
		}
	}

	fmt.Println()
	fmt.Println("[+] Scan finalizado")
	fmt.Println("[+] Subdomínios brutos:", paths.SubdomainsRaw)
	fmt.Println("[+] Subdomínios resolvidos:", paths.Resolved)
	fmt.Println("[+] Hosts vivos:", paths.Alive)
	fmt.Println("[+] Detalhes HTTP:", paths.HTTPDetails)
}
