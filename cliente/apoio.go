package main

import (
	"fmt"
	"os"

	"vaijunto/comum"
)

func estaLogado(conexao *comum.ConexaoTCP) bool {
	if conexao.Token == "" {
		fmt.Println("Faça login primeiro.")
		return false
	}
	return true
}

func mostrarErro(resposta comum.Resposta, err error) bool {
	if err != nil {
		fmt.Println("Erro de comunicação:", err)
		return false
	}
	if !resposta.Sucesso {
		fmt.Println("Erro:", resposta.Mensagem)
		return false
	}
	return true
}

func enderecoServidor() string {
	endereco := os.Getenv("VAIJUNTO_SERVER")
	if endereco == "" {
		return "127.0.0.1:9000"
	}
	return endereco
}
