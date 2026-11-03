package main

import (
	"fmt"
	"strings"

	"vaijunto/comum"
	"vaijunto/terminal"
)

// envia a requisição de reserva para o servidor e mostra a resposta.
func confirmarReserva(conexao *comum.ConexaoTCP, itinerario comum.Itinerario) {
	confirmacao := strings.ToLower(entrada.Texto(fmt.Sprintf(
		"Confirmar reserva por %s? (s/n): ", terminal.FormatarDinheiro(itinerario.PrecoCentavos))))
	if confirmacao != "s" {
		fmt.Println("Reserva não realizada.")
		return
	}

	// Envia só o código da opção. O servidor já conhece os trechos e o preço
	// e vai conferir se ainda há vagas antes de aceitar.
	resposta, err := conexao.Enviar(comum.AcaoConfirmarReserva, comum.IDDados{ID: itinerario.ID})
	if !mostrarErro(resposta, err) {
		return
	}
	reserva, err := comum.LerDados[comum.Reserva](resposta)
	if err != nil {
		fmt.Println("Resposta do servidor inválida:", err)
		return
	}
	fmt.Printf("\nReserva %s confirmada!\n", reserva.ID)
	fmt.Printf("%s -> %s | %s | %s\n",
		reserva.Itinerario.Origem, reserva.Itinerario.Destino,
		terminal.FormatarData(reserva.Itinerario.Data), terminal.FormatarDinheiro(reserva.Itinerario.PrecoCentavos))
	fmt.Printf("Vagas restantes: %d\n", reserva.Itinerario.Vagas)
}

func listarReservas(conexao *comum.ConexaoTCP) {
	if !estaLogado(conexao) { //
		return
	}
	reservas, ok := obterReservas(conexao) // envia a requisição de listagem de reservas para o servidor e retorna a lista de reservas do passageiro.
	if !ok {
		return
	}
	if len(reservas) == 0 {
		fmt.Println("Você ainda não possui reservas.")
		return
	}

	for i, reserva := range reservas {
		mostrarReserva(i+1, reserva)
		for _, trecho := range reserva.Itinerario.Trechos {
			fmt.Printf("     %s -> %s | %s às %s | %s\n",
				trecho.Origem, trecho.Destino, trecho.Saida.Format("02/01/2006"),
				trecho.Saida.Format("15:04"), trecho.CaronaID)
		}
	}
}

func cancelarReserva(conexao *comum.ConexaoTCP) {
	if !estaLogado(conexao) {
		return
	}
	// obtém todas as reservas do passageiro e filtra apenas as confirmadas
	reservas, ok := obterReservas(conexao)
	if !ok {
		return
	}
	// filtra apenas as reservas confirmadas
	ativas := make([]comum.Reserva, 0)
	for _, reserva := range reservas {
		if reserva.Status == comum.StatusReservaConfirmada {
			ativas = append(ativas, reserva)
		}
	}
	if len(ativas) == 0 {
		fmt.Println("Você não possui reservas confirmadas para cancelar.")
		return
	}

	fmt.Println("\n--- Escolha a reserva ---")
	for i, reserva := range ativas {
		mostrarReserva(i+1, reserva)
	}
	// pede para o passageiro escolher uma reserva para cancelar
	escolha := entrada.Opcao("\nEscolha uma reserva (0 para voltar): ", len(ativas))
	if escolha == 0 {
		fmt.Println("Cancelamento desistido.")
		return
	}
	selecionada := ativas[escolha-1]
	confirmacao := strings.ToLower(entrada.Texto("Tem certeza que deseja cancelar? (s/n): "))
	if confirmacao != "s" {
		fmt.Println("Cancelamento desistido.")
		return
	}
	// envia a requisição de cancelamento para o servidor e mostra a resposta
	resposta, err := conexao.Enviar(comum.AcaoCancelarReserva, comum.IDDados{ID: selecionada.ID})
	if mostrarErro(resposta, err) {
		fmt.Println(resposta.Mensagem)
	}
}

// obterReservas envia a requisição de listagem de reservas para o servidor e retorna a lista de reservas do passageiro.
func obterReservas(conexao *comum.ConexaoTCP) ([]comum.Reserva, bool) {
	resposta, err := conexao.Enviar(comum.AcaoListarReservas, nil)
	if !mostrarErro(resposta, err) {
		return nil, false
	}
	reservas, err := comum.LerDados[[]comum.Reserva](resposta)
	if err != nil {
		fmt.Println("Resposta do servidor inválida:", err)
		return nil, false
	}
	return reservas, true
}

func mostrarReserva(numero int, reserva comum.Reserva) {
	fmt.Printf("\n%d) %s | %s\n", numero, reserva.ID, reserva.Status)
	fmt.Printf("   %s -> %s\n", reserva.Itinerario.Origem, reserva.Itinerario.Destino)
	fmt.Printf("   %s | %s | %s\n", terminal.FormatarData(reserva.Itinerario.Data),
		reserva.Itinerario.Trechos[0].Saida.Format("15:04"),
		terminal.FormatarDinheiro(reserva.Itinerario.PrecoCentavos))
}
