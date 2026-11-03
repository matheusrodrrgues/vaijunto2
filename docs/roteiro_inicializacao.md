# Roteiro de testes e demonstração em três máquinas

Este roteiro prepara o VaiJunto para uma apresentação com uma empresa em cada computador, abre os clientes de motorista e passageiro e executa testes de integração, concorrência e recuperação de falha.

## 1. Organização do laboratório

Use três computadores na mesma rede local. Substitua os endereços de exemplo pelos IPs reais:

| Máquina | Empresa | ID | IP de exemplo | TCP dos clientes | REST entre empresas |
|---|---|---|---|---:|---:|
| A | CaronasGo | `empresa-a` | `192.168.0.101` | `9000` | `8080` |
| B | LulaTransportes | `empresa-b` | `192.168.0.102` | `9000` | `8080` |
| C | TetraCaronas | `empresa-c` | `192.168.0.103` | `9000` | `8080` |

Em cada máquina:

1. copie ou clone a mesma versão do projeto;
2. abra o terminal na raiz do projeto;
3. confirme que Go 1.22 ou superior, Docker Engine e Docker Compose estão disponíveis;
4. libere as portas TCP `9000` e `8080` no firewall.

Descubra o IPv4 de cada computador com:

```bash
hostname -I
```

Não use `127.0.0.1` para comunicar computadores diferentes. Os IPs devem permanecer os mesmos durante toda a demonstração.

Se o firewall estiver ativo, libere as portas nas três máquinas. Com UFW:

```bash
sudo ufw allow 9000/tcp
sudo ufw allow 8080/tcp
```

## 2. Validar o projeto antes da apresentação

Execute em pelo menos uma máquina:

```bash
go test ./...
go vet ./...
docker compose config --quiet
```

Todos os comandos devem terminar sem erros.

## 3. Construir a imagem do servidor

Execute nas três máquinas:

```bash
docker build -f Dockerfile.servidor -t vaijunto-servidor .
```

## 4. Ligar uma empresa em cada máquina

O script `scripts/iniciar-empresa.sh` contém a configuração do Docker. Informe sempre os três IPs na ordem A, B e C. Os dados ficam em `dados/<empresa>` e os logs em `logs/<empresa>`.

### Máquina A — CaronasGo

```bash
bash scripts/iniciar-empresa.sh empresa-a 192.168.0.101 192.168.0.102 192.168.0.103
```

### Máquina B — LulaTransportes

```bash
bash scripts/iniciar-empresa.sh empresa-b 192.168.0.101 192.168.0.102 192.168.0.103
```

### Máquina C — TetraCaronas

```bash
bash scripts/iniciar-empresa.sh empresa-c 192.168.0.101 192.168.0.102 192.168.0.103
```

Se um contêiner com o mesmo nome já existir, inicie-o com `docker start empresa-a`, `docker start empresa-b` ou `docker start empresa-c`. Se os IPs mudaram, remova somente o contêiner correspondente e execute novamente seu comando de criação; os dados montados nas pastas locais permanecem.

## 5. Conferir a comunicação entre as máquinas

Em cada máquina, confira seu servidor:

```bash
docker ps
docker logs empresa-a
```

Troque o nome no último comando por `empresa-b` ou `empresa-c` quando necessário.

Na máquina A, teste as portas das outras empresas:

```bash
nc -vz 192.168.0.102 8080
nc -vz 192.168.0.103 8080
nc -vz 192.168.0.102 9000
nc -vz 192.168.0.103 9000
```

Consulte a saúde individual e a visão do consórcio:

```bash
curl -s http://192.168.0.101:8080/api/v1/health
curl -s http://192.168.0.102:8080/api/v1/health
curl -s http://192.168.0.103:8080/api/v1/health
curl -s http://192.168.0.101:8080/api/v1/consortium/health
```

O resultado esperado é `online` para as três empresas e `disponivel: true` para os dois pares.

## 6. Abrir cada motorista e passageiro

Abra dois novos terminais em cada computador. O primeiro será o motorista e o segundo, o passageiro. Os clientes podem ser fechados com `Ctrl+C`.

### Clientes da máquina A

Motorista:

```bash
VAIJUNTO_SERVER=192.168.0.101:9000 go run ./motorista
```

Passageiro:

```bash
VAIJUNTO_SERVER=192.168.0.101:9000 go run ./cliente
```

### Clientes da máquina B

Motorista:

```bash
VAIJUNTO_SERVER=192.168.0.102:9000 go run ./motorista
```

Passageiro:

```bash
VAIJUNTO_SERVER=192.168.0.102:9000 go run ./cliente
```

### Clientes da máquina C

Motorista:

```bash
VAIJUNTO_SERVER=192.168.0.103:9000 go run ./motorista
```

Passageiro:

```bash
VAIJUNTO_SERVER=192.168.0.103:9000 go run ./cliente
```

Contas iniciais:

| Perfil | Usuário | Senha |
|---|---|---|
| Motorista | `motorista` | `1234` |
| Passageiro | `cliente` | `1234` |

Cada usuário deve se conectar somente ao servidor de sua própria empresa. A busca e a reserva entre empresas são realizadas pelos servidores.

## 7. Preparar o itinerário distribuído

Escolha uma data futura e execute uma vez, de qualquer máquina que tenha acesso às três portas TCP:

