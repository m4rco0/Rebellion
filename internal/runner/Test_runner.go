package runner_test

import (
	"testing"
	"time"

	runner_test "github.com/m4rco0/Rebellion/internal/runner"
)

func TestRun(t *testing.T) {
	comando := "echo"
	args := []string{"Hello, World!"}

	result, err := runner_test.Run(5*time.Second, comando, args...)
	if err != nil {
		t.Fatalf("Erro ao executar o comando: %v", err)
	}

	expectedOutput := "Hello, World!\n"
	if result.Stdout != expectedOutput {
		t.Errorf("Saída esperada '%s', mas obteve '%s'", expectedOutput, result.Stdout)
	}

}
