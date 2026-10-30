package main

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"vaijunto/comum"
)

func TestCoordenadorRetomaCommitDepoisDeReiniciar(t *testing.T) {
	diretorio := t.TempDir()
	usuarios := filepath.Join(diretorio, "usuarios.json")
	estadoJSON := filepath.Join(diretorio, "estado.json")
	coordenador, err := NovoEstadoEmpresaComArquivos("empresa-a", "CaronasGo", usuarios, estadoJSON)
	if err != nil {
		t.Fatal(err)
	}
	participante := NovoEstadoEmpresa("empresa-b", "LulaTransportes")
	publicarTrechoTeste(t, coordenador, "Salvador", "Feira de Santana", "08:00", 60, 2500)
	publicarTrechoTeste(t, participante, "Feira de Santana", "Vitória da Conquista", "09:20", 120, 3500)
	servidorParticipante := httptest.NewServer(NovaAPIREST(
		configuracaoRESTTeste("empresa-b", "LulaTransportes", nil, time.Second), participante,
	))
	defer servidorParticipante.Close()
	pares := []EmpresaRemota{{ID: "empresa-b", URLBase: servidorParticipante.URL}}
	coordenador.ConfigurarBuscaDistribuida(pares, 200*time.Millisecond)

	passageiro := loginTeste(t, coordenador, "cliente", "1234")
	resultado, err := coordenador.BuscarItinerariosDetalhado(passageiro.Token, comum.BuscarItinerariosDados{
		Origem: "Salvador", Destino: "Vitória da Conquista", Data: "2026-10-20",
	})
	if err != nil || len(resultado.Itinerarios) != 1 {
		t.Fatalf("itinerário não encontrado: %v", err)
	}
	coordenador.mutex.Lock()
	coordenador.ganchoAposCommit = func(confirmados int) error {
		if confirmados == 1 {
			return errQuedaSimulada
		}
		return nil
	}
	coordenador.mutex.Unlock()

	if _, err := coordenador.ConfirmarReserva(passageiro.Token, resultado.Itinerarios[0].ID); !errors.Is(err, errQuedaSimulada) {
		t.Fatalf("a queda deveria ocorrer depois do primeiro commit: %v", err)
	}
	if vagas := vagasTrechoTeste(coordenador, "CAR-001", 0); vagas != 0 {
		t.Fatalf("primeiro participante não confirmou antes da queda: vagas=%d", vagas)
	}
	if vagas := vagasTrechoTeste(participante, "CAR-001", 0); vagas != 0 {
		// O trecho está preparado e, por isso, também aparece indisponível.
		t.Fatalf("segundo participante não preservou a preparação: vagas=%d", vagas)
	}

	reiniciado, err := NovoEstadoEmpresaComArquivos("empresa-a", "CaronasGo", usuarios, estadoJSON)
	if err != nil {
		t.Fatal(err)
	}
	reiniciado.ConfigurarBuscaDistribuida(pares, 200*time.Millisecond)
	esperarCondicao(t, 3*time.Second, func() bool {
		reiniciado.mutex.Lock()
		defer reiniciado.mutex.Unlock()
		return len(reiniciado.reservas) == 1
	}, "o coordenador reiniciado não concluiu a reserva")

	if vagas := vagasTrechoTeste(reiniciado, "CAR-001", 0); vagas != 0 {
		t.Fatalf("trecho local foi perdido na recuperação: vagas=%d", vagas)
	}
	if vagas := vagasTrechoTeste(participante, "CAR-001", 0); vagas != 0 {
		t.Fatalf("trecho remoto não foi confirmado na recuperação: vagas=%d", vagas)
	}
	reiniciado.mutex.Lock()
	for _, coordenacao := range reiniciado.coordenacoes {
		if coordenacao.Status != statusCoordenacaoConcluida {
			t.Fatalf("estado final da coordenação: %s", coordenacao.Status)
		}
	}
	reiniciado.mutex.Unlock()
}

