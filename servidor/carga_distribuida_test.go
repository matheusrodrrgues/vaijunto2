package main

import (
	"sync"
	"testing"
	"time"

	"net/http/httptest"

	"vaijunto/comum"
)

func TestCargaDistribuidaNaoVendeUltimoAssentoDuasVezes(t *testing.T) {
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
	tokens := make([]string, len(coordenadores))
	for i, coordenador := range coordenadores {
		coordenador.ConfigurarBuscaDistribuida(
			[]EmpresaRemota{{ID: "empresa-b", URLBase: servidorDona.URL}}, time.Second,
		)
		tokens[i] = loginTeste(t, coordenador, "cliente", "1234").Token
	}

	const passageiros = 60
	type tentativaCarga struct {
		coordenador int
		itinerario  string
	}
	tentativas := make([]tentativaCarga, 0, passageiros)
	for i := 0; i < passageiros; i++ {
		indice := i % len(coordenadores)
		resultado, err := coordenadores[indice].BuscarItinerariosDetalhado(
			tokens[indice],
			comum.BuscarItinerariosDados{Origem: "Salvador", Destino: "Feira de Santana", Data: "2026-10-20"},
		)
		if err != nil || len(resultado.Itinerarios) != 1 {
			t.Fatalf("busca %d falhou: %v", i, err)
		}
		tentativas = append(tentativas, tentativaCarga{
			coordenador: indice, itinerario: resultado.Itinerarios[0].ID,
		})
	}

	inicio := make(chan struct{})
	resultados := make(chan bool, passageiros)
	var grupo sync.WaitGroup
	for _, tentativa := range tentativas {
		tentativa := tentativa
		grupo.Add(1)
		go func() {
			defer grupo.Done()
			<-inicio
			_, err := coordenadores[tentativa.coordenador].ConfirmarReserva(
				tokens[tentativa.coordenador], tentativa.itinerario,
			)
			resultados <- err == nil
		}()
	}
	close(inicio)
	grupo.Wait()
	close(resultados)

	sucessos := 0
	for sucesso := range resultados {
		if sucesso {
			sucessos++
		}
	}
	if sucessos != 1 {
		t.Fatalf("esperava exatamente uma confirmação sob carga, recebeu %d", sucessos)
	}
	if vagasTrechoTeste(dona, "CAR-001", 0) != 0 {
		t.Fatal("o assento final não ficou consistente")
	}
	resumo := dona.ResumoTransacoes()
	if resumo.Confirmadas != 1 || resumo.Preparadas != 0 {
		t.Fatalf("diagnóstico final inesperado: %#v", resumo)
	}
	for _, coordenador := range coordenadores {
		if pendentes := coordenador.ResumoTransacoes().CoordenacoesPendentes; pendentes != 0 {
			t.Fatalf("coordenador %s terminou com %d transações pendentes", coordenador.empresaID, pendentes)
		}
	}
}
