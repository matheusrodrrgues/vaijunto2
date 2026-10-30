package main

import (
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
)

// Durante os testes os logs são descartados. Ao iniciar o programa, um logger
// curto escreve na tela e outro guarda data e hora completas no arquivo.
var registroTela = log.New(io.Discard, "", log.Ltime)
var registroArquivo = log.New(io.Discard, "", log.Ldate|log.Ltime)

func configurarRegistro(caminho string) (*os.File, error) {
	if err := os.MkdirAll(filepath.Dir(caminho), 0755); err != nil {
		return nil, err
	}
	arquivo, err := os.OpenFile(caminho, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return nil, err
	}
	registroTela = log.New(os.Stdout, "", log.Ltime)
	registroArquivo = log.New(arquivo, "", log.Ldate|log.Ltime)
	return arquivo, nil
}

func registrar(formato string, valores ...any) {
	mensagem := fmt.Sprintf(formato, valores...)
	registroTela.Print(mensagem)
	registroArquivo.Print(mensagem)
}

// Requisições individuais ficam apenas no arquivo para não poluir a tela.
func registrarTecnico(formato string, valores ...any) {
	registroArquivo.Printf(formato, valores...)
}

func descartarRegistro() {
	registroTela = log.New(io.Discard, "", log.Ltime)
	registroArquivo = log.New(io.Discard, "", log.Ldate|log.Ltime)
}
