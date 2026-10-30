package main

import (
	"strings"

	"vaijunto/comum"
)

// normalizarCidade tolera maiúsculas e espaços nas pontas, mas não traduz
// apelidos: FSA e Feira de Santana ainda são cidades diferentes para a busca.
func normalizarCidade(cidade string) string {
	return strings.ToLower(strings.TrimSpace(cidade))
}

// copiarCarona separa os slices da resposta dos dados guardados no servidor.
// Copiar apenas a struct manteria as listas apontando para a mesma memória.
func copiarCarona(c *comum.Carona) comum.Carona {
	copia := *c
	copia.Rota = copiarStrings(c.Rota)
	copia.PrecosCentavos = copiarInts(c.PrecosCentavos)
	copia.DuracoesMin = copiarInts(c.DuracoesMin)
	copia.DisponiveisPorTrecho = copiarInts(c.DisponiveisPorTrecho)
	return copia
}

func copiarStrings(v []string) []string { return append([]string(nil), v...) }
func copiarInts(v []int) []int          { return append([]int(nil), v...) }
func copiarArestas(v []aresta) []aresta { return append([]aresta(nil), v...) }

func copiarVisitadas(v map[string]bool) map[string]bool {
	copia := make(map[string]bool, len(v)+1)
	for chave, valor := range v {
		copia[chave] = valor
	}
	return copia
}
