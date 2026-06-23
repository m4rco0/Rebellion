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
