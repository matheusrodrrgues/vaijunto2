package main

import (
	"fmt"

	"vaijunto/comum"
)

func mostrarMenu(conexao *comum.ConexaoTCP) {
	fmt.Println("\n================================")
	fmt.Println("       VAIJUNTO - MOTORISTA")
	fmt.Println("================================")
	if conexao.Token == "" {
		fmt.Println("1 - Entrar")
		fmt.Println("2 - Criar conta de motorista")
		fmt.Println("0 - Sair")
		fmt.Println("================================")
		return
	}

	fmt.Println("Logado como:", conexao.NomeLogado)
	fmt.Println("--------------------------------")
	fmt.Println("1 - Sair da conta")
	fmt.Println("2 - Publicar carona")
	fmt.Println("3 - Consultar minhas caronas")
	fmt.Println("4 - Consultar passageiros")
	fmt.Println("5 - Cancelar carona")
	fmt.Println("0 - Encerrar programa")
	fmt.Println("================================")
}
