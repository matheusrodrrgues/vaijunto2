package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"vaijunto/comum"
)

func TestBuscaDistribuidaCombinaTresEmpresas(t *testing.T) {
	estadoA := NovoEstadoEmpresa("empresa-a", "Empresa A")
	estadoB := NovoEstadoEmpresa("empresa-b", "Empresa B")
	estadoC := NovoEstadoEmpresa("empresa-c", "Empresa C")
	publicarTrechoTeste(t, estadoA, "Salvador", "Feira de Santana", "08:00", 60, 2500)
	publicarTrechoTeste(t, estadoB, "Feira de Santana", "Vitória da Conquista", "09:20", 120, 3500)
	publicarTrechoTeste(t, estadoC, "Vitória da Conquista", "Montes Claros", "11:40", 180, 4500)

	servidorB := httptest.NewServer(NovaAPIREST(configuracaoRESTTeste("empresa-b", "Empresa B", nil, time.Second), estadoB))
	defer servidorB.Close()
	servidorC := httptest.NewServer(NovaAPIREST(configuracaoRESTTeste("empresa-c", "Empresa C", nil, time.Second), estadoC))
	defer servidorC.Close()
	estadoA.ConfigurarBuscaDistribuida([]EmpresaRemota{
		{ID: "empresa-b", URLBase: servidorB.URL},
		{ID: "empresa-c", URLBase: servidorC.URL},
	}, time.Second)

	passageiro := loginTeste(t, estadoA, "cliente", "1234")
	resultado, err := estadoA.BuscarItinerariosDetalhado(passageiro.Token, comum.BuscarItinerariosDados{
		Origem: "Salvador", Destino: "Montes Claros", Data: "2026-10-20",
	})
	if err != nil {
		t.Fatal(err)
	}
	if resultado.BuscaParcial || len(resultado.EmpresasIndisponiveis) != 0 {
		t.Fatalf("busca completa foi marcada como parcial: %#v", resultado.EmpresasIndisponiveis)
	}
	if len(resultado.Itinerarios) != 1 || len(resultado.Itinerarios[0].Trechos) != 3 {
		t.Fatalf("itinerário de três empresas não encontrado: %#v", resultado.Itinerarios)
	}
	esperadas := []string{"empresa-a", "empresa-b", "empresa-c"}
	for i, trecho := range resultado.Itinerarios[0].Trechos {
		if trecho.EmpresaID != esperadas[i] {
			t.Fatalf("empresa do trecho %d: recebeu %q, esperava %q", i, trecho.EmpresaID, esperadas[i])
		}
	}
	reserva, err := estadoA.ConfirmarReserva(passageiro.Token, resultado.Itinerarios[0].ID)
	if err != nil {
		t.Fatalf("confirmação distribuída falhou: %v", err)
	}
	if reserva.TransacaoID == "" || reserva.Status != comum.StatusReservaConfirmada {
		t.Fatalf("reserva distribuída inválida: %#v", reserva)
	}
	for _, estado := range []*Estado{estadoA, estadoB, estadoC} {
		if estado.caronas["CAR-001"].DisponiveisPorTrecho[0] != 0 {
			t.Fatalf("empresa %s não confirmou seu trecho", estado.empresaID)
		}
	}
	if err := estadoA.CancelarReserva(passageiro.Token, reserva.ID); err != nil {
		t.Fatalf("cancelamento distribuído falhou: %v", err)
	}
	for _, estado := range []*Estado{estadoA, estadoB, estadoC} {
		if estado.caronas["CAR-001"].DisponiveisPorTrecho[0] != 1 {
			t.Fatalf("empresa %s não liberou seu trecho", estado.empresaID)
		}
	}
}

func TestBuscaDistribuidaContinuaParcialComServidorFora(t *testing.T) {
	estadoA := NovoEstadoEmpresa("empresa-a", "Empresa A")
	estadoB := NovoEstadoEmpresa("empresa-b", "Empresa B")
	publicarTrechoTeste(t, estadoA, "Salvador", "Feira de Santana", "08:00", 60, 2500)
	publicarTrechoTeste(t, estadoB, "Feira de Santana", "Vitória da Conquista", "09:20", 120, 3500)

	servidorB := httptest.NewServer(NovaAPIREST(configuracaoRESTTeste("empresa-b", "Empresa B", nil, time.Second), estadoB))
	defer servidorB.Close()
	servidorC := httptest.NewServer(NovaAPIREST(configuracaoRESTTeste("empresa-c", "Empresa C", nil, time.Second), NovoEstadoEmpresa("empresa-c", "Empresa C")))
	urlC := servidorC.URL
	servidorC.Close()
	estadoA.ConfigurarBuscaDistribuida([]EmpresaRemota{
		{ID: "empresa-b", URLBase: servidorB.URL},
		{ID: "empresa-c", URLBase: urlC},
	}, 200*time.Millisecond)

	passageiro := loginTeste(t, estadoA, "cliente", "1234")
	resultado, err := estadoA.BuscarItinerariosDetalhado(passageiro.Token, comum.BuscarItinerariosDados{
		Origem: "Salvador", Destino: "Vitória da Conquista", Data: "2026-10-20",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !resultado.BuscaParcial || len(resultado.EmpresasIndisponiveis) != 1 || resultado.EmpresasIndisponiveis[0] != "empresa-c" {
		t.Fatalf("indisponibilidade não informada corretamente: %#v", resultado)
	}
	if len(resultado.Itinerarios) != 1 || len(resultado.Itinerarios[0].Trechos) != 2 {
		t.Fatalf("resultado que não dependia da empresa C foi perdido: %#v", resultado.Itinerarios)
	}
}

func TestAPIRestBuscaSomenteTrechosLocaisDisponiveis(t *testing.T) {
	estado := NovoEstadoEmpresa("empresa-a", "Empresa A")
	publicarTrechoTeste(t, estado, "Salvador", "Feira de Santana", "08:00", 60, 2500)
	api := NovaAPIREST(configuracaoRESTTeste("empresa-a", "Empresa A", nil, time.Second), estado)
	resposta := httptest.NewRecorder()
	api.ServeHTTP(resposta, httptest.NewRequest(http.MethodGet, "/api/v1/rides/search?date=2026-10-20", nil))

	if resposta.Code != http.StatusOK {
		t.Fatalf("status inesperado: %d", resposta.Code)
	}
	var catalogo comum.RespostaBuscaTrechosREST
	if err := json.NewDecoder(resposta.Body).Decode(&catalogo); err != nil {
		t.Fatal(err)
	}
	if catalogo.EmpresaID != "empresa-a" || len(catalogo.Trechos) != 1 {
		t.Fatalf("catálogo inesperado: %#v", catalogo)
	}
	if catalogo.Trechos[0].EmpresaID != "empresa-a" || catalogo.Trechos[0].Vagas != 1 {
		t.Fatalf("trecho local inválido: %#v", catalogo.Trechos[0])
	}
}

func publicarTrechoTeste(t *testing.T, estado *Estado, origem, destino, horario string, duracao, preco int) {
	t.Helper()
	motorista := loginTeste(t, estado, "motorista", "1234")
	_, err := estado.PublicarCarona(motorista.Token, comum.PublicarCaronaDados{
		Rota: []string{origem, destino}, Data: "2026-10-20", Horario: horario,
		Assentos: 1, PrecosCentavos: []int{preco}, DuracoesMin: []int{duracao},
	})
	if err != nil {
		t.Fatal(err)
	}
}