func TestRespostaDeCommitPerdidaERecuperadaPorConsultaDeEstado(t *testing.T) {
	participante := NovoEstadoEmpresa("empresa-b", "LulaTransportes")
	publicarTrechoTeste(t, participante, "Salvador", "Feira de Santana", "08:00", 60, 2500)
	configuracao := configuracaoRESTTeste("empresa-b", "LulaTransportes", nil, time.Second)
	api := NovaAPIREST(configuracao, participante)
	servidor := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/reservations/commit" {
			api.ServeHTTP(w, r)
			return
		}
		gravador := httptest.NewRecorder()
		api.ServeHTTP(gravador, r)
		// A operação já foi aplicada, mas a resposta demora mais que o timeout.
		time.Sleep(120 * time.Millisecond)
		for chave, valores := range gravador.Header() {
			for _, valor := range valores {
				w.Header().Add(chave, valor)
			}
		}
		w.WriteHeader(gravador.Code)
		_, _ = w.Write(gravador.Body.Bytes())
	}))
	defer servidor.Close()

	coordenador := NovoEstadoEmpresa("empresa-a", "CaronasGo")
	coordenador.ConfigurarBuscaDistribuida(
		[]EmpresaRemota{{ID: "empresa-b", URLBase: servidor.URL}}, 40*time.Millisecond,
	)
	passageiro := loginTeste(t, coordenador, "cliente", "1234")
	resultado, err := coordenador.BuscarItinerariosDetalhado(passageiro.Token, comum.BuscarItinerariosDados{
		Origem: "Salvador", Destino: "Feira de Santana", Data: "2026-10-20",
	})
	if err != nil || len(resultado.Itinerarios) != 1 {
		t.Fatalf("itinerário não encontrado: %v", err)
	}
	reserva, err := coordenador.ConfirmarReserva(passageiro.Token, resultado.Itinerarios[0].ID)
	if err != nil {
		t.Fatalf("a consulta de estado não recuperou a resposta perdida: %v", err)
	}
	if reserva.Status != comum.StatusReservaConfirmada || vagasTrechoTeste(participante, "CAR-001", 0) != 0 {
		t.Fatalf("reserva final inconsistente: %#v", reserva)
	}
}

func TestRecuperacaoCompensaCommitQuandoPreparacaoRemotaExpirou(t *testing.T) {
	diretorio := t.TempDir()
	usuarios := filepath.Join(diretorio, "usuarios.json")
	estadoJSON := filepath.Join(diretorio, "estado.json")
	coordenador, err := NovoEstadoEmpresaComArquivos("empresa-a", "CaronasGo", usuarios, estadoJSON)
	if err != nil {
		t.Fatal(err)
	}
	participante := NovoEstadoEmpresa("empresa-b", "LulaTransportes")
	publicarTrechoTeste(t, coordenador, "Salvador", "Feira de Santana", "08:00", 60, 2500)
	publicarTrechoTeste(t, participante, "Feira de Santana", "Vitória da Conquista", "09:20", 120, 3500)
	servidorParticipante := httptest.NewServer(NovaAPIREST(
		configuracaoRESTTeste("empresa-b", "LulaTransportes", nil, time.Second), participante,
	))
	defer servidorParticipante.Close()
	pares := []EmpresaRemota{{ID: "empresa-b", URLBase: servidorParticipante.URL}}
	coordenador.ConfigurarBuscaDistribuida(pares, 200*time.Millisecond)
	passageiro := loginTeste(t, coordenador, "cliente", "1234")
	resultado, err := coordenador.BuscarItinerariosDetalhado(passageiro.Token, comum.BuscarItinerariosDados{
		Origem: "Salvador", Destino: "Vitória da Conquista", Data: "2026-10-20",
	})
	if err != nil || len(resultado.Itinerarios) != 1 {
		t.Fatalf("itinerário não encontrado: %v", err)
	}
	coordenador.mutex.Lock()
	coordenador.ganchoAposCommit = func(confirmados int) error {
		if confirmados == 1 {
			return errQuedaSimulada
		}
		return nil
	}
	coordenador.mutex.Unlock()
	_, _ = coordenador.ConfirmarReserva(passageiro.Token, resultado.Itinerarios[0].ID)

	coordenador.mutex.Lock()
	var transacaoID string
	for id := range coordenador.coordenacoes {
		transacaoID = id
	}
	coordenador.mutex.Unlock()
	participante.mutex.Lock()
	participante.transacoes[transacaoID].ExpiraEm = time.Now().Add(-time.Second)
	participante.mutex.Unlock()
	participante.expirarTransacao(transacaoID)

	reiniciado, err := NovoEstadoEmpresaComArquivos("empresa-a", "CaronasGo", usuarios, estadoJSON)
	if err != nil {
		t.Fatal(err)
	}
	reiniciado.ConfigurarBuscaDistribuida(pares, 200*time.Millisecond)
	esperarCondicao(t, 3*time.Second, func() bool {
		reiniciado.mutex.Lock()
		defer reiniciado.mutex.Unlock()
		registro := reiniciado.coordenacoes[transacaoID]
		return registro != nil && registro.Status == statusCoordenacaoAbortada
	}, "a recuperação não decidiu pelo aborto integral")

	if vagasTrechoTeste(reiniciado, "CAR-001", 0) != 1 || vagasTrechoTeste(participante, "CAR-001", 0) != 1 {
		t.Fatal("a compensação não devolveu todos os assentos")
	}
	reiniciado.mutex.Lock()
	quantidadeReservas := len(reiniciado.reservas)
	reiniciado.mutex.Unlock()
	if quantidadeReservas != 0 {
		t.Fatalf("uma reserva parcial foi mantida: %d", quantidadeReservas)
	}
}

