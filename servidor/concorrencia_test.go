package main

import (
	"fmt"
	"net"
	"sync"
	"testing"
	"time"

	"vaijunto/comum"
)

func TestVariosClientesDisputamMesmoAssento(t *testing.T) {
	estado := NovoEstado()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	go func() { _ = ExecutarServidor(listener, estado) }()

	endereco := listener.Addr().String()
	motorista := conectarELogar(t, endereco, "motorista", "1234")
	defer motorista.Fechar()
	publicacao, err := motorista.Enviar(comum.AcaoPublicarCarona, comum.PublicarCaronaDados{
		Rota: []string{"Salvador", "Feira de Santana"},
		Data: "2026-09-12", Horario: "08:00", Assentos: 1,
		PrecosCentavos: []int{3000}, DuracoesMin: []int{70},
	})
	if err != nil || !publicacao.Sucesso {
		t.Fatalf("não foi possível preparar a carona: %v - %s", err, publicacao.Mensagem)
	}

	const quantidade = 30
	clientes := make([]*comum.ConexaoTCP, 0, quantidade)
	itinerarios := make([]string, 0, quantidade)
	for i := 0; i < quantidade; i++ {
		cliente := conectarELogar(t, endereco, "cliente", "1234")
		clientes = append(clientes, cliente)
		resposta, err := cliente.Enviar(comum.AcaoBuscarItinerarios, comum.BuscarItinerariosDados{
			Origem: "Salvador", Destino: "Feira de Santana", Data: "2026-09-12",
		})
		if err != nil || !resposta.Sucesso {
			t.Fatalf("busca %d falhou: %v - %s", i, err, resposta.Mensagem)
		}
		resultado, err := comum.LerDados[comum.ResultadoBuscaItinerarios](resposta)
		if err != nil || len(resultado.Itinerarios) == 0 {
			t.Fatalf("busca %d não retornou opção: %v", i, err)
		}
		itinerarios = append(itinerarios, resultado.Itinerarios[0].ID)
	}
	defer func() {
		for _, cliente := range clientes {
			cliente.Fechar()
		}
	}()

	// Todos já encontraram a mesma vaga. Agora ficam esperando o mesmo sinal
	// para disputar a confirmação, em vez de fazer uma reserva de cada vez.
	inicio := make(chan struct{})
	resultados := make(chan bool, quantidade)
	erros := make(chan error, quantidade)
	var grupo sync.WaitGroup
	tempoInicial := time.Now()

	for i := 0; i < quantidade; i++ {
		grupo.Add(1)
		go func(indice int) {
			defer grupo.Done()
			<-inicio
			resposta, err := clientes[indice].Enviar(comum.AcaoConfirmarReserva, comum.IDDados{ID: itinerarios[indice]})
			if err != nil {
				erros <- fmt.Errorf("cliente %d: %w", indice, err)
				return
			}
			resultados <- resposta.Sucesso
		}(i)
	}
	close(inicio)
	grupo.Wait()
	close(resultados)
	close(erros)

	for err := range erros {
		t.Error(err)
	}
	sucessos := 0
	for sucesso := range resultados {
		if sucesso {
			sucessos++
		}
	}
	if sucessos != 1 {
		t.Fatalf("esperado exatamente 1 sucesso, obtido %d", sucessos)
	}
	// Mede o tempo do grupo inteiro; não é a latência individual de cada pedido.
	t.Logf("%d clientes concorrentes atendidos em %s", quantidade, time.Since(tempoInicial))
}

func conectarELogar(t *testing.T, endereco, usuario, senha string) *comum.ConexaoTCP {
	t.Helper()
	conexao, err := comum.Conectar(endereco)
	if err != nil {
		t.Fatal(err)
	}
	resposta, err := conexao.Enviar(comum.AcaoLogin, comum.LoginDados{Usuario: usuario, Senha: senha})
	if err != nil || !resposta.Sucesso {
		conexao.Fechar()
		t.Fatalf("login falhou: %v - %s", err, resposta.Mensagem)
	}
	login, err := comum.LerDados[comum.LoginResposta](resposta)
	if err != nil {
		conexao.Fechar()
		t.Fatal(err)
	}
	conexao.Token = login.Token
	return conexao
}
