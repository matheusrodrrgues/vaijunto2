package main

import (
	"path/filepath"
	"testing"
)

func TestConfiguracaoSeparaDiretoriosPorEmpresa(t *testing.T) {
	t.Setenv("VAIJUNTO_EMPRESA_ID", "empresa-a")
	t.Setenv("VAIJUNTO_EMPRESA_NOME", "Empresa A")
	t.Setenv("VAIJUNTO_ADDR", "")
	t.Setenv("VAIJUNTO_DADOS_DIR", "")
	t.Setenv("VAIJUNTO_LOGS_DIR", "")
	t.Setenv("VAIJUNTO_USUARIOS", "")
	t.Setenv("VAIJUNTO_ESTADO", "")
	t.Setenv("VAIJUNTO_LOG", "")

	configuracao, err := carregarConfiguracaoServidor()
	if err != nil {
		t.Fatal(err)
	}
	if configuracao.EmpresaID != "empresa-a" || configuracao.EmpresaNome != "Empresa A" {
		t.Fatalf("identidade inesperada: %#v", configuracao)
	}
	if configuracao.ArquivoUsuarios != filepath.Join("dados", "empresa-a", "usuarios.json") {
		t.Fatalf("arquivo de usuários inesperado: %s", configuracao.ArquivoUsuarios)
	}
	if configuracao.ArquivoEstado != filepath.Join("dados", "empresa-a", "estado.json") {
		t.Fatalf("arquivo de estado inesperado: %s", configuracao.ArquivoEstado)
	}
	if configuracao.ArquivoLog != filepath.Join("logs", "empresa-a", "servidor.log") {
		t.Fatalf("arquivo de log inesperado: %s", configuracao.ArquivoLog)
	}
}

func TestConfiguracaoAceitaCaminhosExplicitos(t *testing.T) {
	pasta := t.TempDir()
	t.Setenv("VAIJUNTO_EMPRESA_ID", "empresa-b")
	t.Setenv("VAIJUNTO_EMPRESA_NOME", "Empresa B")
	t.Setenv("VAIJUNTO_DADOS_DIR", filepath.Join(pasta, "dados"))
	t.Setenv("VAIJUNTO_LOGS_DIR", filepath.Join(pasta, "logs"))
	t.Setenv("VAIJUNTO_USUARIOS", "")
	t.Setenv("VAIJUNTO_ESTADO", "")
	t.Setenv("VAIJUNTO_LOG", "")

	configuracao, err := carregarConfiguracaoServidor()
	if err != nil {
		t.Fatal(err)
	}
	if configuracao.ArquivoEstado != filepath.Join(pasta, "dados", "estado.json") {
		t.Fatalf("diretório explícito ignorado: %s", configuracao.ArquivoEstado)
	}
}

func TestConfiguracaoRecusaIDInseguro(t *testing.T) {
	t.Setenv("VAIJUNTO_EMPRESA_ID", "../empresa")
	t.Setenv("VAIJUNTO_EMPRESA_NOME", "Empresa")
	if _, err := carregarConfiguracaoServidor(); err == nil {
		t.Fatal("ID com travessia de diretório deveria ser recusado")
	}
}
