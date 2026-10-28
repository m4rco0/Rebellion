package main

import (
	"fmt"

	"github.com/joho/godotenv"
	"github.com/m4rco0/Rebellion/internal/cli"
	"github.com/m4rco0/Rebellion/internal/database"
	pathing "github.com/m4rco0/Rebellion/internal/pathing"
	"github.com/m4rco0/Rebellion/internal/recon"
	"github.com/m4rco0/Rebellion/internal/store"
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

	// montando a string de conexão a partir do .env (com defaults)
	dsn, err := database.DSNFromEnv()
	if err != nil {
		fmt.Println(utils.VERMELHO+"[!] Configuração do banco inválida:"+utils.BRANCO, err)
		return
	}

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

	defer sqlDB.Close()

	fmt.Println(utils.VERDE + "[+] Postgres carregado com sucesso!\n\n" + utils.BRANCO)

	if err := sqlDB.Ping(); err != nil {
		fmt.Println(utils.VERMELHO+"[!] Erro ao conectar com o banco de dados:"+utils.BRANCO, err)
		return
	}

	fmt.Println(utils.VERDE + "[+] Conexão com o banco de dados estabelecida com sucesso!" + utils.BRANCO)

	// criando estrutura de pastas do projeto
	fmt.Println(utils.VERDE+"[+] Alvo:"+utils.BRANCO, args.Domain)
	fmt.Println(utils.VERDE+"[+] Scan ativo está:"+utils.BRANCO, args.ActiveScan)
	paths, err := pathing.NewScan(args.Domain)
	if err != nil {
		fmt.Println(utils.VERMELHO+"[!] Erro criando estrutura do projeto:"+utils.BRANCO, err)
		return
	}

	fmt.Println(utils.VERDE+"[+] Pasta do scan:"+utils.BRANCO, paths.Base)

	st := store.New(db)
	scan, err := st.StartScan(args.Domain)
	if err != nil {
		fmt.Println(utils.VERMELHO+"[!] Erro ao registrar o scan no banco:"+utils.BRANCO, err)
		return
	}

	fmt.Println(utils.VERDE+"[+] Scan registrado no banco:"+utils.BRANCO, scan.ID)

	status := store.StatusFailed
	var scanErr error
	defer func() {
		if err := st.FinishScan(scan.ID, status, scanErr); err != nil {
			fmt.Println(utils.VERMELHO+"[!] Erro ao finalizar o scan no banco:"+utils.BRANCO, err)
			return
		}

		if status == store.StatusCompleted {
			fmt.Println(utils.VERDE+"[+] Scan finalizado no banco:"+utils.BRANCO, status)
		} else {
			fmt.Println(utils.VERMELHO+"[!] Scan finalizado no banco:"+utils.BRANCO, status)
		}
	}()

	scanErr = recon.Scan_passivo(args.Domain, args.ResolverDNS, &paths)
	if scanErr != nil {
		fmt.Println(utils.VERMELHO+"[!] Erro no scan passivo:"+utils.BRANCO, scanErr)
	}

	// Persiste o que foi gerado mesmo quando uma etapa falha no meio.
	if err := persistPassive(st, scan.ID, paths); err != nil {
		if scanErr == nil {
			scanErr = err
		}
		fmt.Println(utils.VERMELHO+"[!] Erro ao salvar dados do scan passivo:"+utils.BRANCO, err)
	}

	if scanErr != nil {
		fmt.Println(utils.VERMELHO + "[!] Scan interrompido antes da etapa ativa" + utils.BRANCO)
		return
	}

	if args.ActiveScan {
		scanErr = recon.ScanAtivo(paths.Resolved, paths.HTTPDetails)
		if scanErr != nil {
			fmt.Println(utils.VERMELHO+"[!] Erro no scan ativo:"+utils.BRANCO, scanErr)
		}

		if err := persistActive(st, scan.ID, paths); err != nil {
			if scanErr == nil {
				scanErr = err
			}
			fmt.Println(utils.VERMELHO+"[!] Erro ao salvar dados do scan ativo:"+utils.BRANCO, err)
		}

		if scanErr != nil {
			fmt.Println(utils.VERMELHO + "[!] Scan interrompido" + utils.BRANCO)
			return
		}
	}

	status = store.StatusCompleted

	fmt.Println()
	fmt.Println(utils.VERDE + "[+] Scan finalizado" + utils.BRANCO)
	fmt.Println(utils.VERDE+"[+] Subdomínios brutos:"+utils.BRANCO, paths.SubdomainsRaw)
	fmt.Println(utils.VERDE+"[+] Subdomínios resolvidos:"+utils.BRANCO, paths.Resolved)
	fmt.Println(utils.VERDE+"[+] Hosts vivos:"+utils.BRANCO, paths.Alive)
	fmt.Println(utils.VERDE+"[+] Detalhes HTTP:"+utils.BRANCO, paths.HTTPDetails)
}
