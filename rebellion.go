package main

import (
	"flag"
	"fmt"

	"github.com/m4rco0/Rebellion/internal/cli"
	pathing "github.com/m4rco0/Rebellion/internal/pathing"
	"github.com/m4rco0/Rebellion/internal/recon"
	"github.com/m4rco0/Rebellion/internal/utils"
)


func main() {

	args := cli.ParserArgs()
	utils.ShowBanner()
	fmt.Println("[+] Alvo:", args.Domain)

	fmt.Println(flag.Arg(1))

	paths, err := pathing.NewScan(args.Domain)
	if err != nil {
		fmt.Println("[!] Erro criando estrutura do projeto:", err)
		return
	}

	fmt.Println("[+] Pasta do scan:", paths.Base)
	err = recon.Scan_passivo(args.Domain, &paths)
	if err != nil {
		fmt.Println("[!] Erro no scan passivo:", err)
		return
	}
	if args.ActiveScan{
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
