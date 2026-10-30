package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"vaijunto/comum"
)

func TestAPIRestInformaSaudeEPropagaRequestID(t *testing.T) {
	api := NovaAPIREST(configuracaoRESTTeste("empresa-a", "Empresa A", nil, 200*time.Millisecond))
	requisicao := httptest.NewRequest(http.MethodGet, "/api/v1/health", nil)
	requisicao.Header.Set(comum.CabecalhoRequestID, "req-teste-123")
	resposta := httptest.NewRecorder()

	api.ServeHTTP(resposta, requisicao)

	if resposta.Code != http.StatusOK {
		t.Fatalf("status inesperado: %d", resposta.Code)
	}
	if resposta.Header().Get(comum.CabecalhoRequestID) != "req-teste-123" {
		t.Fatalf("request ID não foi propagado: %q", resposta.Header().Get(comum.CabecalhoRequestID))
	}
	if resposta.Header().Get("Content-Type") != "application/json; charset=utf-8" {
		t.Fatalf("content-type inesperado: %s", resposta.Header().Get("Content-Type"))
	}

	var saude comum.SaudeEmpresa
	if err := json.NewDecoder(resposta.Body).Decode(&saude); err != nil {
		t.Fatal(err)
	}
	if saude.EmpresaID != "empresa-a" || saude.EmpresaNome != "Empresa A" || saude.Status != comum.StatusEmpresaOnline {
		t.Fatalf("resposta de saúde inesperada: %#v", saude)
	}
}

func TestAPIRestRecusaMetodoERecursoDesconhecido(t *testing.T) {
	api := NovaAPIREST(configuracaoRESTTeste("empresa-a", "Empresa A", nil, time.Second))

	respostaMetodo := httptest.NewRecorder()
	api.ServeHTTP(respostaMetodo, httptest.NewRequest(http.MethodPost, "/api/v1/health", nil))
	if respostaMetodo.Code != http.StatusMethodNotAllowed || respostaMetodo.Header().Get("Allow") != http.MethodGet {
		t.Fatalf("método não foi recusado corretamente: status=%d allow=%q", respostaMetodo.Code, respostaMetodo.Header().Get("Allow"))
	}

	respostaRecurso := httptest.NewRecorder()
	api.ServeHTTP(respostaRecurso, httptest.NewRequest(http.MethodGet, "/api/v1/inexistente", nil))
	if respostaRecurso.Code != http.StatusNotFound {
		t.Fatalf("recurso desconhecido retornou %d", respostaRecurso.Code)
	}
}

func TestConsultaConsorcioContinuaComEmpresaIndisponivel(t *testing.T) {
	servidorB := httptest.NewServer(NovaAPIREST(configuracaoRESTTeste("empresa-b", "Empresa B", nil, time.Second)))
	defer servidorB.Close()
	servidorFora := httptest.NewServer(NovaAPIREST(configuracaoRESTTeste("empresa-c", "Empresa C", nil, time.Second)))
	urlFora := servidorFora.URL
	servidorFora.Close()

	pares := []EmpresaRemota{
		{ID: "empresa-b", URLBase: servidorB.URL},
		{ID: "empresa-c", URLBase: urlFora},
	}
	api := NovaAPIREST(configuracaoRESTTeste("empresa-a", "Empresa A", pares, 200*time.Millisecond))
	resposta := httptest.NewRecorder()
	api.ServeHTTP(resposta, httptest.NewRequest(http.MethodGet, "/api/v1/consortium/health", nil))

	if resposta.Code != http.StatusOK {
		t.Fatalf("empresa local deixou de responder: %d", resposta.Code)
	}
	var saude comum.SaudeConsorcio
	if err := json.NewDecoder(resposta.Body).Decode(&saude); err != nil {
		t.Fatal(err)
	}
	if len(saude.Pares) != 2 {
		t.Fatalf("quantidade de pares inesperada: %d", len(saude.Pares))
	}
	if !saude.Pares[0].Disponivel || saude.Pares[0].EmpresaID != "empresa-b" {
		t.Fatalf("empresa B deveria estar disponível: %#v", saude.Pares[0])
	}
	if saude.Pares[1].Disponivel || saude.Pares[1].EmpresaID != "empresa-c" || saude.Pares[1].Erro == "" {
		t.Fatalf("empresa C deveria estar indisponível: %#v", saude.Pares[1])
	}
}

func TestConsultaConsorcioRespeitaTimeout(t *testing.T) {
	servidorLento := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		time.Sleep(200 * time.Millisecond)
		escreverJSON(w, http.StatusOK, comum.SaudeEmpresa{
			EmpresaID: "empresa-b", EmpresaNome: "Empresa B", Status: comum.StatusEmpresaOnline,
		})
	}))
	defer servidorLento.Close()

	api := NovaAPIREST(configuracaoRESTTeste("empresa-a", "Empresa A", []EmpresaRemota{
		{ID: "empresa-b", URLBase: servidorLento.URL},
	}, 40*time.Millisecond))
	inicio := time.Now()
	resposta := httptest.NewRecorder()
	api.ServeHTTP(resposta, httptest.NewRequest(http.MethodGet, "/api/v1/consortium/health", nil))
	duracao := time.Since(inicio)

	if duracao >= 150*time.Millisecond {
		t.Fatalf("timeout não interrompeu a consulta: %s", duracao)
	}
	var saude comum.SaudeConsorcio
	if err := json.NewDecoder(resposta.Body).Decode(&saude); err != nil {
		t.Fatal(err)
	}
	if len(saude.Pares) != 1 || saude.Pares[0].Disponivel || saude.Pares[0].Erro != "tempo limite excedido" {
		t.Fatalf("resultado de timeout inesperado: %#v", saude.Pares)
	}
}

func configuracaoRESTTeste(id, nome string, pares []EmpresaRemota, timeout time.Duration) ConfiguracaoServidor {
	return ConfiguracaoServidor{
		EmpresaID: id, EmpresaNome: nome, EnderecoTCP: ":9000", EnderecoREST: ":8080",
		TimeoutREST: timeout, Pares: pares,
		ArquivoUsuarios: "usuarios.json", ArquivoEstado: "estado.json", ArquivoLog: "servidor.log",
	}
}
