package main

import (
	"path/filepath"
	"sync"
	"testing"
	"time"

	"net/http/httptest"

	"vaijunto/comum"
)

func TestPassageirosDeEmpresasDiferentesDisputamUltimoAssentoSimultaneamente(t *testing.T) {
	dona := NovoEstadoEmpresa("empresa-b", "LulaTransportes")
	publicarTrechoTeste(t, dona, "Salvador", "Feira de Santana", "08:00", 60, 2500)
	servidorDona := httptest.NewServer(NovaAPIREST(
		configuracaoRESTTeste("empresa-b", "LulaTransportes", nil, time.Second), dona,
	))
	defer servidorDona.Close()

	coordenadores := []*Estado{
		NovoEstadoEmpresa("empresa-a", "CaronasGo"),
		NovoEstadoEmpresa("empresa-c", "TetraCaronas"),
	}
	for _, estado := range coordenadores {
		estado.ConfigurarBuscaDistribuida([]EmpresaRemota{{ID: "empresa-b", URLBase: servidorDona.URL}}, time.Second)
	}

	tokens := make([]string, len(coordenadores))
	itinerarios := make([]string, len(coordenadores))
	for i, estado := range coordenadores {
		passageiro := loginTeste(t, estado, "cliente", "1234")
		tokens[i] = passageiro.Token
		resultado, err := estado.BuscarItinerariosDetalhado(passageiro.Token, comum.BuscarItinerariosDados{
			Origem: "Salvador", Destino: "Feira de Santana", Data: "2026-10-20",
		})
		if err != nil || len(resultado.Itinerarios) != 1 {
			t.Fatalf("preparação da disputa %d falhou: %v, opções=%d", i, err, len(resultado.Itinerarios))
		}
		itinerarios[i] = resultado.Itinerarios[0].ID
	}

	inicio := make(chan struct{})
	resultados := make(chan error, len(coordenadores))
	var grupo sync.WaitGroup
	for i, estado := range coordenadores {
		grupo.Add(1)
		go func(i int, estado *Estado) {
			defer grupo.Done()
			<-inicio
			_, err := estado.ConfirmarReserva(tokens[i], itinerarios[i])
			resultados <- err
		}(i, estado)
	}
	close(inicio)
	grupo.Wait()
	close(resultados)

	sucessos := 0
	for err := range resultados {
		if err == nil {
			sucessos++
		}
	}
	if sucessos != 1 {
		t.Fatalf("esperava exatamente um vencedor simultâneo, recebeu %d", sucessos)
	}
	if vagas := dona.caronas["CAR-001"].DisponiveisPorTrecho[0]; vagas != 0 {
		t.Fatalf("último assento deveria ter sido vendido uma única vez: vagas=%d", vagas)
	}
	reservas := len(coordenadores[0].reservas) + len(coordenadores[1].reservas)
	if reservas != 1 {
		t.Fatalf("esperava uma reserva no consórcio, recebeu %d", reservas)
	}
}

