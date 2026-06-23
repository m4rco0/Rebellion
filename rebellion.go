package main

import (
	"fmt"
	"os"

	pathin "github.com/m4rco0/Rebellion/internal/pathing"
	"github.com/m4rco0/Rebellion/internal/recon"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Uso: go run . dominio.com")
		return
	}

	domain := os.Args[1]

	fmt.Println("================================")
	fmt.Println("        Rebellion Scanner       ")
	fmt.Println("================================")
	fmt.Println("[+] Alvo:", domain)

	paths, err := pathin.NewScan(domain) 
	if err != nil {
		fmt.Println("[!] Erro criando estrutura do projeto:", err)
		return
	}

	fmt.Println("[+] Pasta do scan:", paths.Base)

	err = step("Subfinder", func() error {
		return recon.BuscarSubdominios(domain, paths.SubdomainsRaw)
	})
	if err != nil {
		return
	}

	err = step("PureDNS", func() error {
		return recon.RunPureDNS(paths.SubdomainsRaw, "resolvers.txt", paths.Resolved)
	})
	if err != nil {
		return
	}


	err = step("Httpx", func() error {
		return recon.RunHttpx(paths.Resolved, paths.Alive)
	})

	if err != nil {
		return
	}

	fmt.Println()
	fmt.Println("[+] Scan finalizado")
	fmt.Println("[+] Subdomínios brutos:", paths.SubdomainsRaw)
	fmt.Println("[+] Subdomínios resolvidos:", paths.Resolved)
	fmt.Println("[+] Hosts vivos:", paths.Alive)
	fmt.Println("[+] Detalhes HTTP:", paths.HTTPDetails)
}

func step(name string, fn func() error) error {
	fmt.Println()
	fmt.Println("[+] Rodando:", name)

	err := fn()
	if err != nil {
		fmt.Println("[!] Erro em", name+":", err)
		return err
	}

	fmt.Println("[+] Finalizado:", name)
	return nil
}
