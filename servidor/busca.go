package main

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"vaijunto/comum"
)

// Cada aresta representa um trecho que pode entrar na viagem. A empresa faz
// parte da identidade porque servidores distintos podem gerar o mesmo ID local.
type aresta struct {
	empresaID     string
	caronaID      string
	indice        int
	motorista     string
	data          string
	origem        string
	destino       string
	saida         time.Time
	chegada       time.Time
	precoCentavos int
	vagas         int
}

// BuscarItinerarios preserva a interface usada pelas operações do Problema 1.
func (e *Estado) BuscarItinerarios(token string, dados comum.BuscarItinerariosDados) ([]comum.Itinerario, error) {
	resultado, err := e.BuscarItinerariosDetalhado(token, dados)
	return resultado.Itinerarios, err
}

// BuscarItinerariosDetalhado reúne o catálogo local e os catálogos REST dos
// pares. Falha de uma empresa produz uma busca parcial, sem derrubar as demais.
func (e *Estado) BuscarItinerariosDetalhado(token string, dados comum.BuscarItinerariosDados) (comum.ResultadoBuscaItinerarios, error) {
	resultado := comum.ResultadoBuscaItinerarios{Itinerarios: make([]comum.Itinerario, 0)}
	if strings.TrimSpace(dados.Origem) == "" || strings.TrimSpace(dados.Destino) == "" {
		return resultado, errors.New("informe a origem e o destino")
	}
	if normalizarCidade(dados.Origem) == normalizarCidade(dados.Destino) {
		return resultado, errors.New("a origem e o destino precisam ser diferentes")
	}
	if dados.Data != "" {
		if _, err := time.ParseInLocation("2006-01-02", dados.Data, time.Local); err != nil {
			return resultado, errors.New("a data deve estar no formato AAAA-MM-DD")
		}
	}

	// A trava protege somente o retrato local. Nenhuma chamada de rede é feita
	// enquanto o estado está travado, portanto um par lento não bloqueia reservas.
	e.mutex.Lock()
	u, err := e.usuarioDaSessaoSemLock(token)
	if err != nil {
		e.mutex.Unlock()
		return resultado, err
	}
	if u.Perfil != comum.PerfilPassageiro {
		e.mutex.Unlock()
		return resultado, errors.New("operação permitida somente para passageiro")
	}
	passageiro := u.Login
	arestas := e.montarArestasSemLock(dados.Data)
	pares := append([]EmpresaRemota(nil), e.pares...)
	clienteREST := e.clienteREST
	e.mutex.Unlock()

	if clienteREST != nil && len(pares) > 0 {
		trechosRemotos, indisponiveis := clienteREST.ConsultarTrechosPares(context.Background(), pares, dados.Data)
		for _, trecho := range trechosRemotos {
			arestas = append(arestas, arestaDoTrechoREST(trecho))
		}
		resultado.EmpresasIndisponiveis = indisponiveis
		resultado.BuscaParcial = len(indisponiveis) > 0
	}

	for _, data := range datasDasArestas(arestas, dados.Data) {
		dataInicial, _ := time.ParseInLocation("2006-01-02", data, time.Local)
		encontrados := make([]comum.Itinerario, 0)
		visitadas := map[string]bool{normalizarCidade(dados.Origem): true}
		e.buscarCaminhos(arestas, dados.Origem, dados.Destino, dataInicial, data, nil, visitadas, &encontrados)
		for i := range encontrados {
			encontrados[i].Data = data
		}
		resultado.Itinerarios = append(resultado.Itinerarios, encontrados...)
	}

	sort.Slice(resultado.Itinerarios, func(i, j int) bool {
		saidaI := resultado.Itinerarios[i].Trechos[0].Saida
		saidaJ := resultado.Itinerarios[j].Trechos[0].Saida
		if !saidaI.Equal(saidaJ) {
			return saidaI.Before(saidaJ)
		}
		if resultado.Itinerarios[i].PrecoCentavos == resultado.Itinerarios[j].PrecoCentavos {
			return resultado.Itinerarios[i].Conexoes < resultado.Itinerarios[j].Conexoes
		}
		return resultado.Itinerarios[i].PrecoCentavos < resultado.Itinerarios[j].PrecoCentavos
	})
	if len(resultado.Itinerarios) > 20 {
		resultado.Itinerarios = resultado.Itinerarios[:20]
	}

	e.mutex.Lock()
	for i := range resultado.Itinerarios {
		e.proximoItinerario++
		resultado.Itinerarios[i].ID = fmt.Sprintf("ITI-%03d", e.proximoItinerario)
		resultado.Itinerarios[i].Origem = dados.Origem
		resultado.Itinerarios[i].Destino = dados.Destino
		e.itinerarios[resultado.Itinerarios[i].ID] = &itinerarioPendente{
			Passageiro: passageiro,
			Itinerario: resultado.Itinerarios[i],
		}
	}
	e.mutex.Unlock()

	registrar("BUSCA usuario=%s origem=%q destino=%q resultados=%d parcial=%t indisponiveis=%v",
		passageiro, dados.Origem, dados.Destino, len(resultado.Itinerarios), resultado.BuscaParcial, resultado.EmpresasIndisponiveis)
	return resultado, nil
}

