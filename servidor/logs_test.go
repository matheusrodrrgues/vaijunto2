package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLogNaoGravaSenhaNemToken(t *testing.T) {
	caminho := filepath.Join(t.TempDir(), "servidor.log")
	arquivo, err := configurarRegistro(caminho)
	if err != nil {
		t.Fatal(err)
	}
	defer descartarRegistro()

	estado := NovoEstado()
	_, _ = estado.Login("motorista", "senha-super-secreta")
	login, err := estado.Login("motorista", "1234")
	if err != nil {
		t.Fatal(err)
	}
	if err := arquivo.Close(); err != nil {
		t.Fatal(err)
	}

	conteudoBytes, err := os.ReadFile(caminho)
	if err != nil {
		t.Fatal(err)
	}
	conteudo := string(conteudoBytes)
	if strings.Contains(conteudo, "senha-super-secreta") || strings.Contains(conteudo, "1234") {
		t.Fatal("uma senha apareceu no arquivo de log")
	}
	if strings.Contains(conteudo, login.Token) {
		t.Fatal("um token apareceu no arquivo de log")
	}
	if !strings.Contains(conteudo, "resultado=falha") || !strings.Contains(conteudo, "resultado=sucesso") {
		t.Fatal("os resultados de login não foram registrados")
	}
}
