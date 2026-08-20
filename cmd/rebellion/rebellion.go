package main

import (
	"fmt"
	"os"

	"github.com/joho/godotenv"
	"github.com/m4rco0/Rebellion/internal/cli"
	"github.com/m4rco0/Rebellion/internal/database"
	pathing "github.com/m4rco0/Rebellion/internal/pathing"
	"github.com/m4rco0/Rebellion/internal/recon"
	"github.com/m4rco0/Rebellion/internal/utils"
)

func main() {
	args := cli.ParserArgs()
	utils.ShowBanner()
	fmt.Println(utils.AZUL + "[+] Inicializando o banco de dados" + utils.BRANCO)

	// pegando as credenciais no .env
	if err := godotenv.Load(); err != nil {
		fmt.Println(utils.VERMELHO+"[!] Erro ao carregar o arquivo .env:"+utils.BRANCO, err)
		return
	}

	// criando string do bd
	dsn := fmt.Sprintf("host=%s user=%s password=%s dbname=%s port=%s sslmode=disable",
		os.Getenv("DB_HOST"), os.Getenv("DB_USER"), os.Getenv("DB_PASSW"), os.Getenv("DB_NAME"), os.Getenv("DB_PORT"))

	db, err := database.InitDB(dsn)
	if err != nil {
		fmt.Println(utils.VERMELHO+"[!] Erro ao inicializar o banco de dados:"+utils.BRANCO, err)
		return
	}

	// Testando a conexão com o banco de dados
	sqlDB, err := db.DB()
	if err != nil {
		fmt.Println(utils.VERMELHO+"[!] Erro ao obter a conexão com o banco de dados:"+utils.BRANCO, err)
		return
	}

	fmt.Println(utils.VERDE + "[+] Postgres carregado com sucesso!\n\n" + utils.BRANCO)

	sqlDB.Ping()

	fmt.Println(utils.VERDE + "[+] Conexão com o banco de dados estabelecida com sucesso!" + utils.BRANCO)

	// criando estrutura de pastas do projeto
	fmt.Println(utils.VERDE+"[+] Alvo:"+utils.BRANCO, args.Domain)
	fmt.Println(utils.VERDE+"[+] Scan ativo está:"+utils.BRANCO, args.ActiveScan)
	fmt.Println(utils.VERDE+"[+] Domínio:"+utils.BRANCO, args.Domain)
	paths, err := pathing.NewScan(args.Domain)
	if err != nil {
		fmt.Println(utils.VERMELHO+"[!] Erro criando estrutura do projeto:"+utils.BRANCO, err)
		return
	}

	fmt.Println(utils.VERDE+"[+] Pasta do scan:"+utils.BRANCO, paths.Base)

	err = recon.Scan_passivo(args.Domain, args.ResolverDNS, &paths)
	if err != nil {
		fmt.Println(utils.VERMELHO+"[!] Erro no scan passivo:"+utils.BRANCO, err)
		return
	}

	if args.ActiveScan {
		err := recon.ScanAtivo(paths.Resolved, paths.Alive)
		if err != nil {
			fmt.Println(utils.VERMELHO+"[!] Erro no scan ativo:"+utils.BRANCO, err)
			return
		}
	}

	fmt.Println()
	fmt.Println(utils.VERDE + "[+] Scan finalizado" + utils.BRANCO)
	fmt.Println(utils.VERDE+"[+] Subdomínios brutos:"+utils.BRANCO, paths.SubdomainsRaw)
	fmt.Println(utils.VERDE+"[+] Subdomínios resolvidos:"+utils.BRANCO, paths.Resolved)
	fmt.Println(utils.VERDE+"[+] Hosts vivos:"+utils.BRANCO, paths.Alive)
	fmt.Println(utils.VERDE+"[+] Detalhes HTTP:"+utils.BRANCO, paths.HTTPDetails)
}
