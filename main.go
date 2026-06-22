package main

import (
	"fmt"
	"os"
	"os/exec"
)

func Buscar_Subdominios(hostname string) (error, string) {
	fmt.Println("[+] Iniciando Subfinder em ", hostname)
	cmd := exec.Command("subfinder", "-all", "-silent", "-d", hostname)
	out, err := cmd.Output()

	if err != nil {
		fmt.Println("Error em executar o comando: ", err)
		return err, ""
	}

	return nil, string(out)
} 

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Uso: go run main.go dominio.com")
		return
	}

	hostname := os.Args[1]

	fmt.Println("Bem vindo ao Rebellion!")


	Buscar_Subdominios(hostname)
}
