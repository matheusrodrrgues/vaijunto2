package main

import (
	"fmt"

	"vaijunto/comum"
)

func fazerLogin(conexao *comum.ConexaoTCP) {
	// mostra o menu de login e pede usuário e senha
	fmt.Println("\n--- Login de passageiro ---")
	usuario := entrada.Texto("Usuário: ")
	senha := entrada.Texto("Senha: ")
	entrarComUsuario(conexao, usuario, senha)
}

// entrarComUsuario envia os dados de login para o servidor e, se forem válidos, guarda o token na conexão.
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
	if dados.Perfil != comum.PerfilPassageiro {
		fmt.Println("Esta conta não é de passageiro.")
		return false
	}
	// login bem-sucedido, guarda o token e o nome do usuário logado na conexão
	conexao.Token = dados.Token
	conexao.NomeLogado = dados.Nome
	fmt.Println("Bem-vindo,", dados.Nome)
	return true
}

func fazerCadastro(conexao *comum.ConexaoTCP) {
	fmt.Println("\n--- Criar conta de passageiro ---")
	nome := entrada.Texto("Nome: ")
	usuario := entrada.Texto("Usuário: ")
	senha := entrada.Texto("Senha: ")
	confirmacao := entrada.Texto("Confirmar senha: ")
	if senha != confirmacao {
		fmt.Println("As senhas não conferem.")
		return
	}

	resposta, err := conexao.Enviar(comum.AcaoCadastro, comum.CadastroDados{
		Nome: nome, Usuario: usuario, Senha: senha, Perfil: comum.PerfilPassageiro,
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
