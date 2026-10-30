# API REST entre servidores

Versão atual: `v1`

A API REST é usada somente na comunicação entre empresas do consórcio. Os clientes motorista e passageiro continuam conectados por TCP ao servidor da própria empresa.

## Convenções

- Corpo e respostas em `application/json; charset=utf-8`.
- Toda resposta contém o cabeçalho `X-Request-ID`.
- Se o emissor enviar `X-Request-ID`, o servidor devolve o mesmo valor.
- Se o cabeçalho estiver ausente, o servidor gera um identificador.
- Operações mutáveis futuras usarão também `Idempotency-Key` para permitir repetição segura.
- Cada chamada entre empresas possui timeout configurável.

## Formato de erro

```json
{
  "erro": {
    "codigo": "recurso_nao_encontrado",
    "mensagem": "recurso REST não encontrado"
  }
}
```

## `GET /api/v1/health`

Informa a identidade e a disponibilidade do servidor consultado.

Resposta `200 OK`:

```json
{
  "empresa_id": "empresa-b",
  "empresa_nome": "Empresa B",
  "status": "online"
}
```

Outras respostas:

| Código | Situação |
| --- | --- |
| `405 Method Not Allowed` | Método diferente de `GET`; a resposta inclui `Allow: GET` |

## `GET /api/v1/consortium/health`

Solicita ao servidor local que consulte, em paralelo, todos os pares configurados. A indisponibilidade de uma empresa aparece na lista e não transforma a resposta local em erro fatal.

Resposta `200 OK`:

```json
{
  "empresa_id": "empresa-a",
  "status": "online",
  "pares": [
    {
      "empresa_id": "empresa-b",
      "url_base": "http://empresa-b:8080",
      "disponivel": true,
      "status": "online",
      "duracao_ms": 3
    },
    {
      "empresa_id": "empresa-c",
      "url_base": "http://empresa-c:8080",
      "disponivel": false,
      "duracao_ms": 2001,
      "erro": "tempo limite excedido"
    }
  ]
}
```

As consultas são paralelas. Portanto, uma empresa lenta não soma seu timeout ao timeout das outras.

## `GET /api/v1/rides/search`

Devolve os trechos com vagas que pertencem à empresa consultada. O parâmetro opcional `date` filtra a data no formato `AAAA-MM-DD`. Sem o parâmetro, todas as datas disponíveis são retornadas.

Exemplo: `GET /api/v1/rides/search?date=2026-10-20`

Resposta `200 OK`:

```json
{
  "empresa_id": "empresa-b",
  "trechos": [
    {
      "empresa_id": "empresa-b",
      "carona_id": "CAR-001",
      "indice": 0,
      "data": "2026-10-20",
      "origem": "Feira de Santana",
      "destino": "Vitória da Conquista",
      "saida": "2026-10-20T09:20:00-03:00",
      "chegada": "2026-10-20T11:20:00-03:00",
      "preco_centavos": 3500,
      "vagas": 2
    }
  ]
}
```

Somente o catálogo necessário à montagem da rota é compartilhado. Contas, tokens e reservas não são expostos. O receptor valida a identidade da empresa, datas, horários, vagas e preços antes de aceitar os trechos.

## Códigos gerais

| Código | Significado |
| --- | --- |
| `200 OK` | Requisição concluída |
| `400 Bad Request` | Data em formato inválido |
| `404 Not Found` | Recurso inexistente |
| `405 Method Not Allowed` | Método não aceito pelo recurso |

## Configuração

| Variável | Exemplo | Finalidade |
| --- | --- | --- |
| `VAIJUNTO_REST_ADDR` | `:8080` | Endereço da API REST local |
| `VAIJUNTO_REST_TIMEOUT` | `2s` | Limite de cada chamada para outra empresa |
| `VAIJUNTO_PARES` | `empresa-b=http://empresa-b:8080,empresa-c=http://empresa-c:8080` | IDs e URLs dos pares |

O ID local não pode aparecer na lista de pares. IDs repetidos e URLs sem `http` ou `https` são recusados durante a inicialização.

## Comportamento diante de falhas

- Falha de conexão: o par é marcado como indisponível.
- Timeout: a chamada é interrompida e o par recebe o erro `tempo limite excedido`.
- JSON inválido ou identidade diferente da esperada: o par é tratado como resposta inválida.
- Os demais pares e as operações locais continuam funcionando.
- Na busca, os pares são consultados em paralelo. Os trechos válidos das empresas disponíveis ainda formam itinerários e o cliente recebe a indicação de busca parcial e os IDs dos pares indisponíveis.

## Próximos recursos

A próxima etapa acrescentará os recursos transacionais `prepare`, `commit`, `abort` e consulta de estado. Esses recursos seguirão as mesmas convenções de request ID, timeout, erros JSON e idempotência.
