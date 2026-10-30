# Problema 2 - Consórcio de Caronas Compartilhadas

Este documento consolida o enunciado e o barema recebidos em 29/10/2026. Ele distingue requisitos obrigatórios de decisões técnicas propostas para a implementação.

## 1. Mudança em relação ao Problema 1

No Problema 1, um único servidor mantinha todos os usuários, caronas, assentos e reservas. Uma trava local era suficiente para verificar e reservar todos os trechos de um itinerário.

No Problema 2, cada empresa possui:

- seu próprio servidor;
- seus próprios usuários;
- suas próprias caronas e assentos;
- seu próprio armazenamento;
- sua própria fila de requisições;
- comunicação REST com os demais servidores do consórcio.

O passageiro continua conectado somente à empresa em que está cadastrado, mas pode pesquisar e reservar um itinerário formado por caronas de várias empresas. Não pode existir servidor central, coordenador fixo, banco compartilhado ou serviço externo de coordenação.

## 2. Requisitos obrigatórios do produto

### Servidores

- Executar pelo menos três instâncias do servidor, representando empresas distintas.
- Cada instância deve possuir caronas, armazenamento e estado próprios.
- Cada servidor deve oferecer uma API REST aos demais servidores.
- Uma empresa não pode depender de um servidor central nem manter os dados comerciais das demais.
- A queda de uma empresa não pode interromper as operações locais independentes das outras.

### Clientes

- Reaproveitar os clientes motorista e passageiro do Problema 1 quando conveniente.
- Cada cliente deve se conectar exclusivamente ao servidor de sua empresa.
- Motoristas publicam caronas somente na empresa em que estão cadastrados.
- Passageiros pesquisam e reservam itinerários que podem envolver várias empresas.

### Reserva distribuída

- Reservar todos os trechos ou nenhum deles.
- Nunca confirmar o mesmo assento do mesmo trecho para passageiros diferentes.
- Definir e justificar uma regra de precedência para disputas simultâneas.
- Não depender da sincronização dos relógios dos servidores.
- Evitar deadlock quando transações concorrentes solicitam os mesmos recursos em ordens diferentes.
- Detectar falhas por timeout ou mecanismo equivalente.
- Não deixar itinerários confirmados pela metade.
- Não deixar assentos bloqueados indefinidamente.
- Recuperar um estado consistente quando um servidor volta a operar.

### Testes e operação

- Implementar simulador autônomo de passageiros, sem interface gráfica.
- Gerar grande volume de buscas e reservas simultâneas a partir de todas as empresas.
- Automatizar testes de carga, concorrência entre servidores e injeção de falhas.
- Verificar ausência de venda dupla, reserva parcial e bloqueios órfãos.
- Executar os servidores em contêineres Docker isolados.
- Executar as empresas em computadores distintos no laboratório.
- Permitir que o tutor interrompa uma instância durante a apresentação.

### Documentação

- Especificar integralmente a API REST entre servidores.
- Documentar recursos, métodos, requisições, respostas e códigos HTTP.
- Documentar o comportamento de timeout e repetição de requisições.
- Incluir diagrama de sequência da confirmação distribuída.
- Produzir relatório no formato SBC.
- Comparar a arquitetura centralizada do Problema 1 com a arquitetura distribuída.
- Justificar tecnicamente as decisões de projeto.

## 3. Decisões técnicas propostas

Estas decisões ainda devem ser validadas durante a implementação e pelos testes.

### 3.1 Comunicação

- Manter o protocolo TCP existente entre cliente e servidor da mesma empresa.
- Usar HTTP/JSON REST exclusivamente na comunicação entre servidores.
- Identificar cada requisição distribuída por um `transaction_id` globalmente único.
- Tornar `prepare`, `commit` e `abort` idempotentes para permitir repetição segura após timeout.
- Configurar timeout em toda chamada entre servidores.

### 3.2 Busca distribuída

- Usar consulta em tempo real aos pares conhecidos do consórcio.
- Cada empresa retorna somente as caronas sob sua responsabilidade.
- O servidor de origem monta o grafo temporário e calcula os itinerários.
- Se uma empresa estiver indisponível, a busca continua com as respostas recebidas e sinaliza resultado parcial.

Essa estratégia evita replicar catálogos comerciais e simplifica a consistência, ao custo de maior latência e resultados incompletos durante falhas.

### 3.3 Exclusão mútua e precedência

- O servidor proprietário do trecho é a única autoridade capaz de bloquear e confirmar seus assentos.
- Requisições concorrentes para o mesmo trecho são serializadas localmente pelo proprietário.
- A precedência é definida pela ordem monotônica de aceitação no servidor proprietário, e não por relógios enviados pelos demais servidores.
- A resposta do `prepare` informa o resultado definitivo da disputa naquele proprietário.

Essa regra independe da sincronização dos relógios. A ordem pode variar conforme os atrasos de rede, mas permanece única, verificável e igual para todos os solicitantes do mesmo recurso.

### 3.4 Prevenção de deadlock

- Representar cada recurso por `(empresa_id, carona_id, trecho_indice)`.
- Ordenar globalmente os recursos por essa chave.
- Solicitar os bloqueios sempre nessa ordem, independentemente da ordem original do itinerário.
- Desfazer todos os bloqueios obtidos se qualquer `prepare` for recusado ou expirar.

