package main

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"time"

	"vaijunto/comum"
)

type MetricasOperacoes struct {
	Total               int            `json:"total"`
	PorCategoria        map[string]int `json:"por_categoria"`
	BuscasParciais      int            `json:"buscas_parciais"`
	LatenciaMinimaMS    float64        `json:"latencia_minima_ms"`
	LatenciaMediaMS     float64        `json:"latencia_media_ms"`
	LatenciaP95MS       float64        `json:"latencia_p95_ms"`
	LatenciaMaximaMS    float64        `json:"latencia_maxima_ms"`
	OperacoesPorSegundo float64        `json:"operacoes_por_segundo"`
}

type Relatorio struct {
	Inicio                    time.Time                    `json:"inicio"`
	Fim                       time.Time                    `json:"fim"`
	DuracaoMS                 int64                        `json:"duracao_ms"`
	Origem                    string                       `json:"origem"`
	Destino                   string                       `json:"destino"`
	Data                      string                       `json:"data"`
	Concorrencia              int                          `json:"concorrencia"`
	Metricas                  MetricasOperacoes            `json:"metricas"`
	FalhasInjetadas           []string                     `json:"falhas_injetadas,omitempty"`
	ErrosInjecao              []string                     `json:"erros_injecao,omitempty"`
	Diagnosticos              []comum.ResumoTransacoesREST `json:"diagnosticos"`
	Inconsistencias           []string                     `json:"inconsistencias"`
	Erros                     []string                     `json:"erros,omitempty"`
	Aprovado                  bool                         `json:"aprovado"`
	ReservasConfirmadasFinais int                          `json:"reservas_confirmadas_finais"`
	reservasConfirmadas       []comum.Reserva
}

func montarRelatorio(config configuracao, _ []empresaAlvo, inicio, fim time.Time, coletor *coletor) Relatorio {
	coletor.mutex.Lock()
	resultados := append([]resultadoOperacao(nil), coletor.resultados...)
	falhas := append([]string(nil), coletor.falhas...)
	errosFalha := append([]string(nil), coletor.errosFalha...)
	coletor.mutex.Unlock()
	relatorio := Relatorio{
		Inicio: inicio, Fim: fim, DuracaoMS: fim.Sub(inicio).Milliseconds(),
		Origem: config.origem, Destino: config.destino, Data: config.data,
		Concorrencia: config.concorrencia, FalhasInjetadas: falhas, ErrosInjecao: errosFalha,
		Inconsistencias: make([]string, 0), Diagnosticos: make([]comum.ResumoTransacoesREST, 0),
		Metricas: MetricasOperacoes{PorCategoria: make(map[string]int)},
	}
	latencias := make([]float64, 0, len(resultados))
	var soma float64
	for _, resultado := range resultados {
		relatorio.Metricas.Total++
		relatorio.Metricas.PorCategoria[resultado.Categoria]++
		if resultado.Parcial {
			relatorio.Metricas.BuscasParciais++
		}
		latencia := float64(resultado.Duracao.Microseconds()) / 1000
		latencias = append(latencias, latencia)
		soma += latencia
		if resultado.Reserva != nil {
			relatorio.reservasConfirmadas = append(relatorio.reservasConfirmadas, *resultado.Reserva)
		}
		if resultado.Erro != "" && len(relatorio.Erros) < 50 {
			relatorio.Erros = append(relatorio.Erros,
				fmt.Sprintf("%s/%s: %s", resultado.Empresa, resultado.Categoria, resultado.Erro))
		}
	}
	if len(latencias) > 0 {
		sort.Float64s(latencias)
		relatorio.Metricas.LatenciaMinimaMS = latencias[0]
		relatorio.Metricas.LatenciaMaximaMS = latencias[len(latencias)-1]
		relatorio.Metricas.LatenciaMediaMS = soma / float64(len(latencias))
		indiceP95 := int(float64(len(latencias)-1) * 0.95)
		relatorio.Metricas.LatenciaP95MS = latencias[indiceP95]
	}
	segundos := fim.Sub(inicio).Seconds()
	if segundos > 0 {
		relatorio.Metricas.OperacoesPorSegundo = float64(relatorio.Metricas.Total) / segundos
	}
	return relatorio
}

