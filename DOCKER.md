# Execução das empresas com Docker

Esta etapa executa três instâncias isoladas do servidor. Cada empresa possui identidade, diretório de dados, diretório de logs e porta externa próprios.

## Subir as três empresas

```sh
docker compose build
docker compose up -d empresa-a empresa-b empresa-c
docker compose ps
```

| Serviço | Identidade | Porta no computador | Dados | Logs |
| --- | --- | --- | --- | --- |
| `empresa-a` | `empresa-a` | `9001` | `dados/empresa-a/` | `logs/empresa-a/` |
| `empresa-b` | `empresa-b` | `9002` | `dados/empresa-b/` | `logs/empresa-b/` |
| `empresa-c` | `empresa-c` | `9003` | `dados/empresa-c/` | `logs/empresa-c/` |

Os três contêineres escutam internamente em `9000`. As portas `9001`, `9002` e `9003` são apenas os acessos publicados no computador hospedeiro.

## Verificar o isolamento

```sh
docker compose logs empresa-a
docker compose logs empresa-b
docker compose logs empresa-c
```

Cada inicialização informa o ID, o nome da empresa e os caminhos exclusivos. Publicar uma carona na empresa A não altera os arquivos das empresas B e C.

## Abrir clientes da empresa A

```sh
docker compose run --rm --no-deps motorista
docker compose run --rm --no-deps cliente
```

Nesta primeira etapa, os clientes do Compose apontam para `empresa-a:9000`. A seleção/configuração de outras empresas será ampliada nas etapas seguintes.

## Executar uma empresa manualmente

```sh
docker build -f Dockerfile.servidor -t vaijunto-servidor .
docker run --name empresa-a --rm -p 9001:9000 \
  -e VAIJUNTO_EMPRESA_ID=empresa-a \
  -e VAIJUNTO_EMPRESA_NOME="Empresa A" \
  -e VAIJUNTO_DADOS_DIR=/app/dados \
  -e VAIJUNTO_LOGS_DIR=/app/logs \
  -v "${PWD}/dados/empresa-a:/app/dados" \
  -v "${PWD}/logs/empresa-a:/app/logs" \
  vaijunto-servidor
```

Para executar em computadores distintos, use a mesma imagem e altere a identidade da empresa em cada máquina. A comunicação REST entre elas será adicionada na Etapa 2.

## Encerrar

```sh
docker compose down
```

Os diretórios montados permanecem no computador depois que os contêineres são removidos.
