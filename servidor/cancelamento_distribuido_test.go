package main

import (
	"path/filepath"
	"testing"
	"time"

	"net/http/httptest"

	"vaijunto/comum"
)

func TestCancelamentoDistribuidoPendenteSobreviveAoReinicio(t *testing.T) {
	diretorio := t.TempDir()
	usuarios := filepath.Join(diretorio, "usuarios.json")
	estadoJSON := filepath.Join(diretorio, "estado.json")
	estado, err := NovoEstadoEmpresaComArquivos("empresa-b", "LulaTransportes", usuarios, estadoJSON)
	if err != nil {
		t.Fatal(err)
	}
	estado.mutex.Lock()
	estado.cancelamentosPendentes["tx-pendente"] = &cancelamentoCaronaPendente{
		TransacaoID: "tx-pendente", CoordenadorID: "empresa-a", CaronaID: "CAR-001", AtualizadaEm: time.Now(),
	}
	err = estado.salvarEstadoSemLock()
	estado.mutex.Unlock()
	if err != nil {
		t.Fatal(err)
	}

	reiniciado, err := NovoEstadoEmpresaComArquivos("empresa-b", "LulaTransportes", usuarios, estadoJSON)
	if err != nil {
		t.Fatal(err)
	}
	pendente, existe := reiniciado.cancelamentosPendentes["tx-pendente"]
	if !existe || pendente.CoordenadorID != "empresa-a" || pendente.CaronaID != "CAR-001" {
		t.Fatalf("cancelamento pendente não foi recuperado: %#v", pendente)
	}
}

func TestMotoristaCancelaCaronaEInvalidaReservaDistribuida(t *testing.T) {
	coordenador, dona, terceira, reserva, caronaDona, servidorDona, servidorTerceira := prepararReservaDistribuidaParaCancelamento(t)
	defer servidorDona.Close()
	defer servidorTerceira.Close()

	servidorCoordenador := httptest.NewServer(NovaAPIREST(
		configuracaoRESTTeste("empresa-a", "CaronasGo", nil, time.Second), coordenador,
	))
	defer servidorCoordenador.Close()
	dona.ConfigurarBuscaDistribuida([]EmpresaRemota{{ID: "empresa-a", URLBase: servidorCoordenador.URL}}, time.Second)

	motorista := loginTeste(t, dona, "motorista", "1234")
	if err := dona.CancelarCarona(motorista.Token, caronaDona.ID); err != nil {
		t.Fatalf("cancelamento distribuído falhou: %v", err)
	}

	if !dona.caronas[caronaDona.ID].Cancelada {
		t.Fatal("a carona da empresa proprietária não foi cancelada")
	}
	if coordenador.reservas[reserva.ID].Status != comum.StatusCanceladaMotorista {
		t.Fatalf("status da reserva no coordenador: %s", coordenador.reservas[reserva.ID].Status)
	}
	for _, estado := range []*Estado{dona, terceira} {
		if vagas := vagasTrechoTeste(estado, "CAR-001", 0); vagas != 1 {
			t.Fatalf("empresa %s não devolveu o assento: vagas=%d", estado.empresaID, vagas)
		}
		status, err := estado.ConsultarTransacao(reserva.TransacaoID)
		if err != nil || status.Status != comum.StatusTransacaoAbortada {
			t.Fatalf("transação não foi abortada em %s: %#v, erro=%v", estado.empresaID, status, err)
		}
	}
	if len(dona.cancelamentosPendentes) != 0 {
		t.Fatalf("cancelamento entregue permaneceu pendente: %#v", dona.cancelamentosPendentes)
	}
}

