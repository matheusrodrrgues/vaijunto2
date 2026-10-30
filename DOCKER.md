# Execução das empresas com Docker

Esta etapa executa três instâncias isoladas do servidor. Cada empresa possui identidade, diretório de dados, diretório de logs e porta externa próprios.

## Subir as três empresas

```sh
docker compose build
docker compose up -d empresa-a empresa-b empresa-c
docker compose ps
```

| Serviço | Nome | Identidade | TCP | REST | Dados | Logs |
| --- | --- | --- | --- | --- | --- | --- |
| `empresa-a` | CaronasGo | `empresa-a` | `9001` | `8081` | `dados/empresa-a/` | `logs/empresa-a/` |
| `empresa-b` | LulaTransportes | `empresa-b` | `9002` | `8082` | `dados/empresa-b/` | `logs/empresa-b/` |
| `empresa-c` | TetraCaronas | `empresa-c` | `9003` | `8083` | `dados/empresa-c/` | `logs/empresa-c/` |

Os três contêineres escutam internamente em `9000` para clientes e `8080` para os outros servidores.

## Verificar o isolamento

```sh
docker compose logs empresa-a
docker compose logs empresa-b
docker compose logs empresa-c
curl http://localhost:8081/api/v1/consortium/health
```

Cada inicialização informa o ID, o nome da empresa e os caminhos exclusivos. Publicar uma carona na CaronasGo não altera os arquivos da LulaTransportes e da TetraCaronas.

## Abrir clientes da CaronasGo

```sh
docker compose run --rm --no-deps motorista
docker compose run --rm --no-deps cliente
```

Nesta primeira etapa, os clientes do Compose apontam para `empresa-a:9000`. A seleção/configuração de outras empresas será ampliada nas etapas seguintes.

## Executar uma empresa manualmente

```sh
docker build -f Dockerfile.servidor -t vaijunto-servidor .
docker run --name empresa-a --rm -p 9001:9000 -p 8081:8080 \
  -e VAIJUNTO_EMPRESA_ID=empresa-a \
  -e VAIJUNTO_EMPRESA_NOME="CaronasGo" \
  -e VAIJUNTO_REST_ADDR=:8080 \
  -e VAIJUNTO_REST_TIMEOUT=2s \
  -e VAIJUNTO_PARES=empresa-b=http://IP_B:8080,empresa-c=http://IP_C:8080 \
  -e VAIJUNTO_DADOS_DIR=/app/dados \
  -e VAIJUNTO_LOGS_DIR=/app/logs \
  -v "${PWD}/dados/empresa-a:/app/dados" \
  -v "${PWD}/logs/empresa-a:/app/logs" \
  vaijunto-servidor
```

Para executar em computadores distintos, use a mesma imagem, altere a identidade e configure `VAIJUNTO_PARES` com os IPs REST das outras máquinas.

## Encerrar

```sh
docker compose down
```

Os diretórios montados permanecem no computador depois que os contêineres são removidos.
