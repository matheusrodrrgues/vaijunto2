package main

import (
	"fmt"
	"log"
	"net"
)

func main() {
	configuracao, err := carregarConfiguracaoServidor()
	if err != nil {
		log.Fatal("Configuração inválida: ", err)
	}

	logAberto, err := configurarRegistro(configuracao.ArquivoLog)
	if err != nil {
		log.Fatal("Não foi possível abrir o arquivo de log: ", err)
	}
	defer logAberto.Close()

	estado, err := NovoEstadoEmpresaComArquivos(
		configuracao.EmpresaID,
		configuracao.EmpresaNome,
		configuracao.ArquivoUsuarios,
		configuracao.ArquivoEstado,
	)
	if err != nil {
		log.Fatal("Não foi possível carregar os dados: ", err)
	}

	listener, err := net.Listen("tcp", configuracao.EnderecoTCP)
	if err != nil {
		log.Fatal("Não foi possível iniciar o servidor: ", err)
	}
	defer listener.Close()

	_, porta, _ := net.SplitHostPort(listener.Addr().String())
	fmt.Println()
	fmt.Println("====================================")
	fmt.Println("      SERVIDOR DE EMPRESA")
	fmt.Println("====================================")
	fmt.Println("Empresa  :", configuracao.EmpresaNome)
	fmt.Println("ID       :", configuracao.EmpresaID)
	fmt.Println("Status   : ONLINE")
	fmt.Println("Porta TCP:", porta)
	fmt.Println("Usuários :", configuracao.ArquivoUsuarios)
	fmt.Println("Estado   :", configuracao.ArquivoEstado)
	fmt.Println("Logs     :", configuracao.ArquivoLog)
	fmt.Println("------------------------------------")
	fmt.Println("Aguardando conexões...")
	fmt.Println()
	registrar("SERVIDOR_INICIADO empresa=%s nome=%q porta=%s", configuracao.EmpresaID, configuracao.EmpresaNome, porta)

	if err := ExecutarServidor(listener, estado); err != nil {
		log.Fatal(err)
	}
}
