package main

import (
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"vaijunto/comum"
)

func (e *Estado) PublicarCarona(token string, dados comum.PublicarCaronaDados) (comum.Carona, error) {
	e.mutex.Lock()
	defer e.mutex.Unlock()

	u, err := e.usuarioDaSessaoSemLock(token)
	if err != nil {
		return comum.Carona{}, err
	}
	if u.Perfil != comum.PerfilMotorista {
		return comum.Carona{}, errors.New("somente motoristas podem publicar caronas")
	}
	if err := validarCarona(dados); err != nil {
		return comum.Carona{}, err
	}

	e.proximaCarona++
	id := fmt.Sprintf("CAR-%03d", e.proximaCarona)
	// O mesmo assento pode ser usado por pessoas diferentes em trechos
	// que não se sobrepõem. Por isso a disponibilidade é separada por trecho.
	disponiveis := make([]int, len(dados.Rota)-1)
	for i := range disponiveis {
		disponiveis[i] = dados.Assentos
	}

	carona := &comum.Carona{
		ID:                   id,
		Motorista:            u.Login,
		Rota:                 copiarStrings(dados.Rota),
		Data:                 dados.Data,
		Horario:              dados.Horario,
		Assentos:             dados.Assentos,
		PrecosCentavos:       copiarInts(dados.PrecosCentavos),
		DuracoesMin:          copiarInts(dados.DuracoesMin),
		DisponiveisPorTrecho: disponiveis,
	}
	e.caronas[id] = carona
	if err := e.salvarEstadoSemLock(); err != nil {
		delete(e.caronas, id)
		e.proximaCarona--
		return comum.Carona{}, errors.New("não foi possível salvar a carona")
	}
	registrar("CARONA_PUBLICADA usuario=%s carona=%s rota=%q data=%s horario=%s assentos=%d",
		u.Login, id, strings.Join(carona.Rota, " -> "), carona.Data, carona.Horario, carona.Assentos)
	return copiarCarona(carona), nil
}

// validarCarona confere também o que chega por clientes fora dos nossos menus.
// A validação da tela ajuda quem digita; a do servidor protege os dados.
func validarCarona(dados comum.PublicarCaronaDados) error {
	if len(dados.Rota) < 2 {
		return errors.New("a rota precisa ter pelo menos duas cidades")
	}
	if dados.Assentos < 1 {
		return errors.New("a quantidade de assentos deve ser maior que zero")
	}
	if len(dados.PrecosCentavos) != len(dados.Rota)-1 {
		return errors.New("informe um preço para cada trecho")
	}
	if len(dados.DuracoesMin) != len(dados.Rota)-1 {
		return errors.New("informe a duração de cada trecho")
	}
	if _, err := time.Parse("2006-01-02", dados.Data); err != nil {
		return errors.New("a data deve estar no formato AAAA-MM-DD")
	}
	if _, err := time.Parse("15:04", dados.Horario); err != nil {
		return errors.New("o horário deve estar no formato HH:MM")
	}

	vistas := make(map[string]bool)
	for i, cidade := range dados.Rota {
		cidade = strings.TrimSpace(cidade)
		if cidade == "" {
			return errors.New("a rota possui uma cidade vazia")
		}
		normalizada := normalizarCidade(cidade)
		if vistas[normalizada] {
			return errors.New("a rota não pode repetir uma cidade")
		}
		vistas[normalizada] = true
		dados.Rota[i] = cidade
	}

	for i := range dados.PrecosCentavos {
		if dados.PrecosCentavos[i] < 0 {
			return errors.New("o preço não pode ser negativo")
		}
		if dados.DuracoesMin[i] < 1 {
			return errors.New("a duração deve ser maior que zero")
		}
	}
	return nil
}

func (e *Estado) ListarCaronas(token string) ([]comum.Carona, error) {
	e.mutex.Lock()
	defer e.mutex.Unlock()

	u, err := e.usuarioDaSessaoSemLock(token)
	if err != nil {
		return nil, err
	}
	if u.Perfil != comum.PerfilMotorista {
		return nil, errors.New("operação permitida somente para motorista")
	}

	lista := make([]comum.Carona, 0)
	for _, carona := range e.caronas {
		if carona.Motorista == u.Login {
			lista = append(lista, copiarCarona(carona))
		}
	}
	sort.Slice(lista, func(i, j int) bool { return lista[i].ID < lista[j].ID })
	return lista, nil
}

func (e *Estado) CancelarCarona(token, caronaID string) error {
	e.mutex.Lock()
	defer e.mutex.Unlock()

	u, err := e.usuarioDaSessaoSemLock(token)
	if err != nil {
		return err
	}
	carona, existe := e.caronas[caronaID]
	if !existe {
		return errors.New("carona não encontrada")
	}
	if carona.Motorista != u.Login {
		return errors.New("a carona pertence a outro motorista")
	}
	if carona.Cancelada {
		return errors.New("a carona já está cancelada")
	}

	// Se uma parte da viagem caiu, a reserva completa perde o sentido.
	// Guarda uma cópia para desfazer o cancelamento se não conseguir salvar.
	caronasAntes, reservasAntes := e.copiarDadosSemLock()
	carona.Cancelada = true
	for _, reserva := range e.reservas {
		if reserva.Status != comum.StatusReservaConfirmada || !usaCarona(reserva, caronaID) {
			continue
		}
		e.devolverAssentosSemLock(reserva)
		reserva.Status = comum.StatusCanceladaMotorista
	}
	if err := e.salvarEstadoSemLock(); err != nil {
		e.caronas = caronasAntes
		e.reservas = reservasAntes
		return errors.New("não foi possível salvar o cancelamento")
	}
	registrar("CARONA_CANCELADA usuario=%s carona=%s", u.Login, caronaID)
	return nil
}

// ListarPassageiros retorna reservas confirmadas que passam por esta carona.
// O itinerário retornado é completo e pode incluir carros de outros motoristas.
func (e *Estado) ListarPassageiros(token, caronaID string) ([]comum.Reserva, error) {
	e.mutex.Lock()
	defer e.mutex.Unlock()

	u, err := e.usuarioDaSessaoSemLock(token)
	if err != nil {
		return nil, err
	}
	carona, existe := e.caronas[caronaID]
	if !existe {
		return nil, errors.New("carona não encontrada")
	}
	if carona.Motorista != u.Login {
		return nil, errors.New("a carona pertence a outro motorista")
	}

	lista := make([]comum.Reserva, 0)
	for _, reserva := range e.reservas {
		if reserva.Status == comum.StatusReservaConfirmada && usaCarona(reserva, caronaID) {
			lista = append(lista, *reserva)
		}
	}
	sort.Slice(lista, func(i, j int) bool { return lista[i].ID < lista[j].ID })
	return lista, nil
}

func usaCarona(reserva *comum.Reserva, caronaID string) bool {
	for _, trecho := range reserva.Itinerario.Trechos {
		if trecho.CaronaID == caronaID {
			return true
		}
	}
	return false
}
