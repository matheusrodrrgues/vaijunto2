# Simulador de carga e falhas

O simulador cria passageiros sem interface gráfica, conecta-os por TCP aos servidores das três empresas e executa login, busca e tentativa de reserva. Ao final, consulta as APIs REST para conferir o estado das transações.

Antes do teste, publique caronas que formem o itinerário informado. Para uma disputa controlada do último assento, use capacidade igual a `1` em todos os trechos envolvidos e informe `-capacidade 1`.

## Execução no computador

Com as três empresas do Compose em execução:

~~~powershell
go run ./simulador `
  -origem "Salvador" `
  -destino "Montes Claros" `
  -data "2026-10-20" `
  -requisicoes 100 `
  -concorrencia 20 `
  -espera-recuperacao 17s
~~~

Os endereços padrão são:

| Empresa | TCP | REST |
| --- | --- | --- |
| CaronasGo | `127.0.0.1:9001` | `http://127.0.0.1:8081` |
| LulaTransportes | `127.0.0.1:9002` | `http://127.0.0.1:8082` |
| TetraCaronas | `127.0.0.1:9003` | `http://127.0.0.1:8083` |

## Execução com Docker

~~~powershell
docker compose --profile teste run --rm simulador `
  -origem "Salvador" `
  -destino "Montes Claros" `
  -data "2026-10-20" `
  -requisicoes 100 `
  -concorrencia 20
~~~

Os relatórios são gravados em `relatorios/simulacao.json` e `relatorios/simulacao.csv`.

## Injeção de falha

A injeção que controla o Docker deve ser executada no computador, e não dentro do contêiner do simulador:

~~~powershell
go run ./simulador `
  -duracao 30s `
  -concorrencia 20 `
  -origem "Salvador" `
  -destino "Montes Claros" `
  -data "2026-10-20" `
  -falha-servico empresa-b `
  -falha-apos 5s `
  -falha-duracao 5s `
  -espera-recuperacao 17s
~~~

O simulador executa `docker compose stop empresa-b`, mantém a carga nas demais empresas e depois executa `docker compose start empresa-b`.

## Verificações

O resultado é reprovado quando:

- um trecho aparece em mais reservas do que a capacidade informada;
- uma reserva confirmada não está confirmada em todos os participantes;
- uma empresa termina com preparação ainda ativa;
- um coordenador termina com decisão pendente;
- a falha solicitada não pôde ser injetada ou o serviço não pôde ser restaurado;
- algum servidor não responde ao diagnóstico final.

Reservas recusadas por falta de assento são um resultado esperado e não representam inconsistência.

## Métricas

O JSON e o CSV registram:

- total por categoria;
- confirmações, recusas, buscas parciais e erros de conexão;
- latências mínima, média, p95 e máxima;
- operações por segundo;
- falhas injetadas;
- resumo das transações de cada empresa;
- inconsistências detectadas e resultado final.

## Opções principais

| Opção | Padrão | Finalidade |
| --- | --- | --- |
| `-requisicoes` | `100` | Total de passageiros quando não há duração |
| `-duracao` | `0` | Mantém a carga durante o período informado |
| `-concorrencia` | `20` | Máximo de operações simultâneas |
| `-capacidade` | `1` | Capacidade esperada por trecho |
| `-espera-recuperacao` | `17s` | Espera antes da auditoria final |
| `-falha-servico` | vazio | Serviço do Compose que será interrompido |
| `-json` | `relatorios/simulacao.json` | Relatório detalhado |
| `-csv` | `relatorios/simulacao.csv` | Resumo tabular |
