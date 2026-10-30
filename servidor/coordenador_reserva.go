package main

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"time"

	"vaijunto/comum"
)

type participanteReserva struct {
	empresaID string
	par       EmpresaRemota
	local     bool
	trechos   []comum.TrechoTransacaoREST
}

func (e *Estado) itinerarioExigeTransacaoDistribuida(token, itinerarioID string) (bool, error) {
	e.mutex.Lock()
	defer e.mutex.Unlock()
	u, err := e.usuarioDaSessaoSemLock(token)
	if err != nil {
		return false, err
	}
	pendente, existe := e.itinerarios[itinerarioID]
	if !existe || pendente.Passageiro != u.Login {
		return false, errors.New("itinerário não encontrado")
	}
	for _, trecho := range pendente.Itinerario.Trechos {
		if trecho.EmpresaID != "" && trecho.EmpresaID != e.empresaID {
			return true, nil
		}
	}
	return false, nil
}

// confirmarReservaDistribuida coordena um 2PC sem coordenador fixo: o servidor
// da conta do passageiro coordena somente esta transação. Os participantes são
// preparados em ordem de ID para impedir espera circular entre duas reservas.
func (e *Estado) confirmarReservaDistribuida(token, itinerarioID string) (comum.Reserva, error) {
	e.mutex.Lock()
	u, err := e.usuarioDaSessaoSemLock(token)
	if err != nil {
		e.mutex.Unlock()
		return comum.Reserva{}, err
	}
	pendente, existe := e.itinerarios[itinerarioID]
	if !existe || pendente.Passageiro != u.Login {
		e.mutex.Unlock()
		return comum.Reserva{}, errors.New("itinerário não encontrado")
	}
	if pendente.Usado {
		e.mutex.Unlock()
		return comum.Reserva{}, errors.New("este itinerário já foi utilizado")
	}
	if pendente.EmProcessamento {
		e.mutex.Unlock()
		return comum.Reserva{}, errors.New("este itinerário já está sendo confirmado")
	}
	pendente.EmProcessamento = true
	itinerario := pendente.Itinerario
	passageiroLogin, passageiroNome := u.Login, u.Nome
	pares := append([]EmpresaRemota(nil), e.pares...)
	cliente := e.clienteREST
	e.mutex.Unlock()

	liberarProcessamento := func() {
		e.mutex.Lock()
		if atual, ok := e.itinerarios[itinerarioID]; ok {
			atual.EmProcessamento = false
		}
		e.mutex.Unlock()
	}

	participantes, err := e.montarParticipantes(itinerario, pares)
	if err != nil {
		liberarProcessamento()
		return comum.Reserva{}, err
	}
	transacaoID := fmt.Sprintf("tx-%s-%s", e.empresaID, novoIDRequisicaoHTTP())
	preparados := make([]participanteReserva, 0, len(participantes))
	tentados := make([]participanteReserva, 0, len(participantes))

	for _, participante := range participantes {
		tentados = append(tentados, participante)
		dados := comum.PrepararReservaREST{
			TransacaoID: transacaoID, CoordenadorID: e.empresaID,
			DuracaoBloqueioMS: duracaoBloqueioPadrao.Milliseconds(),
			Trechos:           participante.trechos,
		}
		if participante.local {
			_, err = e.PrepararTransacao(dados)
		} else {
			err = cliente.PrepararReserva(context.Background(), participante.par, dados)
		}
		if err != nil {
			e.abortarParticipantes(transacaoID, tentados, cliente)
			liberarProcessamento()
			return comum.Reserva{}, fmt.Errorf("reserva recusada por %s: %w", participante.empresaID, err)
		}
		preparados = append(preparados, participante)
	}

	for _, participante := range preparados {
		if participante.local {
			_, err = e.ConfirmarTransacao(transacaoID)
		} else {
			err = cliente.ConfirmarReserva(context.Background(), participante.par, transacaoID)
		}
		if err != nil {
			e.abortarParticipantes(transacaoID, preparados, cliente)
			liberarProcessamento()
			return comum.Reserva{}, fmt.Errorf("não foi possível confirmar todos os trechos: %w", err)
		}
	}

	e.mutex.Lock()
	e.proximaReserva++
	reserva := &comum.Reserva{
		ID: fmt.Sprintf("RES-%03d", e.proximaReserva), TransacaoID: transacaoID,
		Passageiro: passageiroLogin, NomePassageiro: passageiroNome,
		Status: comum.StatusReservaConfirmada, CriadaEm: time.Now(), Itinerario: itinerario,
	}
	if reserva.Itinerario.Vagas > 0 {
		reserva.Itinerario.Vagas--
	}
	pendente = e.itinerarios[itinerarioID]
	pendente.Usado = true
	pendente.EmProcessamento = false
	e.reservas[reserva.ID] = reserva
	if err := e.salvarEstadoSemLock(); err != nil {
		delete(e.reservas, reserva.ID)
		e.proximaReserva--
		pendente.Usado = false
		e.mutex.Unlock()
		e.abortarParticipantes(transacaoID, preparados, cliente)
		return comum.Reserva{}, errors.New("não foi possível salvar a reserva distribuída")
	}
	e.mutex.Unlock()

	registrar("RESERVA_DISTRIBUIDA_CONFIRMADA usuario=%s reserva=%s transacao=%s empresas=%d",
		passageiroLogin, reserva.ID, transacaoID, len(participantes))
	return *reserva, nil
}

