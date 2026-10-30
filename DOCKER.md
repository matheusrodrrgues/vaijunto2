# Execução com Docker em três máquinas

Todos os computadores precisam estar na mesma rede e possuir Docker.

## Máquina do servidor

```sh
docker build -f Dockerfile.servidor -t vaijunto-servidor .
docker run --name vaijunto-servidor --rm -p 9000:9000 -v "${PWD}/dados:/app/dados" -v "${PWD}/logs:/app/logs" vaijunto-servidor
```

As contas ficam em `dados/usuarios.json` e os registros ficam em `logs/servidor.log`. Os volumes mantêm esses arquivos quando o contêiner for recriado.

Descubra o IPv4 dessa máquina com `ipconfig` no Windows ou `ip addr` no Linux. Também libere a porta TCP 9000 no firewall.

## Máquina do motorista

```sh
docker build -f Dockerfile.motorista -t vaijunto-motorista .
docker run --name vaijunto-motorista --rm -it -e VAIJUNTO_SERVER=192.168.0.10:9000 vaijunto-motorista
```

## Máquina do passageiro

```sh
docker build -f Dockerfile.cliente -t vaijunto-cliente .
docker run --name vaijunto-cliente --rm -it -e VAIJUNTO_SERVER=192.168.0.10:9000 vaijunto-cliente
```

Substitua `192.168.0.10` pelo IP da máquina do servidor.

## Testar a rede

```powershell
Test-NetConnection 192.168.0.10 -Port 9000
```

O campo `TcpTestSucceeded` precisa aparecer como `True`.