func TestCancelamentoDaCaronaRetomaQuandoCoordenadorVolta(t *testing.T) {
	coordenador, dona, terceira, reserva, caronaDona, servidorDona, servidorTerceira := prepararReservaDistribuidaParaCancelamento(t)
	defer servidorDona.Close()
	defer servidorTerceira.Close()

	servidorCoordenador := httptest.NewServer(NovaAPIREST(
		configuracaoRESTTeste("empresa-a", "CaronasGo", nil, time.Second), coordenador,
	))
	urlFora := servidorCoordenador.URL
	servidorCoordenador.Close()
	dona.ConfigurarBuscaDistribuida([]EmpresaRemota{{ID: "empresa-a", URLBase: urlFora}}, 100*time.Millisecond)

	motorista := loginTeste(t, dona, "motorista", "1234")
	if err := dona.CancelarCarona(motorista.Token, caronaDona.ID); err == nil {
		t.Fatal("o cancelamento deveria informar que aguarda o coordenador")
	}
	if len(dona.cancelamentosPendentes) != 1 {
		t.Fatalf("invalidação não foi mantida para recuperação: %#v", dona.cancelamentosPendentes)
	}
	if coordenador.reservas[reserva.ID].Status != comum.StatusReservaConfirmada {
		t.Fatal("a reserva foi alterada sem o coordenador receber a invalidação")
	}

	servidorRecuperado := httptest.NewServer(NovaAPIREST(
		configuracaoRESTTeste("empresa-a", "CaronasGo", nil, time.Second), coordenador,
	))
	defer servidorRecuperado.Close()
	dona.ConfigurarBuscaDistribuida([]EmpresaRemota{{ID: "empresa-a", URLBase: servidorRecuperado.URL}}, 200*time.Millisecond)

	limite := time.Now().Add(3 * time.Second)
	for time.Now().Before(limite) {
		dona.mutex.Lock()
		pendentes := len(dona.cancelamentosPendentes)
		dona.mutex.Unlock()
		if pendentes == 0 {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if len(dona.cancelamentosPendentes) != 0 {
		t.Fatalf("invalidação não foi recuperada: %#v", dona.cancelamentosPendentes)
	}
	if coordenador.reservas[reserva.ID].Status != comum.StatusCanceladaMotorista {
		t.Fatalf("reserva não foi cancelada após recuperação: %s", coordenador.reservas[reserva.ID].Status)
	}
	if vagas := vagasTrechoTeste(terceira, "CAR-001", 0); vagas != 1 {
		t.Fatalf("assento remoto continuou ocupado após recuperação: %d", vagas)
	}
}

func prepararReservaDistribuidaParaCancelamento(t *testing.T) (
	coordenador, dona, terceira *Estado,
	reserva comum.Reserva,
	caronaDona comum.Carona,
	servidorDona, servidorTerceira *httptest.Server,
) {
	t.Helper()
	coordenador = NovoEstadoEmpresa("empresa-a", "CaronasGo")
	dona = NovoEstadoEmpresa("empresa-b", "LulaTransportes")
	terceira = NovoEstadoEmpresa("empresa-c", "TetraCaronas")
	motoristaDona := loginTeste(t, dona, "motorista", "1234")
	var err error
	caronaDona, err = dona.PublicarCarona(motoristaDona.Token, comum.PublicarCaronaDados{
		Rota: []string{"Salvador", "Feira de Santana"}, Data: "2026-10-20", Horario: "08:00",
		Assentos: 1, PrecosCentavos: []int{2500}, DuracoesMin: []int{60},
	})
	if err != nil {
		t.Fatal(err)
	}
	publicarTrechoTeste(t, terceira, "Feira de Santana", "Vitória da Conquista", "09:20", 120, 3500)

	servidorDona = httptest.NewServer(NovaAPIREST(
		configuracaoRESTTeste("empresa-b", "LulaTransportes", nil, time.Second), dona,
	))
	servidorTerceira = httptest.NewServer(NovaAPIREST(
		configuracaoRESTTeste("empresa-c", "TetraCaronas", nil, time.Second), terceira,
	))
	coordenador.ConfigurarBuscaDistribuida([]EmpresaRemota{
		{ID: "empresa-b", URLBase: servidorDona.URL},
		{ID: "empresa-c", URLBase: servidorTerceira.URL},
	}, time.Second)

	passageiro := loginTeste(t, coordenador, "cliente", "1234")
	resultado, err := coordenador.BuscarItinerariosDetalhado(passageiro.Token, comum.BuscarItinerariosDados{
		Origem: "Salvador", Destino: "Vitória da Conquista", Data: "2026-10-20",
	})
	if err != nil || len(resultado.Itinerarios) != 1 {
		t.Fatalf("itinerário distribuído não encontrado: erro=%v opções=%d", err, len(resultado.Itinerarios))
	}
	reserva, err = coordenador.ConfirmarReserva(passageiro.Token, resultado.Itinerarios[0].ID)
	if err != nil {
		t.Fatalf("reserva distribuída falhou: %v", err)
	}
	return
}
