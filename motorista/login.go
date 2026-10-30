package main

import (
	"fmt"

	"vaijunto/comum"
)

func fazerLogin(conexao *comum.ConexaoTCP) {
	usuario := entrada.Texto("Usuário: ")
	senha := entrada.Texto("Senha: ")
	entrarComUsuario(conexao, usuario, senha)
}

func entrarComUsuario(conexao *comum.ConexaoTCP, usuario, senha string) bool {
	resposta, err := conexao.Enviar(comum.AcaoLogin, comum.LoginDados{Usuario: usuario, Senha: senha})
	if !mostrarErro(resposta, err) {
		return false
	}
	dados, err := comum.LerDados[comum.LoginResposta](resposta)
	if err != nil {
		fmt.Println("Resposta do servidor inválida:", err)
		return false
	}
	if dados.Perfil != comum.PerfilMotorista {
		fmt.Println("Esta conta não é de motorista.")
		return false
	}
	conexao.Token = dados.Token
	conexao.NomeLogado = dados.Nome
	fmt.Println("Bem-vindo,", dados.Nome)
	return true
}

func fazerCadastro(conexao *comum.ConexaoTCP) {
	fmt.Println("\n--- Criar conta de motorista ---")
	nome := entrada.Texto("Nome: ")
	usuario := entrada.Texto("Usuário: ")
	senha := entrada.Texto("Senha: ")
	confirmacao := entrada.Texto("Confirmar senha: ")
	if senha != confirmacao {
		fmt.Println("As senhas não conferem.")
		return
	}

	resposta, err := conexao.Enviar(comum.AcaoCadastro, comum.CadastroDados{
		Nome: nome, Usuario: usuario, Senha: senha, Perfil: comum.PerfilMotorista,
	})
	if mostrarErro(resposta, err) {
		fmt.Println(resposta.Mensagem)
		fmt.Println("Entrando automaticamente...")
		entrarComUsuario(conexao, usuario, senha)
	}
}

// sairDaConta limpa o login usado neste cliente. A conexão continua aberta
// e não há uma mensagem para revogar o token no servidor.
func sairDaConta(conexao *comum.ConexaoTCP) {
	conexao.Token = ""
	conexao.NomeLogado = ""
	fmt.Println("Você saiu da conta.")
}