func salvarRelatorios(config configuracao, relatorio Relatorio) error {
	if err := salvarJSON(config.saidaJSON, relatorio); err != nil {
		return err
	}
	return salvarCSV(config.saidaCSV, relatorio)
}

func salvarJSON(caminho string, relatorio Relatorio) error {
	if err := os.MkdirAll(filepath.Dir(caminho), 0755); err != nil {
		return err
	}
	conteudo, err := json.MarshalIndent(relatorio, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(caminho, append(conteudo, '\n'), 0644)
}

func salvarCSV(caminho string, relatorio Relatorio) error {
	if err := os.MkdirAll(filepath.Dir(caminho), 0755); err != nil {
		return err
	}
	arquivo, err := os.Create(caminho)
	if err != nil {
		return err
	}
	defer arquivo.Close()
	escritor := csv.NewWriter(arquivo)
	defer escritor.Flush()
	cabecalho := []string{"inicio", "fim", "duracao_ms", "total", "confirmadas_imediatas", "reservas_finais", "recusadas", "erros_conexao", "latencia_media_ms", "latencia_p95_ms", "ops_segundo", "inconsistencias", "aprovado"}
	linha := []string{
		relatorio.Inicio.Format(time.RFC3339Nano), relatorio.Fim.Format(time.RFC3339Nano),
		strconv.FormatInt(relatorio.DuracaoMS, 10), strconv.Itoa(relatorio.Metricas.Total),
		strconv.Itoa(relatorio.Metricas.PorCategoria["reserva_confirmada"]),
		strconv.Itoa(relatorio.ReservasConfirmadasFinais),
		strconv.Itoa(relatorio.Metricas.PorCategoria["reserva_recusada"]),
		strconv.Itoa(relatorio.Metricas.PorCategoria["erro_conexao"]),
		fmt.Sprintf("%.3f", relatorio.Metricas.LatenciaMediaMS),
		fmt.Sprintf("%.3f", relatorio.Metricas.LatenciaP95MS),
		fmt.Sprintf("%.3f", relatorio.Metricas.OperacoesPorSegundo),
		strconv.Itoa(len(relatorio.Inconsistencias)), strconv.FormatBool(relatorio.Aprovado),
	}
	if err := escritor.Write(cabecalho); err != nil {
		return err
	}
	return escritor.Write(linha)
}

func imprimirResumo(relatorio Relatorio) {
	fmt.Println()
	fmt.Println("====================================")
	fmt.Println("       RELATÓRIO DA SIMULAÇÃO")
	fmt.Println("====================================")
	fmt.Printf("Operações: %d | Concorrência: %d\n", relatorio.Metricas.Total, relatorio.Concorrencia)
	fmt.Printf("Confirmadas imediatamente: %d | Reservas finais: %d | Recusadas: %d | Buscas parciais: %d\n",
		relatorio.Metricas.PorCategoria["reserva_confirmada"],
		relatorio.ReservasConfirmadasFinais,
		relatorio.Metricas.PorCategoria["reserva_recusada"],
		relatorio.Metricas.BuscasParciais)
	fmt.Printf("Latência média: %.2f ms | p95: %.2f ms | máxima: %.2f ms\n",
		relatorio.Metricas.LatenciaMediaMS, relatorio.Metricas.LatenciaP95MS,
		relatorio.Metricas.LatenciaMaximaMS)
	fmt.Printf("Vazão: %.2f operações/s\n", relatorio.Metricas.OperacoesPorSegundo)
	fmt.Printf("Inconsistências: %d | APROVADO: %t\n", len(relatorio.Inconsistencias), relatorio.Aprovado)
	for _, inconsistencia := range relatorio.Inconsistencias {
		fmt.Println(" -", inconsistencia)
	}
}