A ordenação elimina espera circular. O custo é reduzir o paralelismo durante itinerários longos e exigir que o coordenador conheça todos os participantes antes do primeiro bloqueio.

### 3.5 Atomicidade distribuída

- Usar um protocolo de confirmação em duas fases com registros duráveis e recuperação:
  1. o servidor do passageiro atua como coordenador daquela transação;
  2. envia `prepare` aos proprietários dos trechos, em ordem global;
  3. cada participante persiste o bloqueio antes de responder `prepared`;
  4. se todos aceitarem, o coordenador persiste a decisão `commit`;
  5. se alguém recusar, persiste `abort`;
  6. o coordenador transmite a decisão a todos;
  7. mensagens repetidas retornam o estado já persistido.
- O coordenador é escolhido por transação e não constitui um coordenador central fixo.

### 3.6 Falhas e recuperação

- Persistir o estado da transação antes de responder pela rede.
- Manter estados `preparing`, `prepared`, `committed` e `aborted`.
- Após reinício, carregar transações incompletas e executar recuperação.
- Participantes em estado incerto consultam o coordenador e os demais participantes conhecidos.
- Bloqueios possuem prazo e rotina de recuperação; não são removidos apenas por uma mensagem perdida.
- Operações locais que não dependem do servidor indisponível continuam funcionando.

O comportamento será validado sob o modelo de falha por parada e recuperação usado no laboratório. Partições prolongadas exigem explicitar o compromisso entre disponibilidade e consistência no relatório.

## 4. API REST inicial proposta

| Método | Recurso | Finalidade |
|---|---|---|
| `GET` | `/api/v1/health` | Verificar disponibilidade da empresa |
| `GET` | `/api/v1/rides/search` | Consultar trechos locais disponíveis |
| `POST` | `/api/v1/transactions/{id}/prepare` | Solicitar bloqueio durável dos trechos locais |
| `PUT` | `/api/v1/transactions/{id}/commit` | Aplicar decisão de confirmação |
| `PUT` | `/api/v1/transactions/{id}/abort` | Liberar bloqueios da transação |
| `GET` | `/api/v1/transactions/{id}` | Consultar decisão e estado para recuperação |

Todos os endpoints transacionais devem aceitar repetição da mesma operação sem duplicar reservas ou liberar assentos incorretamente.

## 5. Matriz do barema

| # | Critério | Evidência necessária |
|---|---|---|
| 1 | Arquitetura | Diagrama de componentes, propriedade dos dados e comparação com o Problema 1 |
| 2 | API REST | OpenAPI ou especificação equivalente, exemplos, códigos HTTP, timeout e idempotência |
| 3 | Busca distribuída | Algoritmo, resposta parcial e teste com uma empresa indisponível |
| 4 | Exclusão mútua | Teste concorrente disputando o último assento por servidores diferentes |
| 5 | Atomicidade | Protocolo, log durável, diagrama de sequência e teste de recusa parcial |
| 6 | Precedência | Regra independente de relógio e teste reproduzível da disputa |
| 7 | Deadlock | Ordem global de recursos e teste com itinerários solicitados em ordens opostas |
| 8 | Tolerância a falhas | Queda antes, durante e depois da confirmação |
| 9 | Recuperação | Reinício, leitura do log e conclusão consistente das transações |
| 10 | Testes | Carga, concorrência, injeção de falhas e métricas |
| 11 | Emulação | Três contêineres em computadores distintos e demonstração de queda |

## 6. Ordem de implementação

1. Separar configuração, identidade e armazenamento por empresa.
2. Subir três servidores independentes em Docker.
3. Implementar descoberta/configuração dos pares e `health`.
4. Implementar busca REST federada.
5. Definir modelos e persistência das transações.
6. Implementar `prepare`, `commit`, `abort` e consulta de estado.
7. Implementar ordenação global e idempotência.
8. Implementar timeouts e recuperação após reinício.
9. Adaptar os clientes para apontarem somente para sua empresa.
10. Criar simulador de carga e injetor de falhas.
11. Automatizar os cenários do barema.
12. Documentar API, diagramas, resultados e comparação arquitetural.

## 7. Critérios para considerar a implementação pronta

- Três empresas executam com dados e armazenamento separados.
- Um passageiro reserva atomicamente um itinerário com trechos das três empresas.
- Uma disputa concorrente confirma somente uma reserva para o último assento.
- Repetir qualquer mensagem transacional não duplica efeitos.
- Itinerários em ordens opostas não produzem deadlock.
- Derrubar um servidor não interrompe operações independentes nos demais.
- Reiniciar o servidor restaura o estado e resolve transações incompletas.
- Nenhum bloqueio fica órfão após o cenário de recuperação.
- O simulador produz métricas de carga e resultados reproduzíveis.
- A documentação e a apresentação cobrem os 11 itens do barema.

## 8. Datas registradas no enunciado

- Entrega e apresentação: 27/10/2026.
- Final das apresentações no calendário: 29/10/2026.
- Duração prevista para apresentação e arguição: 30 minutos.

Essas datas já foram alcançadas na data de recebimento deste documento; qualquer novo prazo deve ser confirmado com o tutor.
