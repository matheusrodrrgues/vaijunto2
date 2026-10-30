package main

import (
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"vaijunto/comum"
)

// Cada aresta representa um trecho que pode entrar na viagem.
// Não basta ligar duas cidades: o horário e a vaga também precisam servir.
type aresta struct {
	caronaID      string
	indice        int
	motorista     string
	origem        string
	destino       string
	saida         time.Time
	chegada       time.Time
	precoCentavos int
	vagas         int
}

// BuscarItinerarios monta opções, mas ainda não segura nenhum assento.
// Entre buscar e confirmar, outro passageiro pode reservar a última vaga.
func (e *Estado) BuscarItinerarios(token string, dados comum.BuscarItinerariosDados) ([]comum.Itinerario, error) {
	e.mutex.Lock()
	defer e.mutex.Unlock()

	u, err := e.usuarioDaSessaoSemLock(token)
	if err != nil {
		return nil, err
	}
	if u.Perfil != comum.PerfilPassageiro {
		return nil, errors.New("operação permitida somente para passageiro")
	}
	if strings.TrimSpace(dados.Origem) == "" || strings.TrimSpace(dados.Destino) == "" {
		return nil, errors.New("informe a origem e o destino")
	}
	if normalizarCidade(dados.Origem) == normalizarCidade(dados.Destino) {
		return nil, errors.New("a origem e o destino precisam ser diferentes")
	}
	resultados := make([]comum.Itinerario, 0)
	datas, err := e.datasDaBuscaSemLock(dados.Data)
	if err != nil {
		return nil, err
	}

	for _, data := range datas {
		dataInicial, _ := time.ParseInLocation("2006-01-02", data, time.Local)
		arestas := e.montarArestasSemLock(data)
		encontrados := make([]comum.Itinerario, 0)
		visitadas := map[string]bool{normalizarCidade(dados.Origem): true}
		e.buscarCaminhos(arestas, dados.Origem, dados.Destino, dataInicial, nil, visitadas, &encontrados)
		for i := range encontrados {
			encontrados[i].Data = data
		}
		resultados = append(resultados, encontrados...)
	}

	sort.Slice(resultados, func(i, j int) bool {
		saidaI := resultados[i].Trechos[0].Saida
		saidaJ := resultados[j].Trechos[0].Saida
		if !saidaI.Equal(saidaJ) {
			return saidaI.Before(saidaJ)
		}
		if resultados[i].PrecoCentavos == resultados[j].PrecoCentavos {
			return resultados[i].Conexoes < resultados[j].Conexoes
		}
		return resultados[i].PrecoCentavos < resultados[j].PrecoCentavos
	})
	if len(resultados) > 20 {
		resultados = resultados[:20]
	}

	// A opção fica ligada a quem fez a busca. O cliente manda apenas o código
	// na confirmação; não escolhe por conta própria os preços e trechos.
	for i := range resultados {
		e.proximoItinerario++
		resultados[i].ID = fmt.Sprintf("ITI-%03d", e.proximoItinerario)
		resultados[i].Origem = dados.Origem
		resultados[i].Destino = dados.Destino
		e.itinerarios[resultados[i].ID] = &itinerarioPendente{
			Passageiro: u.Login,
			Itinerario: resultados[i],
		}
	}
	registrar("BUSCA usuario=%s origem=%q destino=%q resultados=%d", u.Login, dados.Origem, dados.Destino, len(resultados))
	return resultados, nil
}

// Sem uma data informada, procura nas datas das caronas com vagas.
// É assim que a tela consegue oferecer as datas depois de receber as cidades.
func (e *Estado) datasDaBuscaSemLock(dataInformada string) ([]string, error) {
	if dataInformada != "" {
		if _, err := time.ParseInLocation("2006-01-02", dataInformada, time.Local); err != nil {
			return nil, errors.New("a data deve estar no formato AAAA-MM-DD")
		}
		return []string{dataInformada}, nil
	}

	datasEncontradas := make(map[string]bool)
	for _, carona := range e.caronas {
		if carona.Cancelada {
			continue
		}
		for _, vagas := range carona.DisponiveisPorTrecho {
			if vagas > 0 {
				datasEncontradas[carona.Data] = true
				break
			}
		}
	}

	datas := make([]string, 0, len(datasEncontradas))
	for data := range datasEncontradas {
		datas = append(datas, data)
	}
	sort.Strings(datas)
	return datas, nil
}

func (e *Estado) montarArestasSemLock(data string) []aresta {
	lista := make([]aresta, 0)
	for _, carona := range e.caronas {
		if carona.Cancelada || carona.Data != data {
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
					caronaID: carona.ID, indice: i, motorista: carona.Motorista,
					origem: carona.Rota[i], destino: carona.Rota[i+1],
					saida: momento, chegada: chegada, precoCentavos: carona.PrecosCentavos[i],
					vagas: carona.DisponiveisPorTrecho[i],
				})
			}
			// A próxima cidade vem depois da duração deste trecho,
			// mesmo quando ele está lotado e não entrou na lista de opções.
			momento = chegada
		}
	}
	return lista
}

func (e *Estado) buscarCaminhos(arestas []aresta, cidade, destino string, horarioMinimo time.Time, caminho []aresta, visitadas map[string]bool, resultados *[]comum.Itinerario) {
	// Esses limites seguram o custo da busca quando existem muitas combinações.
	if len(*resultados) >= 60 || len(caminho) >= 8 {
		return
	}

	for _, atual := range arestas {
		if normalizarCidade(atual.origem) != normalizarCidade(cidade) {
			continue
		}
		if visitadas[normalizarCidade(atual.destino)] {
			continue
		}

		minimo := horarioMinimo
		if len(caminho) > 0 {
			anterior := caminho[len(caminho)-1]
			// Trocou de carro? Precisa dar tempo de fazer a conexão.
			if anterior.caronaID != atual.caronaID {
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

		// Cada tentativa leva sua própria lista de cidades visitadas.
		// Uma rota descartada não pode bloquear as outras possibilidades.
		novasVisitadas := copiarVisitadas(visitadas)
		novasVisitadas[normalizarCidade(atual.destino)] = true
		e.buscarCaminhos(arestas, atual.destino, destino, atual.chegada, novoCaminho, novasVisitadas, resultados)
	}
}

func transformarItinerario(caminho []aresta) comum.Itinerario {
	itinerario := comum.Itinerario{
		Vagas:   caminho[0].vagas,
		Trechos: make([]comum.TrechoItinerario, 0, len(caminho)),
	}
	caronasUsadas := make(map[string]bool)
	for _, a := range caminho {
		itinerario.PrecoCentavos += a.precoCentavos
		// A viagem inteira só tem tantas vagas quanto seu trecho mais apertado.
		if a.vagas < itinerario.Vagas {
			itinerario.Vagas = a.vagas
		}
		caronasUsadas[a.caronaID] = true
		itinerario.Trechos = append(itinerario.Trechos, comum.TrechoItinerario{
			CaronaID: a.caronaID, Indice: a.indice, Motorista: a.motorista,
			Origem: a.origem, Destino: a.destino, Saida: a.saida,
			Chegada: a.chegada, PrecoCentavos: a.precoCentavos,
		})
	}
	itinerario.Conexoes = len(caronasUsadas) - 1
	return itinerario
}
