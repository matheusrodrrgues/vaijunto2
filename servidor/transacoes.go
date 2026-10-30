package main

import (
	"errors"
	"strconv"
	"strings"
	"time"

	"vaijunto/comum"
)

const (
	duracaoBloqueioPadrao = 15 * time.Second
	duracaoBloqueioMaxima = 60 * time.Second
)

// PrepararTransacao reserva provisoriamente os assentos da empresa local.
// O mutex do proprietário do trecho estabelece a precedência: vence a
// primeira preparação efetivamente processada, sem comparar relógios externos.
func (e *Estado) PrepararTransacao(dados comum.PrepararReservaREST) (comum.RespostaTransacaoREST, error) {
	if strings.TrimSpace(dados.TransacaoID) == "" || len(dados.TransacaoID) > 160 {
		return comum.RespostaTransacaoREST{}, errors.New("identificador de transação inválido")
	}
	if strings.TrimSpace(dados.CoordenadorID) == "" || len(dados.Trechos) == 0 {
		return comum.RespostaTransacaoREST{}, errors.New("coordenador e trechos são obrigatórios")
	}
	duracao := time.Duration(dados.DuracaoBloqueioMS) * time.Millisecond
	if duracao <= 0 {
		duracao = duracaoBloqueioPadrao
	}
	if duracao > duracaoBloqueioMaxima {
		duracao = duracaoBloqueioMaxima
	}

	e.mutex.Lock()
	if existente, ok := e.transacoes[dados.TransacaoID]; ok {
		resposta := respostaTransacao(e.empresaID, existente)
		coordenadorID := existente.CoordenadorID
		status := existente.Status
		trechosExistentes := append([]comum.TrechoTransacaoREST(nil), existente.Trechos...)
		e.mutex.Unlock()
		if coordenadorID != dados.CoordenadorID || !mesmosTrechosTransacao(trechosExistentes, dados.Trechos) {
			return resposta, errors.New("chave de idempotência reutilizada com dados diferentes")
		}
		if status == comum.StatusTransacaoAbortada {
			return resposta, errors.New("transação já abortada")
		}
		return resposta, nil
	}

	trechos, err := e.validarTrechosParaPreparacaoSemLock(dados.Trechos)
	if err != nil {
		e.mutex.Unlock()
		return comum.RespostaTransacaoREST{}, err
	}
	for _, trecho := range trechos {
		e.caronas[trecho.CaronaID].DisponiveisPorTrecho[trecho.Indice]--
	}
	transacao := &transacaoParticipante{
		ID: dados.TransacaoID, CoordenadorID: dados.CoordenadorID,
		Status: comum.StatusTransacaoPreparada, Trechos: trechos,
		ExpiraEm: time.Now().Add(duracao),
	}
	e.transacoes[transacao.ID] = transacao
	if err := e.salvarEstadoSemLock(); err != nil {
		delete(e.transacoes, transacao.ID)
		e.liberarTrechosTransacaoSemLock(trechos)
		e.mutex.Unlock()
		return comum.RespostaTransacaoREST{}, errors.New("não foi possível persistir a preparação")
	}
	resposta := respostaTransacao(e.empresaID, transacao)
	e.mutex.Unlock()

	time.AfterFunc(duracao, func() { e.expirarTransacao(dados.TransacaoID) })
	registrar("TRANSACAO_PREPARADA transacao=%s coordenador=%s trechos=%d", transacao.ID, transacao.CoordenadorID, len(trechos))
	return resposta, nil
}

func mesmosTrechosTransacao(a, b []comum.TrechoTransacaoREST) bool {
	if len(a) != len(b) {
		return false
	}
	contagem := make(map[string]int, len(a))
	for _, trecho := range a {
		contagem[trecho.CaronaID+"|"+strconv.Itoa(trecho.Indice)]++
	}
	for _, trecho := range b {
		chave := trecho.CaronaID + "|" + strconv.Itoa(trecho.Indice)
		contagem[chave]--
		if contagem[chave] < 0 {
			return false
		}
	}
	return true
}

func (e *Estado) ConfirmarTransacao(transacaoID string) (comum.RespostaTransacaoREST, error) {
	e.mutex.Lock()
	defer e.mutex.Unlock()
	transacao, ok := e.transacoes[transacaoID]
	if !ok {
		return comum.RespostaTransacaoREST{}, errors.New("transação não encontrada")
	}
	if transacao.Status == comum.StatusTransacaoAbortada {
		return respostaTransacao(e.empresaID, transacao), errors.New("transação já abortada")
	}
	if transacao.Status == comum.StatusTransacaoConfirmada {
		return respostaTransacao(e.empresaID, transacao), nil
	}
	anterior := transacao.Status
	transacao.Status = comum.StatusTransacaoConfirmada
	if err := e.salvarEstadoSemLock(); err != nil {
		transacao.Status = anterior
		return comum.RespostaTransacaoREST{}, errors.New("não foi possível persistir a confirmação")
	}
	registrar("TRANSACAO_CONFIRMADA transacao=%s coordenador=%s", transacao.ID, transacao.CoordenadorID)
	return respostaTransacao(e.empresaID, transacao), nil
}

