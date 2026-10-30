package main

import (
	"context"
	"errors"
	"fmt"
	"time"

	"vaijunto/comum"
)

var (
	errParticipanteAbortado = errors.New("participante já abortou a transação")
	errQuedaSimulada        = errors.New("queda simulada do coordenador")
)

func (e *Estado) registrarDecisaoCommit(
	transacaoID, itinerarioID, passageiroLogin, passageiroNome string,
	itinerario comum.Itinerario,
	participantes []participanteReserva,
) (*transacaoCoordenador, error) {
	e.mutex.Lock()
	defer e.mutex.Unlock()

	e.proximaReserva++
	reserva := comum.Reserva{
		ID: fmt.Sprintf("RES-%03d", e.proximaReserva), TransacaoID: transacaoID,
		Passageiro: passageiroLogin, NomePassageiro: passageiroNome,
		Status: comum.StatusReservaConfirmada, CriadaEm: time.Now(), Itinerario: itinerario,
	}
	if reserva.Itinerario.Vagas > 0 {
		reserva.Itinerario.Vagas--
	}
	registro := &transacaoCoordenador{
		ID: transacaoID, Status: statusCoordenacaoCommit, ItinerarioID: itinerarioID,
		Reserva: reserva, Participantes: persistirParticipantes(participantes),
		AtualizadaEm: time.Now(), EmProcessamento: true,
	}
	e.coordenacoes[registro.ID] = registro
	if err := e.salvarEstadoSemLock(); err != nil {
		delete(e.coordenacoes, registro.ID)
		e.proximaReserva--
		return nil, errors.New("não foi possível registrar a decisão da transação")
	}
	registrar("COORDENADOR_DECISAO transacao=%s decisao=commit participantes=%d", registro.ID, len(registro.Participantes))
	return copiarCoordenacao(registro), nil
}

func persistirParticipantes(participantes []participanteReserva) []participanteCoordenacao {
	resultado := make([]participanteCoordenacao, 0, len(participantes))
	for _, participante := range participantes {
		resultado = append(resultado, participanteCoordenacao{
			EmpresaID: participante.empresaID, URLBase: participante.par.URLBase, Local: participante.local,
		})
	}
	return resultado
}

func (e *Estado) aplicarCommitCoordenacao(registro *transacaoCoordenador) error {
	for indice, participante := range registro.Participantes {
		err := e.confirmarParticipante(registro.ID, participante)
		if err != nil {
			return err
		}
		e.mutex.Lock()
		gancho := e.ganchoAposCommit
		e.mutex.Unlock()
		if gancho != nil {
			if err := gancho(indice + 1); err != nil {
				return err
			}
		}
	}
	return nil
}

func (e *Estado) confirmarParticipante(transacaoID string, participante participanteCoordenacao) error {
	var err error
	if participante.Local {
		_, err = e.ConfirmarTransacao(transacaoID)
	} else {
		cliente := e.clienteRESTAtual()
		err = cliente.ConfirmarReserva(context.Background(), participante.remoto(), transacaoID)
	}
	if err == nil {
		return nil
	}

	status, erroConsulta := e.consultarParticipante(transacaoID, participante)
	if erroConsulta != nil {
		return err
	}
	switch status.Status {
	case comum.StatusTransacaoConfirmada:
		return nil
	case comum.StatusTransacaoAbortada:
		return errParticipanteAbortado
	default:
		return err
	}
}

func (e *Estado) consultarParticipante(transacaoID string, participante participanteCoordenacao) (comum.RespostaTransacaoREST, error) {
	if participante.Local {
		return e.ConsultarTransacao(transacaoID)
	}
	return e.clienteRESTAtual().ConsultarReserva(context.Background(), participante.remoto(), transacaoID)
}

func (e *Estado) aplicarAbortCoordenacao(registro *transacaoCoordenador) error {
	var primeiroErro error
	for _, participante := range registro.Participantes {
		var err error
		if participante.Local {
			_, err = e.AbortarTransacao(registro.ID)
		} else {
			err = e.clienteRESTAtual().AbortarReserva(context.Background(), participante.remoto(), registro.ID)
		}
		if err != nil && primeiroErro == nil {
			primeiroErro = err
		}
	}
	return primeiroErro
}

func (e *Estado) concluirCoordenacao(transacaoID string) (comum.Reserva, error) {
	e.mutex.Lock()
	defer e.mutex.Unlock()
	registro, ok := e.coordenacoes[transacaoID]
	if !ok {
		return comum.Reserva{}, errors.New("coordenação não encontrada")
	}
	if existente, ok := e.reservas[registro.Reserva.ID]; ok {
		registro.Status = statusCoordenacaoConcluida
		registro.EmProcessamento = false
		return *existente, nil
	}

	reserva := registro.Reserva
	e.reservas[reserva.ID] = &reserva
	statusAnterior := registro.Status
	registro.Status = statusCoordenacaoConcluida
	registro.UltimoErro = ""
	registro.AtualizadaEm = time.Now()
	registro.EmProcessamento = false
	if pendente, ok := e.itinerarios[registro.ItinerarioID]; ok {
		pendente.Usado = true
		pendente.EmProcessamento = false
	}
	if err := e.salvarEstadoSemLock(); err != nil {
		delete(e.reservas, reserva.ID)
		registro.Status = statusAnterior
		registro.EmProcessamento = false
		return comum.Reserva{}, errors.New("não foi possível persistir a conclusão da coordenação")
	}
	registrar("COORDENADOR_CONCLUIU transacao=%s reserva=%s", registro.ID, reserva.ID)
	return reserva, nil
}

