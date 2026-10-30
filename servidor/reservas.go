package main

import (
	"errors"
	"fmt"
	"sort"
	"time"

	"vaijunto/comum"
)

// ConfirmarReserva segura o mutex até terminar a validação e o salvamento.
// Conferir vagas sem manter essa trava deixaria dois clientes comprarem a última.
func (e *Estado) ConfirmarReserva(token, itinerarioID string) (comum.Reserva, error) {
	e.mutex.Lock()
	defer e.mutex.Unlock()

	u, err := e.usuarioDaSessaoSemLock(token)
	if err != nil {
		return comum.Reserva{}, err
	}
	pendente, existe := e.itinerarios[itinerarioID]
	if !existe || pendente.Passageiro != u.Login {
		return comum.Reserva{}, errors.New("itinerário não encontrado")
	}
	if pendente.Usado {
		return comum.Reserva{}, errors.New("este itinerário já foi utilizado")
	}
	for _, trecho := range pendente.Itinerario.Trechos {
		if trecho.EmpresaID != "" && trecho.EmpresaID != e.empresaID {
			return comum.Reserva{}, errors.New("a confirmação entre empresas será implementada na etapa de reserva distribuída")
		}
	}

	// Primeiro confere tudo. Se faltar vaga no último trecho, não faz sentido
	// deixar o passageiro com os primeiros trechos reservados.
	for _, trecho := range pendente.Itinerario.Trechos {
		carona, existe := e.caronas[trecho.CaronaID]
		if !existe || carona.Cancelada || trecho.Indice < 0 || trecho.Indice >= len(carona.DisponiveisPorTrecho) {
			return comum.Reserva{}, errors.New("um dos trechos não está mais disponível")
		}
		if carona.DisponiveisPorTrecho[trecho.Indice] < 1 {
			return comum.Reserva{}, errors.New("um dos trechos ficou sem assento")
		}
	}

	// Agora sim: todos os trechos passaram na verificação e a trava continua ativa.
	for _, trecho := range pendente.Itinerario.Trechos {
		e.caronas[trecho.CaronaID].DisponiveisPorTrecho[trecho.Indice]--
	}
	if pendente.Itinerario.Vagas > 0 {
		pendente.Itinerario.Vagas--
	}

	e.proximaReserva++
	reserva := &comum.Reserva{
		ID:             fmt.Sprintf("RES-%03d", e.proximaReserva),
		Passageiro:     u.Login,
		NomePassageiro: u.Nome,
		Status:         comum.StatusReservaConfirmada,
		CriadaEm:       time.Now(),
		Itinerario:     pendente.Itinerario,
	}
	pendente.Usado = true
	e.reservas[reserva.ID] = reserva
	// Só informa sucesso depois de salvar. Se falhar, libera os assentos
	// e permite que esta opção seja tentada novamente.
	if err := e.salvarEstadoSemLock(); err != nil {
		delete(e.reservas, reserva.ID)
		e.proximaReserva--
		pendente.Usado = false
		e.devolverAssentosSemLock(reserva)
		return comum.Reserva{}, errors.New("não foi possível salvar a reserva")
	}
	registrar("RESERVA_CONFIRMADA usuario=%s reserva=%s itinerario=%s origem=%q destino=%q vagas_restantes=%d",
		u.Login, reserva.ID, itinerarioID, reserva.Itinerario.Origem,
		reserva.Itinerario.Destino, reserva.Itinerario.Vagas)
	return *reserva, nil
}

func (e *Estado) ListarReservas(token string) ([]comum.Reserva, error) {
	e.mutex.Lock()
	defer e.mutex.Unlock()

	u, err := e.usuarioDaSessaoSemLock(token)
	if err != nil {
		return nil, err
	}
	if u.Perfil != comum.PerfilPassageiro {
		return nil, errors.New("operação permitida somente para passageiro")
	}

	lista := make([]comum.Reserva, 0)
	for _, reserva := range e.reservas {
		if reserva.Passageiro == u.Login {
			lista = append(lista, *reserva)
		}
	}
	sort.Slice(lista, func(i, j int) bool { return lista[i].ID < lista[j].ID })
	return lista, nil
}

func (e *Estado) CancelarReserva(token, reservaID string) error {
	e.mutex.Lock()
	defer e.mutex.Unlock()

	u, err := e.usuarioDaSessaoSemLock(token)
	if err != nil {
		return err
	}
	reserva, existe := e.reservas[reservaID]
	if !existe || reserva.Passageiro != u.Login {
		return errors.New("reserva não encontrada")
	}
	if reserva.Status != comum.StatusReservaConfirmada {
		return errors.New("a reserva já está cancelada")
	}

	caronasAntes, reservasAntes := e.copiarDadosSemLock()
	e.devolverAssentosSemLock(reserva)
	reserva.Status = comum.StatusReservaCancelada
	if err := e.salvarEstadoSemLock(); err != nil {
		e.caronas = caronasAntes
		e.reservas = reservasAntes
		return errors.New("não foi possível salvar o cancelamento")
	}
	registrar("RESERVA_CANCELADA usuario=%s reserva=%s", u.Login, reservaID)
	return nil
}

// copiarDadosSemLock prepara a volta ao estado anterior se o cancelamento falhar.
// As vagas precisam de slices próprios; copiar só o ponteiro não preservaria nada.
// Os trechos das reservas não são alterados durante o cancelamento.
func (e *Estado) copiarDadosSemLock() (map[string]*comum.Carona, map[string]*comum.Reserva) {
	caronas := make(map[string]*comum.Carona, len(e.caronas))
	for id, carona := range e.caronas {
		copia := copiarCarona(carona)
		caronas[id] = &copia
	}
	reservas := make(map[string]*comum.Reserva, len(e.reservas))
	for id, reserva := range e.reservas {
		copia := *reserva
		reservas[id] = &copia
	}
	return caronas, reservas
}

// devolverAssentosSemLock é compartilhada pelos dois tipos de cancelamento.
// Quem chama precisa manter o mutex travado e decidir se a devolução é devida.
func (e *Estado) devolverAssentosSemLock(reserva *comum.Reserva) {
	for _, trecho := range reserva.Itinerario.Trechos {
		carona, existe := e.caronas[trecho.CaronaID]
		if !existe || trecho.Indice < 0 || trecho.Indice >= len(carona.DisponiveisPorTrecho) {
			continue
		}
		if carona.DisponiveisPorTrecho[trecho.Indice] < carona.Assentos {
			carona.DisponiveisPorTrecho[trecho.Indice]++
		}
	}
}
