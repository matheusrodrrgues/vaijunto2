package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"vaijunto/comum"
)

func TestCadastroSalvoEmJSON(t *testing.T) {
	caminho := filepath.Join(t.TempDir(), "usuarios.json")
	estado, err := NovoEstadoComArquivo(caminho)
	if err != nil {
		t.Fatal(err)
	}

	dados := comum.CadastroDados{
		Nome: "Ana Silva", Usuario: "Ana123", Senha: "senha456", Perfil: comum.PerfilPassageiro,
	}
	if err := estado.Cadastrar(dados); err != nil {
		t.Fatal(err)
	}
	if err := estado.Cadastrar(dados); err == nil {
		t.Fatal("o cadastro repetido deveria ser recusado")
	}

	conteudo, err := os.ReadFile(caminho)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(conteudo), dados.Senha) {
		t.Fatal("a senha foi gravada sem protecao no JSON")
	}

	recarregado, err := NovoEstadoComArquivo(caminho)
	if err != nil {
		t.Fatal(err)
	}
	login, err := recarregado.Login("ANA123", dados.Senha)
	if err != nil {
		t.Fatal(err)
	}
	if login.Nome != dados.Nome || login.Perfil != comum.PerfilPassageiro {
		t.Fatalf("conta recarregada incorretamente: %+v", login)
	}
}
