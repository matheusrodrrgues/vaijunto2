package main

import (
	"path/filepath"
	"testing"
	"time"
)

func TestConfiguracaoSeparaDiretoriosPorEmpresa(t *testing.T) {
	t.Setenv("VAIJUNTO_EMPRESA_ID", "empresa-a")
	t.Setenv("VAIJUNTO_EMPRESA_NOME", "Empresa A")
	t.Setenv("VAIJUNTO_ADDR", "")
	t.Setenv("VAIJUNTO_REST_ADDR", "")
	t.Setenv("VAIJUNTO_REST_TIMEOUT", "")
	t.Setenv("VAIJUNTO_PARES", "")
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
	if configuracao.EnderecoREST != ":8080" || configuracao.TimeoutREST != 2*time.Second {
		t.Fatalf("configuração REST inesperada: endereço=%s timeout=%s", configuracao.EnderecoREST, configuracao.TimeoutREST)
	}
}

func TestConfiguracaoAceitaCaminhosExplicitos(t *testing.T) {
	pasta := t.TempDir()
	t.Setenv("VAIJUNTO_EMPRESA_ID", "empresa-b")
	t.Setenv("VAIJUNTO_EMPRESA_NOME", "Empresa B")
	t.Setenv("VAIJUNTO_REST_ADDR", "")
	t.Setenv("VAIJUNTO_REST_TIMEOUT", "")
	t.Setenv("VAIJUNTO_PARES", "")
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

func TestConfiguracaoCarregaParesOrdenados(t *testing.T) {
	t.Setenv("VAIJUNTO_EMPRESA_ID", "empresa-a")
	t.Setenv("VAIJUNTO_EMPRESA_NOME", "Empresa A")
	t.Setenv("VAIJUNTO_REST_TIMEOUT", "750ms")
	t.Setenv("VAIJUNTO_PARES", "empresa-c=http://empresa-c:8080/,empresa-b=http://empresa-b:8080")

	configuracao, err := carregarConfiguracaoServidor()
	if err != nil {
		t.Fatal(err)
	}
	if configuracao.TimeoutREST != 750*time.Millisecond {
		t.Fatalf("timeout inesperado: %s", configuracao.TimeoutREST)
	}
	if len(configuracao.Pares) != 2 || configuracao.Pares[0].ID != "empresa-b" || configuracao.Pares[1].ID != "empresa-c" {
		t.Fatalf("pares não foram ordenados: %#v", configuracao.Pares)
	}
	if configuracao.Pares[1].URLBase != "http://empresa-c:8080" {
		t.Fatalf("barra final não foi normalizada: %s", configuracao.Pares[1].URLBase)
	}
}

func TestConfiguracaoRecusaParLocalEURLInvalida(t *testing.T) {
	casos := []string{
		"empresa-a=http://empresa-a:8080",
		"empresa-b=empresa-b:8080",
		"empresa-b=http://empresa-b:8080,empresa-b=http://outro:8080",
	}
	for _, valor := range casos {
		t.Run(valor, func(t *testing.T) {
			if _, err := interpretarPares(valor, "empresa-a"); err == nil {
				t.Fatalf("configuração deveria ser recusada: %s", valor)
			}
		})
	}
}
