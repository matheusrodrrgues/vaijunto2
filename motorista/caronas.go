package main

import (
	"fmt"
	"strings"

	"vaijunto/comum"
	"vaijunto/terminal"
)

func publicarCarona(conexao *comum.ConexaoTCP) {
	if !estaLogado(conexao) {
		return
	}
	fmt.Println("\n--- Publicar carona ---")
	qtdCidades := entrada.InteiroMinimo("Quantidade de cidades da rota: ", 2)
	rota := make([]string, qtdCidades)
	for i := range rota {
		for strings.TrimSpace(rota[i]) == "" {
			rota[i] = entrada.Texto(fmt.Sprintf("Cidade %d: ", i+1))
		}
	}

	// Com três cidades existem dois trechos. Cada um recebe seu próprio
	// preço e duração, mesmo sendo parte da mesma carona.
	precosCentavos := make([]int, qtdCidades-1)
	duracoes := make([]int, qtdCidades-1)
	for i := 0; i < qtdCidades-1; i++ {
		fmt.Printf("\nTrecho %s -> %s\n", rota[i], rota[i+1])
		precosCentavos[i] = entrada.DinheiroCentavos("Preço em reais: R$ ")
		duracoes[i] = entrada.InteiroMinimo("Duração aproximada em minutos: ", 1)
	}

	dados := comum.PublicarCaronaDados{
		Rota: rota, Data: entrada.Data("Data (DIA/MÊS/ANO): "),
		Horario:        entrada.Texto("Horário de partida (HH:MM): "),
		Assentos:       entrada.InteiroMinimo("Quantidade de assentos: ", 1),
		PrecosCentavos: precosCentavos, DuracoesMin: duracoes,
	}
	resposta, err := conexao.Enviar(comum.AcaoPublicarCarona, dados)
	if !mostrarErro(resposta, err) {
		return
	}
	carona, err := comum.LerDados[comum.Carona](resposta)
	if err != nil {
		fmt.Println("Resposta do servidor inválida:", err)
		return
	}
	fmt.Println("Carona publicada com o código", carona.ID)
}

func listarCaronas(conexao *comum.ConexaoTCP) {
	if !estaLogado(conexao) {
		return
	}
	caronas, ok := obterCaronas(conexao)
	if !ok {
		return
	}
	if len(caronas) == 0 {
		fmt.Println("Você ainda não publicou nenhuma carona.")
		return
	}

	for i, carona := range caronas {
		situacao := "ativa"
		if carona.Cancelada {
			situacao = "cancelada"
		}
		fmt.Printf("\n%d) %s | %s\n", i+1, carona.ID, situacao)
		fmt.Printf("   %s às %s | %s\n", terminal.FormatarData(carona.Data), carona.Horario, strings.Join(carona.Rota, " -> "))
		for i := 0; i < len(carona.Rota)-1; i++ {
			fmt.Printf("   %s -> %s | %s | %d de %d vagas\n",
				carona.Rota[i], carona.Rota[i+1], terminal.FormatarDinheiro(carona.PrecosCentavos[i]), carona.DisponiveisPorTrecho[i], carona.Assentos)
		}
	}
}

func listarPassageiros(conexao *comum.ConexaoTCP) {
	if !estaLogado(conexao) {
		return
	}
	carona, ok := escolherCaronaAtiva(conexao, "Escolha uma carona para consultar")
	if !ok {
		return
	}
	resposta, err := conexao.Enviar(comum.AcaoListarPassageiros, comum.IDDados{ID: carona.ID})
	if !mostrarErro(resposta, err) {
		return
	}
	reservas, err := comum.LerDados[[]comum.Reserva](resposta)
	if err != nil {
		fmt.Println("Resposta do servidor inválida:", err)
		return
	}
	if len(reservas) == 0 {
		fmt.Println("Nenhum passageiro reservou esta carona.")
		return
	}
	for _, reserva := range reservas {
		fmt.Printf("\n%s | %s\n", reserva.ID, reserva.Status)
		fmt.Printf("Passageiro: %s (%s)\n", reserva.NomePassageiro, reserva.Passageiro)
		fmt.Printf("Trecho reservado: %s -> %s\n", reserva.Itinerario.Origem, reserva.Itinerario.Destino)
	}
}

func cancelarCarona(conexao *comum.ConexaoTCP) {
	if !estaLogado(conexao) {
		return
	}
	carona, ok := escolherCaronaAtiva(conexao, "Escolha uma carona para cancelar")
	if !ok {
		return
	}
	confirmacao := strings.ToLower(entrada.Texto("Tem certeza que deseja cancelar? (s/n): "))
	if confirmacao != "s" {
		fmt.Println("Cancelamento desistido.")
		return
	}
	resposta, err := conexao.Enviar(comum.AcaoCancelarCarona, comum.IDDados{ID: carona.ID})
	if mostrarErro(resposta, err) {
		fmt.Println(resposta.Mensagem)
	}
}

func obterCaronas(conexao *comum.ConexaoTCP) ([]comum.Carona, bool) {
	resposta, err := conexao.Enviar(comum.AcaoListarCaronas, nil)
	if !mostrarErro(resposta, err) {
		return nil, false
	}
	caronas, err := comum.LerDados[[]comum.Carona](resposta)
	if err != nil {
		fmt.Println("Resposta do servidor inválida:", err)
		return nil, false
	}
	return caronas, true
}

// escolherCaronaAtiva mostra números no menu para a pessoa não precisar
// decorar CAR-001. O código real segue guardado na carona selecionada.
func escolherCaronaAtiva(conexao *comum.ConexaoTCP, titulo string) (comum.Carona, bool) {
	caronas, ok := obterCaronas(conexao)
	if !ok {
		return comum.Carona{}, false
	}
	ativas := make([]comum.Carona, 0)
	for _, carona := range caronas {
		if !carona.Cancelada {
			ativas = append(ativas, carona)
		}
	}
	if len(ativas) == 0 {
		fmt.Println("Você ainda não possui caronas ativas.")
		return comum.Carona{}, false
	}

	fmt.Println("\n---", titulo, "---")
	for i, carona := range ativas {
		fmt.Printf("%d) %s | %s às %s\n", i+1, strings.Join(carona.Rota, " -> "),
			terminal.FormatarData(carona.Data), carona.Horario)
	}
	escolha := entrada.Opcao("Escolha uma opção (0 para voltar): ", len(ativas))
	if escolha == 0 {
		return comum.Carona{}, false
	}
	return ativas[escolha-1], true
}