// AbortarTransacao é idempotente e também compensa uma confirmação cujo
// retorno tenha se perdido. Assim o coordenador pode desfazer todos os
// participantes quando não obtém confirmação integral.
func (e *Estado) AbortarTransacao(transacaoID string) (comum.RespostaTransacaoREST, error) {
	e.mutex.Lock()
	defer e.mutex.Unlock()
	transacao, ok := e.transacoes[transacaoID]
	if !ok {
		return comum.RespostaTransacaoREST{
			EmpresaID: e.empresaID, TransacaoID: transacaoID, Status: comum.StatusTransacaoAbortada,
		}, nil
	}
	if transacao.Status == comum.StatusTransacaoAbortada {
		return respostaTransacao(e.empresaID, transacao), nil
	}
	anterior := transacao.Status
	e.liberarTrechosTransacaoSemLock(transacao.Trechos)
	transacao.Status = comum.StatusTransacaoAbortada
	if err := e.salvarEstadoSemLock(); err != nil {
		for _, trecho := range transacao.Trechos {
			e.caronas[trecho.CaronaID].DisponiveisPorTrecho[trecho.Indice]--
		}
		transacao.Status = anterior
		return comum.RespostaTransacaoREST{}, errors.New("não foi possível persistir o cancelamento")
	}
	registrar("TRANSACAO_ABORTADA transacao=%s coordenador=%s", transacao.ID, transacao.CoordenadorID)
	return respostaTransacao(e.empresaID, transacao), nil
}

func (e *Estado) ConsultarTransacao(transacaoID string) (comum.RespostaTransacaoREST, error) {
	e.mutex.Lock()
	defer e.mutex.Unlock()
	transacao, ok := e.transacoes[transacaoID]
	if !ok {
		return comum.RespostaTransacaoREST{}, errors.New("transação não encontrada")
	}
	return respostaTransacao(e.empresaID, transacao), nil
}

func (e *Estado) validarTrechosParaPreparacaoSemLock(recebidos []comum.TrechoTransacaoREST) ([]comum.TrechoTransacaoREST, error) {
	chaves := make(map[string]bool)
	trechos := make([]comum.TrechoTransacaoREST, 0, len(recebidos))
	for _, trecho := range recebidos {
		chave := trecho.CaronaID + "|" + strconv.Itoa(trecho.Indice)
		if chaves[chave] {
			continue
		}
		chaves[chave] = true
		carona, ok := e.caronas[trecho.CaronaID]
		if !ok || carona.Cancelada || trecho.Indice < 0 || trecho.Indice >= len(carona.DisponiveisPorTrecho) {
			return nil, errors.New("um dos trechos não está mais disponível")
		}
		if carona.DisponiveisPorTrecho[trecho.Indice] < 1 {
			return nil, errors.New("um dos trechos ficou sem assento")
		}
		trechos = append(trechos, trecho)
	}
	return trechos, nil
}

// recuperarTransacoesPreparadas restaura os temporizadores depois de uma
// reinicialização. Preparações cujo prazo venceu são liberadas imediatamente.
func (e *Estado) recuperarTransacoesPreparadas() error {
	e.mutex.Lock()
	agora := time.Now()
	prazos := make(map[string]time.Duration)
	alterou := false
	for id, transacao := range e.transacoes {
		if transacao.Status != comum.StatusTransacaoPreparada {
			continue
		}
		if !agora.Before(transacao.ExpiraEm) {
			e.liberarTrechosTransacaoSemLock(transacao.Trechos)
			transacao.Status = comum.StatusTransacaoAbortada
			alterou = true
			continue
		}
		prazos[id] = time.Until(transacao.ExpiraEm)
	}
	if alterou {
		if err := e.salvarEstadoSemLock(); err != nil {
			e.mutex.Unlock()
			return err
		}
	}
	e.mutex.Unlock()
	for id, prazo := range prazos {
		id := id
		time.AfterFunc(prazo, func() { e.expirarTransacao(id) })
	}
	return nil
}

func (e *Estado) liberarTrechosTransacaoSemLock(trechos []comum.TrechoTransacaoREST) {
	for _, trecho := range trechos {
		carona, ok := e.caronas[trecho.CaronaID]
		if ok && trecho.Indice >= 0 && trecho.Indice < len(carona.DisponiveisPorTrecho) &&
			carona.DisponiveisPorTrecho[trecho.Indice] < carona.Assentos {
			carona.DisponiveisPorTrecho[trecho.Indice]++
		}
	}
}

func (e *Estado) expirarTransacao(transacaoID string) {
	e.mutex.Lock()
	defer e.mutex.Unlock()
	transacao, ok := e.transacoes[transacaoID]
	if !ok || transacao.Status != comum.StatusTransacaoPreparada || time.Now().Before(transacao.ExpiraEm) {
		return
	}
	e.liberarTrechosTransacaoSemLock(transacao.Trechos)
	transacao.Status = comum.StatusTransacaoAbortada
	if err := e.salvarEstadoSemLock(); err != nil {
		registrarTecnico("TRANSACAO_EXPIRACAO_FALHOU transacao=%s erro=%q", transacaoID, err.Error())
		return
	}
	registrar("TRANSACAO_EXPIRADA transacao=%s", transacaoID)
}

func respostaTransacao(empresaID string, transacao *transacaoParticipante) comum.RespostaTransacaoREST {
	return comum.RespostaTransacaoREST{
		EmpresaID: empresaID, TransacaoID: transacao.ID, Status: transacao.Status,
	}
}
