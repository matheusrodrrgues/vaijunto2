package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
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

type resultadoTrechosPar struct {
	EmpresaID string
	Trechos   []comum.TrechoDisponivelREST
	Erro      string
}

// ConsultarTrechos busca somente o catálogo público de um par e rejeita
// respostas cuja identidade ou conteúdo não correspondam à empresa esperada.
func (c *ClienteRESTConsorcio) ConsultarTrechos(ctx context.Context, par EmpresaRemota, data string) ([]comum.TrechoDisponivelREST, error) {
	endpoint, err := url.Parse(par.URLBase + "/api/v1/rides/search")
	if err != nil {
		return nil, err
	}
	if data != "" {
		consulta := endpoint.Query()
		consulta.Set("date", data)
		endpoint.RawQuery = consulta.Encode()
	}

	requisicao, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint.String(), nil)
	if err != nil {
		return nil, err
	}
	requisicao.Header.Set(comum.CabecalhoRequestID, novoIDRequisicaoHTTP())
	resposta, err := c.httpClient.Do(requisicao)
	if err != nil {
		return nil, errors.New(normalizarErroComunicacao(err))
	}
	defer resposta.Body.Close()
	if resposta.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("resposta HTTP %d", resposta.StatusCode)
	}

	var catalogo comum.RespostaBuscaTrechosREST
	decoder := json.NewDecoder(io.LimitReader(resposta.Body, limiteRespostaREST))
	if err := decoder.Decode(&catalogo); err != nil {
		return nil, errors.New("resposta JSON inválida")
	}
	if catalogo.EmpresaID != par.ID {
		return nil, errors.New("identidade inesperada na resposta")
	}
	for _, trecho := range catalogo.Trechos {
		if err := validarTrechoRemoto(trecho, par.ID, data); err != nil {
			return nil, err
		}
	}
	return catalogo.Trechos, nil
}

func (c *ClienteRESTConsorcio) ConsultarTrechosPares(ctx context.Context, pares []EmpresaRemota, data string) ([]comum.TrechoDisponivelREST, []string) {
	canal := make(chan resultadoTrechosPar, len(pares))
	for _, par := range pares {
		par := par
		go func() {
			trechos, err := c.ConsultarTrechos(ctx, par, data)
			resultado := resultadoTrechosPar{EmpresaID: par.ID, Trechos: trechos}
			if err != nil {
				resultado.Erro = err.Error()
			}
			canal <- resultado
		}()
	}

	trechos := make([]comum.TrechoDisponivelREST, 0)
	indisponiveis := make([]string, 0)
	for range pares {
		resultado := <-canal
		if resultado.Erro != "" {
			indisponiveis = append(indisponiveis, resultado.EmpresaID)
			registrar("BUSCA_REMOTA_FALHOU empresa=%s erro=%q", resultado.EmpresaID, resultado.Erro)
			continue
		}
		trechos = append(trechos, resultado.Trechos...)
	}
	sort.Strings(indisponiveis)
	return trechos, indisponiveis
}

func validarTrechoRemoto(t comum.TrechoDisponivelREST, empresaEsperada, dataEsperada string) error {
	if t.EmpresaID != empresaEsperada || strings.TrimSpace(t.CaronaID) == "" || t.Indice < 0 ||
		strings.TrimSpace(t.Origem) == "" || strings.TrimSpace(t.Destino) == "" ||
		t.PrecoCentavos < 0 || t.Vagas < 1 || !t.Chegada.After(t.Saida) {
		return errors.New("trecho remoto inválido")
	}
	if _, err := time.ParseInLocation("2006-01-02", t.Data, time.Local); err != nil {
		return errors.New("data remota inválida")
	}
	if dataEsperada != "" && t.Data != dataEsperada {
		return errors.New("data inesperada na resposta")
	}
	return nil
}

func NovoClienteRESTConsorcio(timeout time.Duration) *ClienteRESTConsorcio {
	return &ClienteRESTConsorcio{
		httpClient: &http.Client{Timeout: timeout},
	}
}

func (c *ClienteRESTConsorcio) PrepararReserva(ctx context.Context, par EmpresaRemota, dados comum.PrepararReservaREST) error {
	resposta, err := c.enviarTransacao(ctx, par, "/api/v1/reservations/prepare", dados, dados.TransacaoID)
	if err != nil {
		return err
	}
	if resposta.Status != comum.StatusTransacaoPreparada && resposta.Status != comum.StatusTransacaoConfirmada {
		return errors.New("estado inesperado na preparação")
	}
	return nil
}

func (c *ClienteRESTConsorcio) ConfirmarReserva(ctx context.Context, par EmpresaRemota, transacaoID string) error {
	resposta, err := c.enviarTransacao(ctx, par, "/api/v1/reservations/commit",
		comum.IdentificarTransacaoREST{TransacaoID: transacaoID}, transacaoID)
	if err != nil {
		return err
	}
	if resposta.Status != comum.StatusTransacaoConfirmada {
		return errors.New("estado inesperado na confirmação")
	}
	return nil
}

func (c *ClienteRESTConsorcio) AbortarReserva(ctx context.Context, par EmpresaRemota, transacaoID string) error {
	resposta, err := c.enviarTransacao(ctx, par, "/api/v1/reservations/abort",
		comum.IdentificarTransacaoREST{TransacaoID: transacaoID}, transacaoID)
	if err != nil {
		return err
	}
	if resposta.Status != comum.StatusTransacaoAbortada {
		return errors.New("estado inesperado no cancelamento")
	}
	return nil
}

func (c *ClienteRESTConsorcio) enviarTransacao(ctx context.Context, par EmpresaRemota, caminho string, dados any, transacaoID string) (comum.RespostaTransacaoREST, error) {
	corpo, err := json.Marshal(dados)
	if err != nil {
		return comum.RespostaTransacaoREST{}, err
	}
	requisicao, err := http.NewRequestWithContext(ctx, http.MethodPost, par.URLBase+caminho, bytes.NewReader(corpo))
	if err != nil {
		return comum.RespostaTransacaoREST{}, err
	}
	requisicao.Header.Set("Content-Type", "application/json")
	requisicao.Header.Set(comum.CabecalhoRequestID, novoIDRequisicaoHTTP())
	requisicao.Header.Set(comum.CabecalhoIdempotencia, transacaoID)
	respostaHTTP, err := c.httpClient.Do(requisicao)
	if err != nil {
		return comum.RespostaTransacaoREST{}, errors.New(normalizarErroComunicacao(err))
	}
	defer respostaHTTP.Body.Close()
	if respostaHTTP.StatusCode != http.StatusOK {
		var respostaErro comum.RespostaErroREST
		_ = json.NewDecoder(io.LimitReader(respostaHTTP.Body, limiteRespostaREST)).Decode(&respostaErro)
		if respostaErro.Erro.Mensagem != "" {
			return comum.RespostaTransacaoREST{}, errors.New(respostaErro.Erro.Mensagem)
		}
		return comum.RespostaTransacaoREST{}, fmt.Errorf("resposta HTTP %d", respostaHTTP.StatusCode)
	}
	var resposta comum.RespostaTransacaoREST
	if err := json.NewDecoder(io.LimitReader(respostaHTTP.Body, limiteRespostaREST)).Decode(&resposta); err != nil {
		return comum.RespostaTransacaoREST{}, errors.New("resposta JSON inválida")
	}
	if resposta.EmpresaID != par.ID || resposta.TransacaoID != transacaoID {
		return comum.RespostaTransacaoREST{}, errors.New("identidade inesperada na resposta")
	}
	return resposta, nil
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
