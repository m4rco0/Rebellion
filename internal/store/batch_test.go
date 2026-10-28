package store

import "testing"

func TestChunkSlice(t *testing.T) {
	values := []int{1, 2, 3, 4, 5, 6, 7}

	chunks := chunkSlice(values, 3)
	if len(chunks) != 3 {
		t.Fatalf("esperado 3 lotes, obteve %d", len(chunks))
	}
	if len(chunks[0]) != 3 || len(chunks[1]) != 3 || len(chunks[2]) != 1 {
		t.Errorf("tamanhos inesperados: %d, %d, %d", len(chunks[0]), len(chunks[1]), len(chunks[2]))
	}
	if chunks[2][0] != 7 {
		t.Errorf("último lote deveria conter 7, obteve %v", chunks[2])
	}

	if got := chunkSlice([]int{}, 10); len(got) != 0 {
		t.Errorf("lista vazia deveria gerar nenhum lote, obteve %v", got)
	}

	// Tamanho inválido não pode gerar loop infinito.
	if got := chunkSlice(values, 0); len(got) != len(values) {
		t.Errorf("tamanho 0 deveria ser tratado como 1, obteve %d lotes", len(got))
	}
}

func TestBatchLimitsStayBelowPostgresParameterLimit(t *testing.T) {
	// O protocolo estendido do PostgreSQL aceita no máximo 65535 parâmetros por
	// comando. A tabela mais larga é http_probes (18 colunas por linha).
	const postgresParamLimit = 65535
	const widestRowColumns = 18

	if insertBatchSize*widestRowColumns >= postgresParamLimit {
		t.Fatalf("lote de inserção (%d) excede o limite de parâmetros", insertBatchSize)
	}

	// findStoredURLs combina duas listas por consulta.
	if queryBatchSize*2 >= postgresParamLimit {
		t.Fatalf("lote de consulta (%d) excede o limite de parâmetros", queryBatchSize)
	}
}
