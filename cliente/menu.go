package main

import (
	"fmt"

	"vaijunto/comum"
)

func mostrarMenu(conexao *comum.ConexaoTCP) {
	fmt.Println("\n================================")
	fmt.Println("       VAIJUNTO - PASSAGEIRO")
	fmt.Println("================================")
	if conexao.Token == "" {
		fmt.Println("1 - Entrar")
		fmt.Println("2 - Criar conta de passageiro")
		fmt.Println("0 - Sair")
		fmt.Println("================================")
		return
	}

	fmt.Println("Logado como:", conexao.NomeLogado)
	fmt.Println("--------------------------------")
	fmt.Println("1 - Sair da conta")
	fmt.Println("2 - Buscar corrida e reservar")
	fmt.Println("3 - Minhas reservas")
	fmt.Println("4 - Cancelar reserva")
	fmt.Println("0 - Encerrar programa")
	fmt.Println("================================")
}
