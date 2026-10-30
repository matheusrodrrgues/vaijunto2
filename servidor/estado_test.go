package main

import (
	"testing"

	"vaijunto/comum"
)

func TestBuscaSemDataMostraCorridasDisponiveis(t *testing.T) {
	estado := NovoEstado()
	motorista := loginTeste(t, estado, "motorista", "1234")
	passageiro := loginTeste(t, estado, "cliente", "1234")

	for _, carona := range []comum.PublicarCaronaDados{
		{Rota: []string{"FSA", "Alagoinhas"}, Data: "2026-10-20", Horario: "10:00", Assentos: 3, PrecosCentavos: []int{5000}, DuracoesMin: []int{35}},
		{Rota: []string{"FSA", "Alagoinhas"}, Data: "2026-10-18", Horario: "08:00", Assentos: 2, PrecosCentavos: []int{4500}, DuracoesMin: []int{35}},
	} {
		if _, err := estado.PublicarCarona(motorista.Token, carona); err != nil {
			t.Fatal(err)
		}
	}

	opcoes, err := estado.BuscarItinerarios(passageiro.Token, comum.BuscarItinerariosDados{
		Origem: "FSA", Destino: "Alagoinhas",
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(opcoes) != 2 {
		t.Fatalf("esperava duas corridas, recebeu %d", len(opcoes))
	}
	if opcoes[0].Data != "2026-10-18" || opcoes[0].Vagas != 2 {
		t.Fatalf("primeira opção inesperada: data=%s vagas=%d", opcoes[0].Data, opcoes[0].Vagas)
	}
	if opcoes[1].Data != "2026-10-20" || opcoes[1].Vagas != 3 {
		t.Fatalf("segunda opção inesperada: data=%s vagas=%d", opcoes[1].Data, opcoes[1].Vagas)
	}
}

func TestReservaEDevolucaoDeAssentos(t *testing.T) {
	estado := NovoEstado()
	motorista := loginTeste(t, estado, "motorista", "1234")
	passageiro := loginTeste(t, estado, "cliente", "1234")

	carona, err := estado.PublicarCarona(motorista.Token, comum.PublicarCaronaDados{
		Rota: []string{"Salvador", "Feira de Santana", "Itaberaba"},
		Data: "2026-09-10", Horario: "08:00", Assentos: 1,
		PrecosCentavos: []int{2500, 3500}, DuracoesMin: []int{70, 90},
	})
	if err != nil {
		t.Fatal(err)
	}

	itinerarios, err := estado.BuscarItinerarios(passageiro.Token, comum.BuscarItinerariosDados{
		Origem: "Salvador", Destino: "Itaberaba", Data: "2026-09-10",
	})
	if err != nil || len(itinerarios) == 0 {
		t.Fatalf("busca falhou: %v, encontrados: %d", err, len(itinerarios))
	}

	reserva, err := estado.ConfirmarReserva(passageiro.Token, itinerarios[0].ID)
	if err != nil {
		t.Fatal(err)
	}
	if reserva.NomePassageiro != "Maria Passageira" {
		t.Fatalf("nome do passageiro inesperado: %s", reserva.NomePassageiro)
	}
	if reserva.Itinerario.Vagas != 0 {
		t.Fatalf("quantidade de vagas restantes inesperada: %d", reserva.Itinerario.Vagas)
	}
	if got := estado.caronas[carona.ID].DisponiveisPorTrecho; got[0] != 0 || got[1] != 0 {
		t.Fatalf("assentos não foram descontados corretamente: %v", got)
	}

	if err := estado.CancelarReserva(passageiro.Token, reserva.ID); err != nil {
		t.Fatal(err)
	}
	if got := estado.caronas[carona.ID].DisponiveisPorTrecho; got[0] != 1 || got[1] != 1 {
		t.Fatalf("assentos não foram devolvidos corretamente: %v", got)
	}
}

func TestItinerarioNaoEConfirmadoPelaMetade(t *testing.T) {
	estado := NovoEstado()
	motorista := loginTeste(t, estado, "motorista", "1234")
	passageiro := loginTeste(t, estado, "cliente", "1234")

	primeira, err := estado.PublicarCarona(motorista.Token, comum.PublicarCaronaDados{
		Rota: []string{"Salvador", "Feira de Santana"},
		Data: "2026-09-11", Horario: "08:00", Assentos: 1,
		PrecosCentavos: []int{2500}, DuracoesMin: []int{60},
	})
	if err != nil {
		t.Fatal(err)
	}
	_, err = estado.PublicarCarona(motorista.Token, comum.PublicarCaronaDados{
		Rota: []string{"Feira de Santana", "Itaberaba"},
		Data: "2026-09-11", Horario: "09:20", Assentos: 1,
		PrecosCentavos: []int{3500}, DuracoesMin: []int{80},
	})
	if err != nil {
		t.Fatal(err)
	}

	completo, err := estado.BuscarItinerarios(passageiro.Token, comum.BuscarItinerariosDados{
		Origem: "Salvador", Destino: "Itaberaba", Data: "2026-09-11",
	})
	if err != nil || len(completo) == 0 {
		t.Fatalf("itinerário combinado não encontrado: %v", err)
	}
	segundoTrecho, err := estado.BuscarItinerarios(passageiro.Token, comum.BuscarItinerariosDados{
		Origem: "Feira de Santana", Destino: "Itaberaba", Data: "2026-09-11",
	})

	if err != nil || len(segundoTrecho) == 0 {
		t.Fatalf("segundo trecho não encontrado: %v", err)
	}
	if _, err := estado.ConfirmarReserva(passageiro.Token, segundoTrecho[0].ID); err != nil {
		t.Fatal(err)
	}

	if _, err := estado.ConfirmarReserva(passageiro.Token, completo[0].ID); err == nil {
		t.Fatal("a reserva deveria falhar porque o segundo trecho ficou lotado")
	}
	if got := estado.caronas[primeira.ID].DisponiveisPorTrecho[0]; got != 1 {
		t.Fatalf("o primeiro trecho foi alterado mesmo com falha: disponiveis=%d", got)
	}
}

func TestCancelarCaronaCancelaAReservaCompleta(t *testing.T) {
	estado := NovoEstado()
	motorista := loginTeste(t, estado, "motorista", "1234")
	passageiro := loginTeste(t, estado, "cliente", "1234")

	primeira, err := estado.PublicarCarona(motorista.Token, comum.PublicarCaronaDados{
		Rota: []string{"Salvador", "Feira de Santana"},
		Data: "2026-09-13", Horario: "08:00", Assentos: 1,
		PrecosCentavos: []int{2000}, DuracoesMin: []int{60},
	})
	if err != nil {
		t.Fatal(err)
	}
	segunda, err := estado.PublicarCarona(motorista.Token, comum.PublicarCaronaDados{
		Rota: []string{"Feira de Santana", "Itaberaba"},
		Data: "2026-09-13", Horario: "09:20", Assentos: 1,
		PrecosCentavos: []int{3000}, DuracoesMin: []int{80},
	})
	if err != nil {
		t.Fatal(err)
	}

	opcoes, err := estado.BuscarItinerarios(passageiro.Token, comum.BuscarItinerariosDados{
		Origem: "Salvador", Destino: "Itaberaba", Data: "2026-09-13",
	})
	if err != nil || len(opcoes) == 0 {
		t.Fatalf("itinerário não encontrado: %v", err)
	}
	reserva, err := estado.ConfirmarReserva(passageiro.Token, opcoes[0].ID)
	if err != nil {
		t.Fatal(err)
	}

	if err := estado.CancelarCarona(motorista.Token, segunda.ID); err != nil {
		t.Fatal(err)
	}
	if estado.reservas[reserva.ID].Status != comum.StatusCanceladaMotorista {
		t.Fatalf("status inesperado: %s", estado.reservas[reserva.ID].Status)
	}
	if estado.caronas[primeira.ID].DisponiveisPorTrecho[0] != 1 || estado.caronas[segunda.ID].DisponiveisPorTrecho[0] != 1 {
		t.Fatal("os assentos da reserva completa não foram devolvidos")
	}
}

func loginTeste(t *testing.T, estado *Estado, usuario, senha string) comum.LoginResposta {
	t.Helper()
	login, err := estado.Login(usuario, senha)
	if err != nil {
		t.Fatal(err)
	}
	return login
}