func TestFalhaNaPreparacaoRemotaDesfazTrechoLocal(t *testing.T) {
	local := NovoEstadoEmpresa("empresa-a", "CaronasGo")
	remota := NovoEstadoEmpresa("empresa-b", "LulaTransportes")
	publicarTrechoTeste(t, local, "Salvador", "Feira de Santana", "08:00", 60, 2500)
	publicarTrechoTeste(t, remota, "Feira de Santana", "Vitória da Conquista", "09:20", 120, 3500)
	servidorRemoto := httptest.NewServer(NovaAPIREST(
		configuracaoRESTTeste("empresa-b", "LulaTransportes", nil, time.Second), remota,
	))
	defer servidorRemoto.Close()
	local.ConfigurarBuscaDistribuida([]EmpresaRemota{{ID: "empresa-b", URLBase: servidorRemoto.URL}}, time.Second)

	passageiro := loginTeste(t, local, "cliente", "1234")
	resultado, err := local.BuscarItinerariosDetalhado(passageiro.Token, comum.BuscarItinerariosDados{
		Origem: "Salvador", Destino: "Vitória da Conquista", Data: "2026-10-20",
	})
	if err != nil || len(resultado.Itinerarios) != 1 {
		t.Fatalf("itinerário não encontrado: %v", err)
	}

	// Outra transação ocupa o trecho remoto depois da busca, simulando a
	// corrida entre consulta e confirmação.
	_, err = remota.PrepararTransacao(comum.PrepararReservaREST{
		TransacaoID: "tx-concorrente", CoordenadorID: "empresa-c", DuracaoBloqueioMS: 5000,
		Trechos: []comum.TrechoTransacaoREST{{CaronaID: "CAR-001", Indice: 0}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = remota.ConfirmarTransacao("tx-concorrente"); err != nil {
		t.Fatal(err)
	}

	if _, err = local.ConfirmarReserva(passageiro.Token, resultado.Itinerarios[0].ID); err == nil {
		t.Fatal("a reserva deveria falhar porque o trecho remoto ficou sem vaga")
	}
	if vagas := local.caronas["CAR-001"].DisponiveisPorTrecho[0]; vagas != 1 {
		t.Fatalf("trecho local ficou reservado pela metade: vagas=%d", vagas)
	}
}

func TestQuedaDoServidorRemotoNaoDeixaAssentoLocalBloqueado(t *testing.T) {
	local := NovoEstadoEmpresa("empresa-a", "CaronasGo")
	remota := NovoEstadoEmpresa("empresa-b", "LulaTransportes")
	publicarTrechoTeste(t, local, "Salvador", "Feira de Santana", "08:00", 60, 2500)
	publicarTrechoTeste(t, remota, "Feira de Santana", "Vitória da Conquista", "09:20", 120, 3500)
	servidorRemoto := httptest.NewServer(NovaAPIREST(
		configuracaoRESTTeste("empresa-b", "LulaTransportes", nil, time.Second), remota,
	))
	local.ConfigurarBuscaDistribuida([]EmpresaRemota{{ID: "empresa-b", URLBase: servidorRemoto.URL}}, 100*time.Millisecond)
	passageiro := loginTeste(t, local, "cliente", "1234")
	resultado, err := local.BuscarItinerariosDetalhado(passageiro.Token, comum.BuscarItinerariosDados{
		Origem: "Salvador", Destino: "Vitória da Conquista", Data: "2026-10-20",
	})
	if err != nil || len(resultado.Itinerarios) != 1 {
		servidorRemoto.Close()
		t.Fatalf("itinerário não encontrado: %v", err)
	}

	servidorRemoto.Close()
	if _, err := local.ConfirmarReserva(passageiro.Token, resultado.Itinerarios[0].ID); err == nil {
		t.Fatal("a reserva deveria falhar com o servidor remoto fora do ar")
	}
	if vagas := vagasTrechoTeste(local, "CAR-001", 0); vagas != 1 {
		t.Fatalf("queda remota deixou o trecho local bloqueado: vagas=%d", vagas)
	}
}

func TestPreparacaoAbandonadaExpiraELiberaAssento(t *testing.T) {
	estado := NovoEstadoEmpresa("empresa-a", "CaronasGo")
	publicarTrechoTeste(t, estado, "Salvador", "Feira de Santana", "08:00", 60, 2500)
	_, err := estado.PrepararTransacao(comum.PrepararReservaREST{
		TransacaoID: "tx-abandonada", CoordenadorID: "empresa-b", DuracaoBloqueioMS: 30,
		Trechos: []comum.TrechoTransacaoREST{{CaronaID: "CAR-001", Indice: 0}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if vagas := vagasTrechoTeste(estado, "CAR-001", 0); vagas != 0 {
		t.Fatalf("preparação não bloqueou o assento: vagas=%d", vagas)
	}

	limite := time.Now().Add(time.Second)
	for time.Now().Before(limite) && vagasTrechoTeste(estado, "CAR-001", 0) == 0 {
		time.Sleep(10 * time.Millisecond)
	}
	if vagas := vagasTrechoTeste(estado, "CAR-001", 0); vagas != 1 {
		t.Fatalf("assento permaneceu bloqueado após a expiração: vagas=%d", vagas)
	}
	status, err := estado.ConsultarTransacao("tx-abandonada")
	if err != nil || status.Status != comum.StatusTransacaoAbortada {
		t.Fatalf("estado final inesperado: %#v, erro=%v", status, err)
	}
}

func TestPreparacaoRepetidaEIdempotente(t *testing.T) {
	estado := NovoEstadoEmpresa("empresa-a", "CaronasGo")
	publicarTrechoTeste(t, estado, "Salvador", "Feira de Santana", "08:00", 60, 2500)
	dados := comum.PrepararReservaREST{
		TransacaoID: "tx-idempotente", CoordenadorID: "empresa-b", DuracaoBloqueioMS: 5000,
		Trechos: []comum.TrechoTransacaoREST{{CaronaID: "CAR-001", Indice: 0}},
	}
	if _, err := estado.PrepararTransacao(dados); err != nil {
		t.Fatal(err)
	}
	if _, err := estado.PrepararTransacao(dados); err != nil {
		t.Fatalf("repetição idempotente foi recusada: %v", err)
	}
	if vagas := vagasTrechoTeste(estado, "CAR-001", 0); vagas != 0 {
		t.Fatalf("preparação repetida descontou quantidade incorreta: vagas=%d", vagas)
	}
	if _, err := estado.ConfirmarTransacao(dados.TransacaoID); err != nil {
		t.Fatal(err)
	}
	if _, err := estado.ConfirmarTransacao(dados.TransacaoID); err != nil {
		t.Fatalf("confirmação repetida foi recusada: %v", err)
	}
}

func TestPreparacaoSobreviveAReinicioDoParticipante(t *testing.T) {
	diretorio := t.TempDir()
	usuarios := filepath.Join(diretorio, "usuarios.json")
	estadoJSON := filepath.Join(diretorio, "estado.json")
	estado, err := NovoEstadoEmpresaComArquivos("empresa-a", "CaronasGo", usuarios, estadoJSON)
	if err != nil {
		t.Fatal(err)
	}
	publicarTrechoTeste(t, estado, "Salvador", "Feira de Santana", "08:00", 60, 2500)
	_, err = estado.PrepararTransacao(comum.PrepararReservaREST{
		TransacaoID: "tx-persistida", CoordenadorID: "empresa-b", DuracaoBloqueioMS: 30000,
		Trechos: []comum.TrechoTransacaoREST{{CaronaID: "CAR-001", Indice: 0}},
	})
	if err != nil {
		t.Fatal(err)
	}

	reiniciado, err := NovoEstadoEmpresaComArquivos("empresa-a", "CaronasGo", usuarios, estadoJSON)
	if err != nil {
		t.Fatal(err)
	}
	status, err := reiniciado.ConsultarTransacao("tx-persistida")
	if err != nil || status.Status != comum.StatusTransacaoPreparada {
		t.Fatalf("preparação não foi recuperada: %#v, erro=%v", status, err)
	}
	if vagas := vagasTrechoTeste(reiniciado, "CAR-001", 0); vagas != 0 {
		t.Fatalf("assento preparado reapareceu após reinício: vagas=%d", vagas)
	}
	if _, err := reiniciado.AbortarTransacao("tx-persistida"); err != nil {
		t.Fatal(err)
	}
	if vagas := vagasTrechoTeste(reiniciado, "CAR-001", 0); vagas != 1 {
		t.Fatalf("aborto após reinício não liberou o assento: vagas=%d", vagas)
	}
}

func vagasTrechoTeste(estado *Estado, caronaID string, indice int) int {
	estado.mutex.Lock()
	defer estado.mutex.Unlock()
	return estado.caronas[caronaID].DisponiveisPorTrecho[indice]
}