func (e *Estado) montarParticipantes(itinerario comum.Itinerario, pares []EmpresaRemota) ([]participanteReserva, error) {
	porEmpresa := make(map[string][]comum.TrechoTransacaoREST)
	for _, trecho := range itinerario.Trechos {
		empresaID := trecho.EmpresaID
		if empresaID == "" {
			empresaID = e.empresaID
		}
		porEmpresa[empresaID] = append(porEmpresa[empresaID], comum.TrechoTransacaoREST{
			CaronaID: trecho.CaronaID, Indice: trecho.Indice,
		})
	}
	paresPorID := make(map[string]EmpresaRemota, len(pares))
	for _, par := range pares {
		paresPorID[par.ID] = par
	}
	ids := make([]string, 0, len(porEmpresa))
	for id := range porEmpresa {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	participantes := make([]participanteReserva, 0, len(ids))
	for _, id := range ids {
		participante := participanteReserva{empresaID: id, trechos: porEmpresa[id], local: id == e.empresaID}
		if !participante.local {
			par, ok := paresPorID[id]
			if !ok {
				return nil, fmt.Errorf("empresa %s não está configurada como par", id)
			}
			participante.par = par
		}
		participantes = append(participantes, participante)
	}
	return participantes, nil
}

func (e *Estado) abortarParticipantes(transacaoID string, participantes []participanteReserva, cliente *ClienteRESTConsorcio) {
	for _, participante := range participantes {
		var err error
		if participante.local {
			_, err = e.AbortarTransacao(transacaoID)
		} else {
			err = cliente.AbortarReserva(context.Background(), participante.par, transacaoID)
		}
		if err != nil {
			registrarTecnico("TRANSACAO_ABORTO_FALHOU transacao=%s empresa=%s erro=%q", transacaoID, participante.empresaID, err.Error())
		}
	}
}

func (e *Estado) reservaUsaTransacaoDistribuida(token, reservaID string) (bool, error) {
	e.mutex.Lock()
	defer e.mutex.Unlock()
	u, err := e.usuarioDaSessaoSemLock(token)
	if err != nil {
		return false, err
	}
	reserva, existe := e.reservas[reservaID]
	if !existe || reserva.Passageiro != u.Login {
		return false, errors.New("reserva não encontrada")
	}
	return reserva.TransacaoID != "", nil
}

func (e *Estado) cancelarReservaDistribuida(token, reservaID string) error {
	e.mutex.Lock()
	u, err := e.usuarioDaSessaoSemLock(token)
	if err != nil {
		e.mutex.Unlock()
		return err
	}
	reserva, existe := e.reservas[reservaID]
	if !existe || reserva.Passageiro != u.Login {
		e.mutex.Unlock()
		return errors.New("reserva não encontrada")
	}
	if reserva.Status != comum.StatusReservaConfirmada {
		e.mutex.Unlock()
		return errors.New("a reserva já está cancelada")
	}
	transacaoID := reserva.TransacaoID
	itinerario := reserva.Itinerario
	pares := append([]EmpresaRemota(nil), e.pares...)
	cliente := e.clienteREST
	e.mutex.Unlock()

	participantes, err := e.montarParticipantes(itinerario, pares)
	if err != nil {
		return err
	}
	erros := make([]error, 0)
	for _, participante := range participantes {
		if participante.local {
			_, err = e.AbortarTransacao(transacaoID)
		} else {
			err = cliente.AbortarReserva(context.Background(), participante.par, transacaoID)
		}
		if err != nil {
			erros = append(erros, fmt.Errorf("%s: %w", participante.empresaID, err))
		}
	}
	if len(erros) > 0 {
		return fmt.Errorf("cancelamento pendente em %d empresa(s): %v", len(erros), erros)
	}

	e.mutex.Lock()
	defer e.mutex.Unlock()
	reserva = e.reservas[reservaID]
	reserva.Status = comum.StatusReservaCancelada
	if err := e.salvarEstadoSemLock(); err != nil {
		reserva.Status = comum.StatusReservaConfirmada
		return errors.New("não foi possível salvar o cancelamento")
	}
	registrar("RESERVA_DISTRIBUIDA_CANCELADA usuario=%s reserva=%s transacao=%s", u.Login, reservaID, transacaoID)
	return nil
}
