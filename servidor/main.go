package main

import (
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
)

func main() { // Função principal do servidor
	configuracao, err := carregarConfiguracaoServidor()
	if err != nil {
		log.Fatal("Configuração inválida: ", err)
	}

	logAberto, err := configurarRegistro(configuracao.ArquivoLog)
	if err != nil {
		log.Fatal("Não foi possível abrir o arquivo de log: ", err)
	}
	defer logAberto.Close()

	estado, err := NovoEstadoEmpresaComArquivos( // Cria o estado da empresa a partir dos arquivos de dados
		configuracao.EmpresaID,
		configuracao.EmpresaNome,
		configuracao.ArquivoUsuarios,
		configuracao.ArquivoEstado,
	)
	if err != nil {
		log.Fatal("Não foi possível carregar os dados: ", err)
	}
	estado.ConfigurarBuscaDistribuida(configuracao.Pares, configuracao.TimeoutREST) // Configura a busca distribuída com os pares e o tempo limite

	listenerTCP, err := net.Listen("tcp", configuracao.EnderecoTCP) // Inicia o listener TCP para aceitar conexões de clientes
	if err != nil {
		log.Fatal("Não foi possível iniciar o servidor TCP: ", err)
	}
	defer listenerTCP.Close()

	listenerREST, err := net.Listen("tcp", configuracao.EnderecoREST)
	if err != nil {
		log.Fatal("Não foi possível iniciar a API REST: ", err)
	}
	defer listenerREST.Close()
	servidorHTTP := novoServidorHTTP(configuracao, estado) // Cria o servidor HTTP para a API REST

	_, portaTCP, _ := net.SplitHostPort(listenerTCP.Addr().String())   // Obtém a porta TCP
	_, portaREST, _ := net.SplitHostPort(listenerREST.Addr().String()) // Obtém a porta REST
	fmt.Println()
	fmt.Println("====================================")
	fmt.Println("      SERVIDOR DE EMPRESA")
	fmt.Println("====================================")
	fmt.Println("Empresa  :", configuracao.EmpresaNome)
	fmt.Println("ID       :", configuracao.EmpresaID)
	fmt.Println("Status   : ONLINE")
	fmt.Println("Porta TCP:", portaTCP)
	fmt.Println("Porta REST:", portaREST)
	fmt.Println("Pares REST:", len(configuracao.Pares))
	fmt.Println("Usuários :", configuracao.ArquivoUsuarios)
	fmt.Println("Estado   :", configuracao.ArquivoEstado)
	fmt.Println("Logs     :", configuracao.ArquivoLog)
	fmt.Println("------------------------------------")
	fmt.Println("Aguardando conexões...")
	fmt.Println()
	registrar("SERVIDOR_INICIADO empresa=%s nome=%q porta_tcp=%s porta_rest=%s pares=%d",
		configuracao.EmpresaID, configuracao.EmpresaNome, portaTCP, portaREST, len(configuracao.Pares))

	erros := make(chan error, 2) // canal para receber erros de goroutines
	go func() {
		erros <- ExecutarServidor(listenerTCP, estado)
	}()
	go func() {
		err := servidorHTTP.Serve(listenerREST) // Inicia o servidor HTTP para a API REST
		if errors.Is(err, http.ErrServerClosed) {
			err = nil
		}
		erros <- err
	}()

	if err := <-erros; err != nil {
		log.Fatal(err)
	}
}
