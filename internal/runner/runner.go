package runner

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"time"
)

type Result struct {
	Stdout string
	Stderr string
}

func Run(timeout time.Duration, name string, args ...string) (Result, error) {
	if _, err := exec.LookPath(name); err != nil {
		return Result{}, fmt.Errorf("ferramenta não encontrada: %s", name)
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, name, args...)

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()

	result := Result{
		Stdout: stdout.String(),
		Stderr: stderr.String(),
	}

	if ctx.Err() == context.DeadlineExceeded {
		return result, fmt.Errorf("timeout executando %s", name)
	}

	if err != nil {
		return result, fmt.Errorf("erro executando %s: %v | stderr: %s", name, err, result.Stderr)
	}

	return result, nil
}
func Log_scan_step(name string, fn func() error) error {
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

func RunPipeline(timeout time.Duration, commands ...[]string) (Result, error) {
	if len(commands) == 0 {
		return Result{}, fmt.Errorf("nenhum comando fornecido")
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	cmds := make([]*exec.Cmd, 0, len(commands))

	for _, command := range commands {
		if len(command) == 0 {
			return Result{}, fmt.Errorf("commando vazio")
		}

		if _, err := exec.LookPath(command[0]); err != nil {
			return Result{}, fmt.Errorf("ferramenta não encontrada: %s", command[0])
		}

		cmds = append(cmds, exec.CommandContext(ctx, command[0], command[1:]...))
	}

	var stderr bytes.Buffer
	var stdout bytes.Buffer

	for i := 0; i < len(cmds)-1; i++ {
		pipe, err := cmds[i].StdoutPipe()

		if err != nil {
			return Result{}, fmt.Errorf("erro em criar pipe: %v", err)
		}

		cmds[i+1].Stdin = pipe
	}

	cmds[len(cmds)-1].Stdout = &stdout

	for _, cmd := range cmds {
		cmd.Stderr = &stderr
	}

	for _, cmd := range cmds {
		if err := cmd.Start(); err != nil {
			return Result{}, fmt.Errorf("erro iniciando %s: %v", cmd.Args[0], err)
		}
	}

	for _, cmd := range cmds {
		if err := cmd.Wait(); err != nil {
			if ctx.Err() == context.DeadlineExceeded {
				return Result{}, fmt.Errorf("timeout executando pipeline")
			}

			return Result{
				Stdout: stdout.String(),
				Stderr: stderr.String(),
			}, fmt.Errorf(
				"erro executando %s: %v | stderr: %s",
				cmd.Args[0],
				err,
				stderr.String(),
			)
		}
	}

	return Result{
		Stdout: stdout.String(),
		Stderr: stderr.String(),
	}, nil
}