func (e *Estado) decidirAbortarCoordenacao(transacaoID, motivo string) error {
	e.mutex.Lock()
	defer e.mutex.Unlock()
	registro, ok := e.coordenacoes[transacaoID]
	if !ok {
		return errors.New("coordenação não encontrada")
	}
	registro.Status = statusCoordenacaoAbort
	registro.UltimoErro = motivo
	registro.AtualizadaEm = time.Now()
	if err := e.salvarEstadoSemLock(); err != nil {
		return err
	}
	registrar("COORDENADOR_DECISAO transacao=%s decisao=abort motivo=%q", transacaoID, motivo)
	return nil
}

func (e *Estado) concluirAbortoCoordenacao(transacaoID string) error {
	e.mutex.Lock()
	defer e.mutex.Unlock()
	registro, ok := e.coordenacoes[transacaoID]
	if !ok {
		return errors.New("coordenação não encontrada")
	}
	registro.Status = statusCoordenacaoAbortada
	registro.EmProcessamento = false
	registro.AtualizadaEm = time.Now()
	if pendente, ok := e.itinerarios[registro.ItinerarioID]; ok {
		pendente.EmProcessamento = false
	}
	if err := e.salvarEstadoSemLock(); err != nil {
		return err
	}
	return nil
}

func (e *Estado) registrarErroRecuperacao(transacaoID string, err error) {
	e.mutex.Lock()
	defer e.mutex.Unlock()
	if registro, ok := e.coordenacoes[transacaoID]; ok {
		registro.UltimoErro = err.Error()
		registro.AtualizadaEm = time.Now()
		registro.EmProcessamento = false
		_ = e.salvarEstadoSemLock()
	}
}

func (e *Estado) iniciarRecuperacaoCoordenador() {
	e.mutex.Lock()
	if e.recuperacaoIniciada {
		e.mutex.Unlock()
		return
	}
	e.recuperacaoIniciada = true
	e.mutex.Unlock()

	go func() {
		e.recuperarCoordenacoesPendentes()
		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()
		for range ticker.C {
			e.recuperarCoordenacoesPendentes()
		}
	}()
}

func (e *Estado) recuperarCoordenacoesPendentes() {
	e.mutex.Lock()
	ids := make([]string, 0)
	for id, registro := range e.coordenacoes {
		if registro.EmProcessamento {
			continue
		}
		if registro.Status == statusCoordenacaoCommit || registro.Status == statusCoordenacaoAbort {
			registro.EmProcessamento = true
			ids = append(ids, id)
		}
	}
	e.mutex.Unlock()

	for _, id := range ids {
		e.recuperarCoordenacao(id)
	}
}

func (e *Estado) recuperarCoordenacao(transacaoID string) {
	e.mutex.Lock()
	registro, ok := e.coordenacoes[transacaoID]
	if !ok {
		e.mutex.Unlock()
		return
	}
	copia := copiarCoordenacao(registro)
	e.mutex.Unlock()

	switch copia.Status {
	case statusCoordenacaoCommit:
		err := e.aplicarCommitCoordenacao(copia)
		if errors.Is(err, errParticipanteAbortado) {
			if erroDecisao := e.decidirAbortarCoordenacao(copia.ID, err.Error()); erroDecisao != nil {
				e.registrarErroRecuperacao(copia.ID, erroDecisao)
				return
			}
			e.mutex.Lock()
			copia = copiarCoordenacao(e.coordenacoes[copia.ID])
			e.mutex.Unlock()
			if erroAbort := e.aplicarAbortCoordenacao(copia); erroAbort != nil {
				e.registrarErroRecuperacao(copia.ID, erroAbort)
				return
			}
			_ = e.concluirAbortoCoordenacao(copia.ID)
			return
		}
		if err != nil {
			e.registrarErroRecuperacao(copia.ID, err)
			return
		}
		if _, err := e.concluirCoordenacao(copia.ID); err != nil {
			e.registrarErroRecuperacao(copia.ID, err)
		}
	case statusCoordenacaoAbort:
		if err := e.aplicarAbortCoordenacao(copia); err != nil {
			e.registrarErroRecuperacao(copia.ID, err)
			return
		}
		if err := e.concluirAbortoCoordenacao(copia.ID); err != nil {
			e.registrarErroRecuperacao(copia.ID, err)
		}
	}
}

func (e *Estado) clienteRESTAtual() *ClienteRESTConsorcio {
	e.mutex.Lock()
	defer e.mutex.Unlock()
	return e.clienteREST
}

func (p participanteCoordenacao) remoto() EmpresaRemota {
	return EmpresaRemota{ID: p.EmpresaID, URLBase: p.URLBase}
}

func copiarCoordenacao(origem *transacaoCoordenador) *transacaoCoordenador {
	copia := *origem
	copia.Participantes = append([]participanteCoordenacao(nil), origem.Participantes...)
	return &copia
}