func datasDasArestas(arestas []aresta, dataInformada string) []string {
	if dataInformada != "" {
		return []string{dataInformada}
	}
	vistas := make(map[string]bool)
	for _, a := range arestas {
		vistas[a.data] = true
	}
	datas := make([]string, 0, len(vistas))
	for data := range vistas {
		datas = append(datas, data)
	}
	sort.Strings(datas)
	return datas
}

func (e *Estado) montarArestasSemLock(data string) []aresta {
	lista := make([]aresta, 0)
	for _, carona := range e.caronas {
		if carona.Cancelada || (data != "" && carona.Data != data) {
			continue
		}
		momento, err := time.ParseInLocation("2006-01-02 15:04", carona.Data+" "+carona.Horario, time.Local)
		if err != nil {
			continue
		}
		for i := 0; i < len(carona.Rota)-1; i++ {
			chegada := momento.Add(time.Duration(carona.DuracoesMin[i]) * time.Minute)
			if carona.DisponiveisPorTrecho[i] > 0 {
				lista = append(lista, aresta{
					empresaID: e.empresaID, caronaID: carona.ID, indice: i, motorista: carona.Motorista,
					data: carona.Data, origem: carona.Rota[i], destino: carona.Rota[i+1],
					saida: momento, chegada: chegada, precoCentavos: carona.PrecosCentavos[i],
					vagas: carona.DisponiveisPorTrecho[i],
				})
			}
			momento = chegada
		}
	}
	return lista
}

// ListarTrechosDisponiveisREST expõe apenas o catálogo necessário para montar
// rotas. Usuários, sessões e reservas permanecem privados à empresa.
func (e *Estado) ListarTrechosDisponiveisREST(data string) []comum.TrechoDisponivelREST {
	e.mutex.Lock()
	defer e.mutex.Unlock()
	arestas := e.montarArestasSemLock(data)
	lista := make([]comum.TrechoDisponivelREST, 0, len(arestas))
	for _, a := range arestas {
		lista = append(lista, comum.TrechoDisponivelREST{
			EmpresaID: a.empresaID, CaronaID: a.caronaID, Indice: a.indice, Data: a.data,
			Origem: a.origem, Destino: a.destino, Saida: a.saida, Chegada: a.chegada,
			PrecoCentavos: a.precoCentavos, Vagas: a.vagas,
		})
	}
	return lista
}

func arestaDoTrechoREST(t comum.TrechoDisponivelREST) aresta {
	return aresta{
		empresaID: t.EmpresaID, caronaID: t.CaronaID, indice: t.Indice, data: t.Data,
		origem: t.Origem, destino: t.Destino, saida: t.Saida, chegada: t.Chegada,
		precoCentavos: t.PrecoCentavos, vagas: t.Vagas,
	}
}

func (e *Estado) buscarCaminhos(arestas []aresta, cidade, destino string, horarioMinimo time.Time, data string, caminho []aresta, visitadas map[string]bool, resultados *[]comum.Itinerario) {
	if len(*resultados) >= 60 || len(caminho) >= 8 {
		return
	}
	for _, atual := range arestas {
		if atual.data != data || normalizarCidade(atual.origem) != normalizarCidade(cidade) || visitadas[normalizarCidade(atual.destino)] {
			continue
		}
		minimo := horarioMinimo
		if len(caminho) > 0 {
			anterior := caminho[len(caminho)-1]
			if identidadeCarona(anterior) != identidadeCarona(atual) {
				minimo = minimo.Add(10 * time.Minute)
			}
		}
		if atual.saida.Before(minimo) {
			continue
		}
		novoCaminho := append(copiarArestas(caminho), atual)
		if normalizarCidade(atual.destino) == normalizarCidade(destino) {
			*resultados = append(*resultados, transformarItinerario(novoCaminho))
			continue
		}
		novasVisitadas := copiarVisitadas(visitadas)
		novasVisitadas[normalizarCidade(atual.destino)] = true
		e.buscarCaminhos(arestas, atual.destino, destino, atual.chegada, data, novoCaminho, novasVisitadas, resultados)
	}
}

func identidadeCarona(a aresta) string { return a.empresaID + "|" + a.caronaID }

func transformarItinerario(caminho []aresta) comum.Itinerario {
	itinerario := comum.Itinerario{Vagas: caminho[0].vagas, Trechos: make([]comum.TrechoItinerario, 0, len(caminho))}
	caronasUsadas := make(map[string]bool)
	for _, a := range caminho {
		itinerario.PrecoCentavos += a.precoCentavos
		if a.vagas < itinerario.Vagas {
			itinerario.Vagas = a.vagas
		}
		caronasUsadas[identidadeCarona(a)] = true
		itinerario.Trechos = append(itinerario.Trechos, comum.TrechoItinerario{
			EmpresaID: a.empresaID, CaronaID: a.caronaID, Indice: a.indice, Motorista: a.motorista,
			Origem: a.origem, Destino: a.destino, Saida: a.saida,
			Chegada: a.chegada, PrecoCentavos: a.precoCentavos,
		})
	}
	itinerario.Conexoes = len(caronasUsadas) - 1
	return itinerario
}
