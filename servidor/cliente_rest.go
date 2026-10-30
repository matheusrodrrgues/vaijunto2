package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sort"
	"time"

	"vaijunto/comum"
)

const limiteRespostaREST = 1 << 20 // 1 MiB

// ClienteRESTConsorcio concentra os timeouts e a validação usados na
// comunicação servidor-servidor. Erros de um par são retornados como estado
// indisponível e não encerram a operação da empresa local.
type ClienteRESTConsorcio struct {
	httpClient *http.Client
}

func NovoClienteRESTConsorcio(timeout time.Duration) *ClienteRESTConsorcio {
	return &ClienteRESTConsorcio{
		httpClient: &http.Client{Timeout: timeout},
	}
}

func (c *ClienteRESTConsorcio) ConsultarSaude(ctx context.Context, par EmpresaRemota) comum.SaudePar {
	inicio := time.Now()
	resultado := comum.SaudePar{EmpresaID: par.ID, URLBase: par.URLBase}

	requisicao, err := http.NewRequestWithContext(ctx, http.MethodGet, par.URLBase+"/api/v1/health", nil)
	if err != nil {
		resultado.Erro = err.Error()
		return resultado
	}
	requisicao.Header.Set(comum.CabecalhoRequestID, novoIDRequisicaoHTTP())

	resposta, err := c.httpClient.Do(requisicao)
	resultado.DuracaoMS = time.Since(inicio).Milliseconds()
	if err != nil {
		resultado.Erro = normalizarErroComunicacao(err)
		return resultado
	}
	defer resposta.Body.Close()

	if resposta.StatusCode != http.StatusOK {
		resultado.Erro = fmt.Sprintf("resposta HTTP %d", resposta.StatusCode)
		return resultado
	}

	decoder := json.NewDecoder(io.LimitReader(resposta.Body, limiteRespostaREST))
	var saude comum.SaudeEmpresa
	if err := decoder.Decode(&saude); err != nil {
		resultado.Erro = "resposta JSON inválida"
		return resultado
	}
	if saude.EmpresaID != par.ID || saude.Status != comum.StatusEmpresaOnline {
		resultado.Erro = "identidade ou estado inesperado na resposta"
		return resultado
	}

	resultado.Disponivel = true
	resultado.Status = saude.Status
	return resultado
}

func (c *ClienteRESTConsorcio) ConsultarPares(ctx context.Context, pares []EmpresaRemota) []comum.SaudePar {
	resultados := make(chan comum.SaudePar, len(pares))
	for _, par := range pares {
		par := par
		go func() {
			resultados <- c.ConsultarSaude(ctx, par)
		}()
	}

	lista := make([]comum.SaudePar, 0, len(pares))
	for range pares {
		lista = append(lista, <-resultados)
	}
	sort.Slice(lista, func(i, j int) bool { return lista[i].EmpresaID < lista[j].EmpresaID })
	return lista
}

func normalizarErroComunicacao(err error) string {
	if errors.Is(err, context.DeadlineExceeded) {
		return "tempo limite excedido"
	}
	return "empresa indisponível"
}