```bash
go run ./preparador \
  -data "2026-11-05" \
  -assentos 1 \
  -empresas "empresa-a=192.168.0.101:9000,empresa-b=192.168.0.102:9000,empresa-c=192.168.0.103:9000"
```

O preparador cria:

```text
CaronasGo:       Salvador -> Feira de Santana
LulaTransportes: Feira de Santana -> Vitória da Conquista
TetraCaronas:    Vitória da Conquista -> Montes Claros
```

No cliente de passageiro, faça login, busque de `Salvador` para `Montes Claros` na mesma data e confirme o itinerário. A reserva deve conter os três trechos.

## 8. Executar a simulação de concorrência

Como cada trecho possui um assento, várias tentativas simultâneas disputam a última vaga. Execute de uma máquina:

```bash
go run ./simulador \
  -data "2026-11-05" \
  -origem "Salvador" \
  -destino "Montes Claros" \
  -requisicoes 60 \
  -concorrencia 60 \
  -capacidade 1 \
  -espera-recuperacao 17s \
  -empresas "empresa-a=192.168.0.101:9000,empresa-b=192.168.0.102:9000,empresa-c=192.168.0.103:9000" \
  -rest "empresa-a=http://192.168.0.101:8080,empresa-b=http://192.168.0.102:8080,empresa-c=http://192.168.0.103:8080" \
  -json relatorios/concorrencia-3-maquinas.json \
  -csv relatorios/concorrencia-3-maquinas.csv
```

Resultado esperado:

- somente uma reserva recebe o último assento;
- as outras tentativas são recusadas sem reserva parcial;
- `preparadas` e `coordenacoes_pendentes` terminam em zero;
- o relatório termina aprovado e sem inconsistências.

Se o roteiro manual do item anterior já consumiu a vaga, prepare uma nova data futura antes de iniciar esta simulação.

## 9. Simular queda e recuperação

O parâmetro `-falha-servico` controla apenas contêineres do Docker local. Em três computadores, a queda deve ser feita manualmente.

1. Prepare um novo cenário, usando uma data ainda não utilizada:

```bash
go run ./preparador \
  -data "2026-11-06" \
  -assentos 1 \
  -empresas "empresa-a=192.168.0.101:9000,empresa-b=192.168.0.102:9000,empresa-c=192.168.0.103:9000"
```

2. Inicie o simulador sem injeção automática:

```bash
go run ./simulador \
  -data "2026-11-06" \
  -origem "Salvador" \
  -destino "Montes Claros" \
  -duracao 30s \
  -concorrencia 20 \
  -capacidade 1 \
  -espera-recuperacao 17s \
  -empresas "empresa-a=192.168.0.101:9000,empresa-b=192.168.0.102:9000,empresa-c=192.168.0.103:9000" \
  -rest "empresa-a=http://192.168.0.101:8080,empresa-b=http://192.168.0.102:8080,empresa-c=http://192.168.0.103:8080" \
  -json relatorios/falha-3-maquinas.json \
  -csv relatorios/falha-3-maquinas.csv
```

3. Durante a carga, na máquina B, interrompa a LulaTransportes:

```bash
docker stop empresa-b
```

4. Aguarde aproximadamente cinco segundos e restaure o serviço:

```bash
docker start empresa-b
```

5. Aguarde o simulador concluir a recuperação e a auditoria.

Durante a falha, buscas que não dependem da empresa B podem continuar, e buscas dependentes dela podem aparecer como parciais. Após a recuperação, não deve existir reserva parcialmente confirmada nem trabalho pendente.

## 10. Diagnóstico final

```bash
curl -s http://192.168.0.101:8080/api/v1/diagnostics/transactions
curl -s http://192.168.0.102:8080/api/v1/diagnostics/transactions
curl -s http://192.168.0.103:8080/api/v1/diagnostics/transactions
```

Critérios de aprovação:

- as três empresas respondem;
- `preparadas` é `0` nas três empresas;
- `coordenacoes_pendentes` é `0` nas três empresas;
- nenhuma carona possui mais reservas que sua capacidade;
- uma reserva confirmada aparece confirmada em todos os participantes;
- os arquivos JSON e CSV foram criados em `relatorios/`.

Para acompanhar os logs durante a apresentação:

```bash
docker logs -f empresa-a
```

Use o nome da empresa executada naquela máquina.

## 11. Encerrar o laboratório

Em cada máquina, interrompa seu contêiner:

```bash
docker stop empresa-a
```

Na máquina B use `empresa-b`; na máquina C, `empresa-c`. Os dados e logs continuam preservados nas pastas locais.

## Solução rápida de problemas

| Sintoma | Verificação |
|---|---|
| Cliente não conecta | Confira `VAIJUNTO_SERVER`, o IP, a porta TCP e `docker ps`. |
| Par aparece indisponível | Confira os IPs passados ao script, a porta REST, o firewall e teste com `nc` ou `curl`. |
| Contêiner encerra ao iniciar | Execute `docker logs <empresa>` e verifique IDs, URLs e volumes. |
| Busca não encontra a rota | Confirme a data e execute novamente o preparador para uma data sem reservas. |
| Simulação acusa capacidade excedida | Use `-capacidade` igual ao número passado em `-assentos`. |
| Teste de falha não recupera | Confirme que a empresa voltou a responder no endpoint `/api/v1/health` e aguarde pelo menos 17 segundos. |
| Porta já está em uso | Encerre o processo/contêiner que ocupa a porta ou escolha portas externas novas e atualize todos os endereços. |

