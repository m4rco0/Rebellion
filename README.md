# Rebellion

Rebellion e uma ferramenta de recon para enumerar subdominios, resolver DNS e, opcionalmente, verificar hosts HTTP ativos.

## Requisitos

- Go 1.26.4 ou superior
- `subfinder`
- `puredns`
- `httpx`
- Arquivo `resolvers.txt` na raiz do projeto

As ferramentas externas precisam estar instaladas e disponiveis no `PATH`, pois o Rebellion chama esses binarios diretamente.

## Instalar dependencias externas

Instale as ferramentas da ProjectDiscovery:

```bash
go install github.com/projectdiscovery/subfinder/v2/cmd/subfinder@latest
go install github.com/projectdiscovery/httpx/cmd/httpx@latest
```

Instale o PureDNS:

```bash
go install github.com/d3mondev/puredns/v2@latest
```
Instale o Waybackurls e o Gau
```bash
 go install github.com/lc/gau/v2/cmd/gau@latest
 go install github.com/tomnomnom/waybackurls@latest
```
Confirme que os comandos estao acessiveis:

```bash
subfinder -v
puredns -v
httpx -v
gau -v
waybackurls -v
```

Se algum comando nao for encontrado, adicione o diretorio de binarios do Go ao `PATH`:

```bash
export PATH="$PATH:$(go env GOPATH)/bin"
```

## Preparar o projeto

Clone o repositorio e entre na pasta:

```bash
git clone https://github.com/m4rco0/Rebellion.git
cd Rebellion
```
Resolva as dependencias 
```bash
go mod tidy
```
Garanta que o arquivo `resolvers.txt` exista na raiz do projeto. Ele e usado pelo `puredns` durante a etapa de resolucao:

```bash
ls resolvers.txt
```

## Compilar

Compile o binario:

```bash
go build ./cmd/rebellion/rebellion.go 
```

## Executar

Scan passivo:

```bash
./Rebellion exemplo.com
```

O scan passivo executa:

- `subfinder` para buscar subdominios
- `puredns` para resolver os subdominios encontrados
- `waybackurl` e `gau` para pegar urls antigos

Scan com etapa ativa:

```bash
./Rebellion --active-scan exemplo.com
```

O scan ativo adiciona a execucao do `httpx` sobre os hosts resolvidos.

Tambem e possivel executar sem compilar:

```bash
go run rebellion.go exemplo.com
go run rebellion.go --active-scan exemplo.com
```

## Saida

Cada execucao cria uma pasta em:

```text
output/<dominio>/<data_hora>/
```

Estrutura gerada:

```text
output/<dominio>/<data_hora>/
|-- 01_passive/
|   `-- subdomains_raw.txt
|-- 02_dns/
|   `-- resolved.txt
|-- 03_http/
|   |-- alive.txt
|   `-- httpx_details.txt
|-- 04_urls/
|-- 05_findings/
`-- logs/
```

Arquivos principais:

- `01_passive/subdomains_raw.txt`: subdominios brutos encontrados pelo `subfinder`
- `02_dns/resolved.txt`: subdominios resolvidos pelo `puredns`
- `03_http/alive.txt`: saida do `httpx` quando `--active-scan` for usado
- `03_http/httpx_details.txt`: caminho reservado para detalhes HTTP

## Uso esperado

Execute o Rebellion informando apenas o dominio alvo:

```bash
./Rebellion alvo.com
```

Para incluir validacao HTTP dos hosts resolvidos:

```bash
./Rebellion --active-scan alvo.com
```

Ao final, o programa imprime no terminal os caminhos dos arquivos gerados.

## Solucao de problemas

Erro `ferramenta nao encontrada`:

- Verifique se `subfinder`, `puredns` ou `httpx` estao instalados.
- Confirme que `$(go env GOPATH)/bin` esta no `PATH`.

Erro no `puredns`:

- Confirme que `resolvers.txt` existe na raiz do projeto.
- Verifique se o arquivo contem resolvers DNS validos.

Nenhum host vivo no scan ativo:

- Confira se `02_dns/resolved.txt` contem dominios resolvidos.
- Tente executar novamente com outro alvo ou com uma lista de resolvers atualizada.
