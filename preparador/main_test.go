package main

import (
	"testing"

	"vaijunto/comum"
)

func TestInterpretarEmpresas(t *testing.T) {
	resultado, err := interpretarEmpresas("empresa-a=127.0.0.1:9001,empresa-b=127.0.0.1:9002,empresa-c=127.0.0.1:9003")
	if err != nil {
		t.Fatal(err)
	}
	if resultado["empresa-b"] != "127.0.0.1:9002" {
		t.Fatalf("endereço inesperado: %q", resultado["empresa-b"])
	}
}

func TestInterpretarEmpresasExigeTresInstancias(t *testing.T) {
	if _, err := interpretarEmpresas("empresa-a=localhost:9001"); err == nil {
		t.Fatal("era esperado erro quando faltam empresas")
	}
}

func TestCaronaIgualReutilizaSomenteTrechoDisponivel(t *testing.T) {
	trecho := trechoDemonstracao{
		Rota: []string{"Salvador", "Feira de Santana"}, Horario: "08:00", Duracao: 60, Preco: 2500,
	}
	carona := comum.Carona{
		Rota: []string{"Salvador", "Feira de Santana"}, Data: "2026-11-05", Horario: "08:00",
		Assentos: 1, PrecosCentavos: []int{2500}, DuracoesMin: []int{60}, DisponiveisPorTrecho: []int{1},
	}
	if !caronaIgual(carona, "2026-11-05", 1, trecho) {
		t.Fatal("carona disponível deveria ser reutilizada")
	}
	carona.DisponiveisPorTrecho[0] = 0
	if caronaIgual(carona, "2026-11-05", 1, trecho) {
		t.Fatal("carona sem vaga não deve ser reutilizada")
	}
}
