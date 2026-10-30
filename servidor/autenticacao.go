package main

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strings"

	"vaijunto/comum"
)

func (e *Estado) adicionarUsuario(nome, login, senha, perfil string) {
	login = strings.ToLower(strings.TrimSpace(login))
	e.usuarios[login] = usuario{
		Nome:      nome,
		Login:     login,
		SenhaHash: sha256.Sum256([]byte(senha)),
		Perfil:    perfil,
	}
}

// Cadastrar normaliza o usuário e verifica duplicidade com a trava ativa.
// Dois cadastros simultâneos não podem passar com o mesmo nome de acesso.
func (e *Estado) Cadastrar(dados comum.CadastroDados) error {
	e.mutex.Lock()
	defer e.mutex.Unlock()

	nome := strings.TrimSpace(dados.Nome)
	login := strings.ToLower(strings.TrimSpace(dados.Usuario))
	if len(nome) < 2 {
		return errors.New("informe um nome válido")
	}
	if len(login) < 3 || strings.ContainsAny(login, " \t\r\n") {
		return errors.New("o usuário deve ter pelo menos 3 caracteres e não pode conter espaços")
	}
	if len(dados.Senha) < 4 {
		return errors.New("a senha deve ter pelo menos 4 caracteres")
	}
	if dados.Perfil != comum.PerfilMotorista && dados.Perfil != comum.PerfilPassageiro {
		return errors.New("perfil inválido")
	}
	if _, existe := e.usuarios[login]; existe {
		return errors.New("este usuário já está cadastrado")
	}

	e.adicionarUsuario(nome, login, dados.Senha, dados.Perfil)
	// Se não ficou salvo, não deixa uma conta que só existe até o próximo reinício.
	if err := e.salvarUsuariosSemLock(); err != nil {
		delete(e.usuarios, login)
		registrar("CADASTRO usuario=%s perfil=%s resultado=falha_salvar", login, dados.Perfil)
		return errors.New("não foi possível salvar o cadastro")
	}
	registrar("CADASTRO usuario=%s perfil=%s resultado=sucesso", login, dados.Perfil)
	return nil
}

func (e *Estado) Login(login, senha string) (comum.LoginResposta, error) {
	e.mutex.Lock()
	defer e.mutex.Unlock()

	loginNormalizado := strings.ToLower(strings.TrimSpace(login))
	u, existe := e.usuarios[loginNormalizado]
	if !existe || u.SenhaHash != sha256.Sum256([]byte(senha)) {
		registrar("LOGIN usuario=%s resultado=falha", loginNormalizado)
		return comum.LoginResposta{}, errors.New("usuário ou senha incorretos")
	}

	token, err := novoToken()
	if err != nil {
		registrar("LOGIN usuario=%s resultado=falha_sessao", loginNormalizado)
		return comum.LoginResposta{}, errors.New("não foi possível criar a sessão")
	}
	e.sessoes[token] = u.Login
	registrar("LOGIN usuario=%s perfil=%s resultado=sucesso", u.Login, u.Perfil)
	return comum.LoginResposta{Token: token, Nome: u.Nome, Perfil: u.Perfil}, nil
}

// Esta função só deve ser chamada quando o mutex do estado já estiver travado.
func (e *Estado) usuarioDaSessaoSemLock(token string) (usuario, error) {
	login, existe := e.sessoes[token]
	if !existe || token == "" {
		return usuario{}, errors.New("faça login antes de continuar")
	}
	return e.usuarios[login], nil
}

// novoToken gera uma chave aleatória para a sessão, sem colocar a senha nela.
func novoToken() (string, error) {
	bytes := make([]byte, 24)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}
