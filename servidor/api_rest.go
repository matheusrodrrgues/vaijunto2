package main

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync/atomic"
	"time"

	"vaijunto/comum"
)

type APIREST struct {
	configuracao ConfiguracaoServidor
	cliente      *ClienteRESTConsorcio
	estado       *Estado
}

func NovaAPIREST(configuracao ConfiguracaoServidor, estado *Estado) http.Handler {
	api := &APIREST{
		configuracao: configuracao,
		cliente:      NovoClienteRESTConsorcio(configuracao.TimeoutREST),
		estado:       estado,
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/health", api.saudeEmpresa)
	mux.HandleFunc("/api/v1/consortium/health", api.saudeConsorcio)
	mux.HandleFunc("/api/v1/rides/search", api.buscarTrechos)
	mux.HandleFunc("/api/v1/reservations/prepare", api.prepararReserva)
	mux.HandleFunc("/api/v1/reservations/commit", api.confirmarReserva)
	mux.HandleFunc("/api/v1/reservations/abort", api.abortarReserva)
	mux.HandleFunc("/api/v1/reservations/status", api.consultarReserva)
	mux.HandleFunc("/", recursoRESTNaoEncontrado)
	return metadadosRequisicaoREST(mux)
}

func (a *APIREST) prepararReserva(w http.ResponseWriter, r *http.Request) {
	if !permitirMetodo(w, r, http.MethodPost) {
		return
	}
	var dados comum.PrepararReservaREST
	if err := lerJSONREST(r, &dados); err != nil {
		escreverErroREST(w, http.StatusBadRequest, "requisicao_invalida", err.Error())
		return
	}
	resposta, err := a.estado.PrepararTransacao(dados)
	if err != nil {
		escreverErroREST(w, http.StatusConflict, "preparacao_recusada", err.Error())
		return
	}
	escreverJSON(w, http.StatusOK, resposta)
}

func (a *APIREST) confirmarReserva(w http.ResponseWriter, r *http.Request) {
	if !permitirMetodo(w, r, http.MethodPost) {
		return
	}
	dados, ok := lerIdentificacaoTransacao(w, r)
	if !ok {
		return
	}
	resposta, err := a.estado.ConfirmarTransacao(dados.TransacaoID)
	if err != nil {
		escreverErroREST(w, http.StatusConflict, "confirmacao_recusada", err.Error())
		return
	}
	escreverJSON(w, http.StatusOK, resposta)
}

func (a *APIREST) abortarReserva(w http.ResponseWriter, r *http.Request) {
	if !permitirMetodo(w, r, http.MethodPost) {
		return
	}
	dados, ok := lerIdentificacaoTransacao(w, r)
	if !ok {
		return
	}
	resposta, err := a.estado.AbortarTransacao(dados.TransacaoID)
	if err != nil {
		escreverErroREST(w, http.StatusConflict, "cancelamento_recusado", err.Error())
		return
	}
	escreverJSON(w, http.StatusOK, resposta)
}

func (a *APIREST) consultarReserva(w http.ResponseWriter, r *http.Request) {
	if !permitirMetodo(w, r, http.MethodGet) {
		return
	}
	transacaoID := strings.TrimSpace(r.URL.Query().Get("transaction_id"))
	if transacaoID == "" {
		escreverErroREST(w, http.StatusBadRequest, "requisicao_invalida", "transaction_id é obrigatório")
		return
	}
	resposta, err := a.estado.ConsultarTransacao(transacaoID)
	if err != nil {
		escreverErroREST(w, http.StatusNotFound, "transacao_nao_encontrada", err.Error())
		return
	}
	escreverJSON(w, http.StatusOK, resposta)
}

func lerIdentificacaoTransacao(w http.ResponseWriter, r *http.Request) (comum.IdentificarTransacaoREST, bool) {
	var dados comum.IdentificarTransacaoREST
	if err := lerJSONREST(r, &dados); err != nil || strings.TrimSpace(dados.TransacaoID) == "" {
		mensagem := "transacao_id é obrigatório"
		if err != nil {
			mensagem = err.Error()
		}
		escreverErroREST(w, http.StatusBadRequest, "requisicao_invalida", mensagem)
		return dados, false
	}
	return dados, true
}

func lerJSONREST(r *http.Request, destino any) error {
	decoder := json.NewDecoder(io.LimitReader(r.Body, limiteRespostaREST))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(destino); err != nil {
		return fmt.Errorf("JSON inválido: %w", err)
	}
	return nil
}

func (a *APIREST) buscarTrechos(w http.ResponseWriter, r *http.Request) {
	if !permitirMetodo(w, r, http.MethodGet) {
		return
	}
	data := strings.TrimSpace(r.URL.Query().Get("date"))
	if data != "" {
		if _, err := time.ParseInLocation("2006-01-02", data, time.Local); err != nil {
			escreverErroREST(w, http.StatusBadRequest, "data_invalida", "date deve estar no formato AAAA-MM-DD")
			return
		}
	}
	if a.estado == nil {
		escreverErroREST(w, http.StatusServiceUnavailable, "estado_indisponivel", "estado da empresa indisponível")
		return
	}
	escreverJSON(w, http.StatusOK, comum.RespostaBuscaTrechosREST{
		EmpresaID: a.configuracao.EmpresaID,
		Trechos:   a.estado.ListarTrechosDisponiveisREST(data),
	})
}

func (a *APIREST) saudeEmpresa(w http.ResponseWriter, r *http.Request) {
	if !permitirMetodo(w, r, http.MethodGet) {
		return
	}
	escreverJSON(w, http.StatusOK, comum.SaudeEmpresa{
		EmpresaID:   a.configuracao.EmpresaID,
		EmpresaNome: a.configuracao.EmpresaNome,
		Status:      comum.StatusEmpresaOnline,
	})
}

func (a *APIREST) saudeConsorcio(w http.ResponseWriter, r *http.Request) {
	if !permitirMetodo(w, r, http.MethodGet) {
		return
	}

	pares := a.cliente.ConsultarPares(r.Context(), a.configuracao.Pares)
	escreverJSON(w, http.StatusOK, comum.SaudeConsorcio{
		EmpresaID: a.configuracao.EmpresaID,
		Status:    comum.StatusEmpresaOnline,
		Pares:     pares,
	})
}

func recursoRESTNaoEncontrado(w http.ResponseWriter, _ *http.Request) {
	escreverErroREST(w, http.StatusNotFound, "recurso_nao_encontrado", "recurso REST não encontrado")
}

func permitirMetodo(w http.ResponseWriter, r *http.Request, permitido string) bool {
	if r.Method == permitido {
		return true
	}
	w.Header().Set("Allow", permitido)
	escreverErroREST(w, http.StatusMethodNotAllowed, "metodo_nao_permitido", "método HTTP não permitido")
	return false
}

func escreverJSON(w http.ResponseWriter, status int, valor any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(valor); err != nil {
		registrarTecnico("REST_RESPOSTA_JSON_FALHOU erro=%q", err.Error())
	}
}

func escreverErroREST(w http.ResponseWriter, status int, codigo, mensagem string) {
	escreverJSON(w, status, comum.RespostaErroREST{
		Erro: comum.ErroREST{Codigo: codigo, Mensagem: mensagem},
	})
}

func metadadosRequisicaoREST(proximo http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestID := strings.TrimSpace(r.Header.Get(comum.CabecalhoRequestID))
		if requestID == "" || len(requestID) > 128 {
			requestID = novoIDRequisicaoHTTP()
		}
		w.Header().Set(comum.CabecalhoRequestID, requestID)
		proximo.ServeHTTP(w, r)
	})
}

var contadorRequestID atomic.Uint64

func novoIDRequisicaoHTTP() string {
	bytes := make([]byte, 12)
	if _, err := rand.Read(bytes); err == nil {
		return "req-" + hex.EncodeToString(bytes)
	}
	return fmt.Sprintf("req-%d-%d", time.Now().UnixNano(), contadorRequestID.Add(1))
}

func novoServidorHTTP(configuracao ConfiguracaoServidor, estado *Estado) *http.Server {
	return &http.Server{
		Handler:           NovaAPIREST(configuracao, estado),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
}
