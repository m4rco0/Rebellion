package main

import (
	"errors"
	"fmt"
	"os"

	pathing "github.com/m4rco0/Rebellion/internal/pathing"
	"github.com/m4rco0/Rebellion/internal/storage"
	"github.com/m4rco0/Rebellion/internal/store"
	"github.com/m4rco0/Rebellion/internal/utils"
)

// persistPassive lê os artefatos gerados pelo scan passivo e grava no banco.
// Arquivos de etapas desativadas (subfinder/puredns) são ignorados sem erro.
func persistPassive(st *store.Store, scanID uint, paths pathing.ScanPaths) error {
	var errs []error

	subdomains, err := storage.ParseLinesFile(paths.SubdomainsRaw)
	switch {
	case err == nil:
		if len(subdomains) > 0 {
			result, err := st.SaveSubdomains(scanID, subdomains, "subfinder")
			if err != nil {
				errs = append(errs, fmt.Errorf("salvar subdomínios: %w", err))
			} else {
				fmt.Println(utils.VERDE+"[+] Subdomínios:"+utils.BRANCO,
					result.Processed, "no escopo,", result.Created, "novos no domínio")
			}
		}
	case errors.Is(err, os.ErrNotExist):
		fmt.Println(utils.AZUL+"[!] Arquivo não encontrado, pulando persistência:"+utils.BRANCO, paths.SubdomainsRaw)
	default:
		errs = append(errs, fmt.Errorf("ler %s: %w", paths.SubdomainsRaw, err))
	}

	resolved, err := storage.ParseLinesFile(paths.Resolved)
	switch {
	case err == nil:
		if len(resolved) > 0 {
			result, err := st.SaveSubdomains(scanID, resolved, "puredns")
			if err != nil {
				errs = append(errs, fmt.Errorf("salvar subdomínios resolvidos: %w", err))
			} else {
				fmt.Println(utils.VERDE+"[+] Subdomínios resolvidos:"+utils.BRANCO,
					result.Processed, "no escopo,", result.Created, "novos no domínio")
			}
		}
	case errors.Is(err, os.ErrNotExist):
		fmt.Println(utils.AZUL+"[!] Arquivo não encontrado, pulando persistência:"+utils.BRANCO, paths.Resolved)
	default:
		errs = append(errs, fmt.Errorf("ler %s: %w", paths.Resolved, err))
	}

	urls, err := storage.ParseLinesFile(paths.KatanaURLs)
	switch {
	case err == nil:
		if len(urls) > 0 {
			if err := st.SaveURLs(scanID, urls); err != nil {
				errs = append(errs, fmt.Errorf("salvar URLs: %w", err))
			} else {
				fmt.Println(utils.VERDE+"[+] URLs salvas no banco:"+utils.BRANCO, len(urls))
			}
		}
	case errors.Is(err, os.ErrNotExist):
		fmt.Println(utils.AZUL+"[!] Arquivo não encontrado, pulando persistência:"+utils.BRANCO, paths.KatanaURLs)
	default:
		errs = append(errs, fmt.Errorf("ler %s: %w", paths.KatanaURLs, err))
	}

	return errors.Join(errs...)
}

// persistActive lê a saída JSON do httpx, grava os probes no banco e deriva o
// arquivo alive.txt a partir das URLs que responderam.
func persistActive(st *store.Store, scanID uint, paths pathing.ScanPaths) error {
	probes, err := storage.ParseHttpxFile(paths.HTTPDetails)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			fmt.Println(utils.AZUL+"[!] Saída do httpx não encontrada, pulando persistência:"+utils.BRANCO, paths.HTTPDetails)
			return nil
		}
		return fmt.Errorf("ler %s: %w", paths.HTTPDetails, err)
	}
	if len(probes) == 0 {
		return nil
	}

	if err := st.SaveHTTPProbes(scanID, probes); err != nil {
		return fmt.Errorf("salvar resultados HTTP: %w", err)
	}

	alive := make([]string, 0, len(probes))
	for _, probe := range probes {
		if probe.Failed || probe.URL == "" {
			continue
		}
		alive = append(alive, probe.URL)
	}
	if err := storage.WriteLines(paths.Alive, alive); err != nil {
		return fmt.Errorf("gravar %s: %w", paths.Alive, err)
	}

	fmt.Println(utils.VERDE+"[+] Resultados HTTP salvos no banco:"+utils.BRANCO, len(probes))
	return nil
}
