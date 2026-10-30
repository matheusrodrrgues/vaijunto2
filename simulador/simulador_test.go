package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestConfiguracaoRelacionaTCPESTPorEmpresa(t *testing.T) {
	config := configuracao{
		empresasTCP:  "empresa-b=127.0.0.1:9002,empresa-a=127.0.0.1:9001",
		empresasREST: "empresa-a=http://127.0.0.1:8081,empresa-b=http://127.0.0.1:8082",
		usuario:      "cliente", senha: "1234", origem: "A", destino: "B", data: "2026-10-20",
		requisicoes: 1, concorrencia: 1, capacidadeEsperada: 1,
	}
	alvos, err := config.alvos()
	if err != nil {
		t.Fatal(err)
	}
	if len(alvos) != 2 || alvos[0].ID != "empresa-a" || alvos[0].TCP != "127.0.0.1:9001" {
		t.Fatalf("alvos inesperados: %#v", alvos)
	}
}

func TestRelatorioCalculaLatenciaEVazao(t *testing.T) {
	coletor := &coletor{}
	coletor.adicionar(resultadoOperacao{Categoria: "reserva_confirmada", Duracao: 10 * time.Millisecond})
	coletor.adicionar(resultadoOperacao{Categoria: "reserva_recusada", Duracao: 30 * time.Millisecond, Parcial: true})
	inicio := time.Now()
	relatorio := montarRelatorio(configuracao{origem: "A", destino: "B", data: "2026-10-20", concorrencia: 2}, nil, inicio, inicio.Add(time.Second), coletor)
	if relatorio.Metricas.Total != 2 || relatorio.Metricas.LatenciaMediaMS != 20 ||
		relatorio.Metricas.OperacoesPorSegundo != 2 || relatorio.Metricas.BuscasParciais != 1 {
		t.Fatalf("métricas inesperadas: %#v", relatorio.Metricas)
	}
}

func TestRelatoriosJSONECSVSaemNoDisco(t *testing.T) {
	diretorio := t.TempDir()
	config := configuracao{
		saidaJSON: filepath.Join(diretorio, "resultado.json"),
		saidaCSV:  filepath.Join(diretorio, "resultado.csv"),
	}
	relatorio := Relatorio{
		Inicio: time.Now(), Fim: time.Now(), Metricas: MetricasOperacoes{PorCategoria: map[string]int{}},
		Inconsistencias: []string{}, Aprovado: true,
	}
	if err := salvarRelatorios(config, relatorio); err != nil {
		t.Fatal(err)
	}
	for _, caminho := range []string{config.saidaJSON, config.saidaCSV} {
		conteudo, err := os.ReadFile(caminho)
		if err != nil || len(conteudo) == 0 {
			t.Fatalf("relatório não foi gravado em %s: %v", caminho, err)
		}
	}
}