func TestRecuperacaoContinuaTentandoAteParticipanteVoltar(t *testing.T) {
	diretorio := t.TempDir()
	usuarios := filepath.Join(diretorio, "usuarios.json")
	estadoJSON := filepath.Join(diretorio, "estado.json")
	coordenador, err := NovoEstadoEmpresaComArquivos("empresa-a", "CaronasGo", usuarios, estadoJSON)
	if err != nil {
		t.Fatal(err)
	}
	participante := NovoEstadoEmpresa("empresa-b", "LulaTransportes")
	publicarTrechoTeste(t, coordenador, "Salvador", "Feira de Santana", "08:00", 60, 2500)
	publicarTrechoTeste(t, participante, "Feira de Santana", "Vitória da Conquista", "09:20", 120, 3500)
	apiParticipante := NovaAPIREST(
		configuracaoRESTTeste("empresa-b", "LulaTransportes", nil, time.Second), participante,
	)
	var disponivel atomic.Bool
	disponivel.Store(true)
	servidorParticipante := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !disponivel.Load() && strings.HasPrefix(r.URL.Path, "/api/v1/reservations/") {
			http.Error(w, "indisponível", http.StatusServiceUnavailable)
			return
		}
		apiParticipante.ServeHTTP(w, r)
	}))
	defer servidorParticipante.Close()
	pares := []EmpresaRemota{{ID: "empresa-b", URLBase: servidorParticipante.URL}}
	coordenador.ConfigurarBuscaDistribuida(pares, 100*time.Millisecond)
	passageiro := loginTeste(t, coordenador, "cliente", "1234")
	resultado, err := coordenador.BuscarItinerariosDetalhado(passageiro.Token, comum.BuscarItinerariosDados{
		Origem: "Salvador", Destino: "Vitória da Conquista", Data: "2026-10-20",
	})
	if err != nil || len(resultado.Itinerarios) != 1 {
		t.Fatalf("itinerário não encontrado: %v", err)
	}
	coordenador.mutex.Lock()
	coordenador.ganchoAposCommit = func(confirmados int) error {
		if confirmados == 1 {
			return errQuedaSimulada
		}
		return nil
	}
	coordenador.mutex.Unlock()
	_, _ = coordenador.ConfirmarReserva(passageiro.Token, resultado.Itinerarios[0].ID)
	disponivel.Store(false)

	reiniciado, err := NovoEstadoEmpresaComArquivos("empresa-a", "CaronasGo", usuarios, estadoJSON)
	if err != nil {
		t.Fatal(err)
	}
	reiniciado.ConfigurarBuscaDistribuida(pares, 100*time.Millisecond)
	time.Sleep(1200 * time.Millisecond)
	reiniciado.mutex.Lock()
	reservasEnquantoFora := len(reiniciado.reservas)
	var statusEnquantoFora string
	for _, registro := range reiniciado.coordenacoes {
		statusEnquantoFora = registro.Status
	}
	reiniciado.mutex.Unlock()
	if reservasEnquantoFora != 0 || statusEnquantoFora != statusCoordenacaoCommit {
		t.Fatalf("decisão foi perdida durante a indisponibilidade: reservas=%d status=%s", reservasEnquantoFora, statusEnquantoFora)
	}

	// O servidor local segue operando enquanto aguarda a empresa remota.
	if _, err := reiniciado.Login("cliente", "1234"); err != nil {
		t.Fatalf("operação local parou durante a recuperação: %v", err)
	}
	disponivel.Store(true)
	esperarCondicao(t, 3*time.Second, func() bool {
		reiniciado.mutex.Lock()
		defer reiniciado.mutex.Unlock()
		return len(reiniciado.reservas) == 1
	}, "a reserva não foi concluída depois que o participante voltou")
	if vagasTrechoTeste(participante, "CAR-001", 0) != 0 {
		t.Fatal("o participante que voltou não confirmou o assento")
	}
}

func esperarCondicao(t *testing.T, limite time.Duration, condicao func() bool, mensagem string) {
	t.Helper()
	prazo := time.Now().Add(limite)
	for time.Now().Before(prazo) {
		if condicao() {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal(mensagem)
}
