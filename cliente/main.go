package main

import (
	"fmt"
	"os"

	"vaijunto/comum"
	"vaijunto/terminal"
)

var entrada = terminal.NovaEntrada(os.Stdin)

func main() {
	conexao, err := comum.Conectar(enderecoServidor())
	if err != nil {
		fmt.Println("Não foi possível conectar ao servidor:", err)
		fmt.Println("Confira se o servidor está ligado e se VAIJUNTO_SERVER está correto.")
		return
	}
	defer conexao.Fechar()

	fmt.Println("Conectado ao servidor", enderecoServidor())
	for {
		mostrarMenu(conexao)
		switch entrada.Texto("Escolha uma opção: ") {
		case "1":
			if conexao.Token == "" {
				fazerLogin(conexao)
			} else {
				sairDaConta(conexao)
			}
		case "2":
			if conexao.Token == "" {
				fazerCadastro(conexao)
			} else {
				buscarItinerarios(conexao)
			}
		case "3":
			listarReservas(conexao)
		case "4":
			cancelarReserva(conexao)
		case "0":
			fmt.Println("Cliente passageiro encerrado.")
			return
		default:
			fmt.Println("Opção inválida.")
		}
		entrada.Pausa()
	}
}
