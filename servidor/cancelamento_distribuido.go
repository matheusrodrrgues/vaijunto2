package main

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"vaijunto/comum"
)

// InvalidarReservaDistribuida recebe do proprietário de uma carona a notícia
// de que o motorista a cancelou. O coordenador persiste a decisão antes de
// desfazer os assentos nas empresas participantes.
func (e *Estado) InvalidarReservaDistribuida(dados comum.InvalidarReservaREST) (comum.RespostaTransacaoREST, error) {
	if strings.TrimSpace(dados.TransacaoID) == "" || strings.TrimSpace(dados.EmpresaOrigemID) == "" || strings.TrimSpace(dados.CaronaID) == "" {
		return comum.RespostaTransacaoREST{}, errors.New("transação, empresa de origem e carona são obrigatórias")
	}

	e.mutex.Lock()
	registro, existe := e.coordenacoes[dados.TransacaoID]
	if !existe {
		e.mutex.Unlock()
		return comum.RespostaTransacaoREST{}, errors.New("coordenação não encontrada")
	}
	if !reservaUsaCaronaDaEmpresa(&registro.Reserva, dados.EmpresaOrigemID, dados.CaronaID) {
		e.mutex.Unlock()
		return comum.RespostaTransacaoREST{}, errors.New("a carona informada não participa desta reserva")
	}
	if registro.Status == statusCoordenacaoAbortada && registro.Reserva.Status == comum.StatusCanceladaMotorista {
		resposta := respostaCancelamentoDistribuido(e.empresaID, dados.TransacaoID)
		e.mutex.Unlock()
		return resposta, nil
	}
	if registro.EmProcessamento {
		e.mutex.Unlock()
		return comum.RespostaTransacaoREST{}, errors.New("a coordenação ainda está em processamento")
	}

	statusAnterior := registro.Status
	statusReservaAnterior := registro.Reserva.Status
	registro.Status = statusCoordenacaoAbort
	registro.Reserva.Status = comum.StatusCanceladaMotorista
	registro.UltimoErro = "carona cancelada pelo motorista"
	registro.AtualizadaEm = time.Now()
	registro.EmProcessamento = true
	var reservaLocal *comum.Reserva
	var statusLocalAnterior string
	if reserva, ok := e.reservas[registro.Reserva.ID]; ok {
		reservaLocal = reserva
		statusLocalAnterior = reserva.Status
		reserva.Status = comum.StatusCanceladaMotorista
	}
	if err := e.salvarEstadoSemLock(); err != nil {
		registro.Status = statusAnterior
		registro.Reserva.Status = statusReservaAnterior
		registro.EmProcessamento = false
		if reservaLocal != nil {
			reservaLocal.Status = statusLocalAnterior
		}
		e.mutex.Unlock()
		return comum.RespostaTransacaoREST{}, errors.New("não foi possível persistir o cancelamento distribuído")
	}
	copia := copiarCoordenacao(registro)
	e.mutex.Unlock()

	if err := e.aplicarAbortCoordenacao(copia); err != nil {
		e.registrarErroRecuperacao(dados.TransacaoID, err)
		return comum.RespostaTransacaoREST{}, fmt.Errorf("cancelamento em recuperação: %w", err)
	}
	if err := e.concluirAbortoCoordenacao(dados.TransacaoID); err != nil {
		e.registrarErroRecuperacao(dados.TransacaoID, err)
		return comum.RespostaTransacaoREST{}, fmt.Errorf("cancelamento em recuperação: %w", err)
	}
	registrar("RESERVA_INVALIDADA_PELO_MOTORISTA transacao=%s empresa=%s carona=%s", dados.TransacaoID, dados.EmpresaOrigemID, dados.CaronaID)
	return respostaCancelamentoDistribuido(e.empresaID, dados.TransacaoID), nil
}

func reservaUsaCaronaDaEmpresa(reserva *comum.Reserva, empresaID, caronaID string) bool {
	for _, trecho := range reserva.Itinerario.Trechos {
		if trecho.EmpresaID == empresaID && trecho.CaronaID == caronaID {
			return true
		}
	}
	return false
}

func respostaCancelamentoDistribuido(empresaID, transacaoID string) comum.RespostaTransacaoREST {
	return comum.RespostaTransacaoREST{
		EmpresaID: empresaID, TransacaoID: transacaoID, Status: comum.StatusTransacaoAbortada,
	}
}

// processarCancelamentoCaronaPendente entrega uma invalidação ao coordenador.
// A entrada só é removida depois da resposta, permitindo nova tentativa após
// falha de rede ou reinicialização do servidor proprietário da carona.
func (e *Estado) processarCancelamentoCaronaPendente(transacaoID string) error {
	e.mutex.Lock()
	pendente, existe := e.cancelamentosPendentes[transacaoID]
	if !existe {
		e.mutex.Unlock()
		return nil
	}
	if pendente.EmProcessamento {
		e.mutex.Unlock()
		return errors.New("cancelamento já está em processamento")
	}
	pendente.EmProcessamento = true
	dados := comum.InvalidarReservaREST{
		TransacaoID: pendente.TransacaoID, EmpresaOrigemID: e.empresaID, CaronaID: pendente.CaronaID,
	}
	coordenadorID := pendente.CoordenadorID
	cliente := e.clienteREST
	par, parEncontrado := e.encontrarParSemLock(coordenadorID)
	e.mutex.Unlock()

	var err error
	if coordenadorID == e.empresaID {
		_, err = e.InvalidarReservaDistribuida(dados)
	} else if !parEncontrado {
		err = fmt.Errorf("coordenador %s não está configurado como par", coordenadorID)
	} else {
		err = cliente.InvalidarReserva(context.Background(), par, dados)
	}

	e.mutex.Lock()
	defer e.mutex.Unlock()
	pendente, existe = e.cancelamentosPendentes[transacaoID]
	if !existe {
		return err
	}
	if err != nil {
		pendente.EmProcessamento = false
		pendente.UltimoErro = err.Error()
		pendente.AtualizadaEm = time.Now()
		_ = e.salvarEstadoSemLock()
		return err
	}
	delete(e.cancelamentosPendentes, transacaoID)
	if erroPersistencia := e.salvarEstadoSemLock(); erroPersistencia != nil {
		pendente.EmProcessamento = false
		pendente.UltimoErro = erroPersistencia.Error()
		pendente.AtualizadaEm = time.Now()
		e.cancelamentosPendentes[transacaoID] = pendente
		return erroPersistencia
	}
	return nil
}

func (e *Estado) encontrarParSemLock(empresaID string) (EmpresaRemota, bool) {
	for _, par := range e.pares {
		if par.ID == empresaID {
			return par, true
		}
	}
	return EmpresaRemota{}, false
}

func (e *Estado) recuperarCancelamentosCaronaPendentes() {
	e.mutex.Lock()
	ids := make([]string, 0, len(e.cancelamentosPendentes))
	for id, pendente := range e.cancelamentosPendentes {
		if !pendente.EmProcessamento {
			ids = append(ids, id)
		}
	}
	e.mutex.Unlock()
	for _, id := range ids {
		_ = e.processarCancelamentoCaronaPendente(id)
	}
}
