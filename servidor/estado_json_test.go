package main

import (
	"path/filepath"
	"testing"

	"vaijunto/comum"
)

func TestCaronasEReservasContinuamDepoisDeReiniciar(t *testing.T) {
	pasta := t.TempDir()
	usuarios := filepath.Join(pasta, "usuarios.json")
	estadoJSON := filepath.Join(pasta, "estado.json")

	estado, err := NovoEstadoComArquivos(usuarios, estadoJSON)
	if err != nil {
		t.Fatal(err)
	}
	motorista := loginTeste(t, estado, "motorista", "1234")
	passageiro := loginTeste(t, estado, "cliente", "1234")
	carona, err := estado.PublicarCarona(motorista.Token, comum.PublicarCaronaDados{
		Rota: []string{"Feira de Santana", "Salvador"}, Data: "2026-09-20",
		Horario: "08:00", Assentos: 2, PrecosCentavos: []int{3500}, DuracoesMin: []int{70},
	})
	if err != nil {
		t.Fatal(err)
	}
	itinerarios, err := estado.BuscarItinerarios(passageiro.Token, comum.BuscarItinerariosDados{
		Origem: "Feira de Santana", Destino: "Salvador", Data: "2026-09-20",
	})
	if err != nil || len(itinerarios) != 1 {
		t.Fatalf("busca inesperada: %v, opcoes=%d", err, len(itinerarios))
	}
	reserva, err := estado.ConfirmarReserva(passageiro.Token, itinerarios[0].ID)
	if err != nil {
		t.Fatal(err)
	}

	reiniciado, err := NovoEstadoComArquivos(usuarios, estadoJSON)
	if err != nil {
		t.Fatal(err)
	}
	if reiniciado.caronas[carona.ID].DisponiveisPorTrecho[0] != 1 {
		t.Fatal("a quantidade de vagas não foi recuperada")
	}
	if reiniciado.reservas[reserva.ID].Status != comum.StatusReservaConfirmada {
		t.Fatal("a reserva não foi recuperada")
	}

	novoMotorista := loginTeste(t, reiniciado, "motorista", "1234")
	novoPassageiro := loginTeste(t, reiniciado, "cliente", "1234")
	if err := reiniciado.CancelarReserva(novoPassageiro.Token, reserva.ID); err != nil {
		t.Fatal(err)
	}
	novaCarona, err := reiniciado.PublicarCarona(novoMotorista.Token, comum.PublicarCaronaDados{
		Rota: []string{"Salvador", "Lauro de Freitas"}, Data: "2026-09-21",
		Horario: "09:00", Assentos: 1, PrecosCentavos: []int{1500}, DuracoesMin: []int{40},
	})
	if err != nil {
		t.Fatal(err)
	}
	if novaCarona.ID != "CAR-002" {
		t.Fatalf("contador de caronas não foi recuperado: %s", novaCarona.ID)
	}
	if err := reiniciado.CancelarCarona(novoMotorista.Token, novaCarona.ID); err != nil {
		t.Fatal(err)
	}

	reiniciadoNovamente, err := NovoEstadoComArquivos(usuarios, estadoJSON)
	if err != nil {
		t.Fatal(err)
	}
	if reiniciadoNovamente.reservas[reserva.ID].Status != comum.StatusReservaCancelada {
		t.Fatal("o cancelamento da reserva não foi recuperado")
	}
	if reiniciadoNovamente.caronas[carona.ID].DisponiveisPorTrecho[0] != 2 {
		t.Fatal("a devolução da vaga não foi recuperada")
	}
	if !reiniciadoNovamente.caronas[novaCarona.ID].Cancelada {
		t.Fatal("o cancelamento da carona não foi recuperado")
	}
}
