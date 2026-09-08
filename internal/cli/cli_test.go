package cli

import (
	"strings"
	"testing"
)

func TestExpandirTil(t *testing.T) {
	expectedDirs := map[string]string{
		"~/testdir":      "/home/user/testdir.txt", // Ajuste conforme o diretório home do usuário
		"/absolute/path": "/absolute/path.log",
		"/temp/user/dir": "/temp/user/dir.txt",
		"~/anotherdir":   "/home/user/anotherdir.txt", // Ajuste conforme o diretório home do usuário
	}

	for relativo, absoluto := range expectedDirs {
		if absoluto == relativo {
			t.Errorf("Esperado que o caminho relativo '%s' seja expandido, mas obteve '%s'", relativo, absoluto)
		}
		if !strings.HasPrefix(absoluto, "/") {
			t.Errorf("Esperado que o caminho absoluto comece com '/', mas obteve '%s'", absoluto)
		}
	}

}
