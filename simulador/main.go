package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"time"
)

func main() {
	config := configuracao{}
	flag.StringVar(&config.empresasTCP, "empresas", valorAmbienteSimulador("VAIJUNTO_SIM_EMPRESAS", "empresa-a=127.0.0.1:9001,empresa-b=127.0.0.1:9002,empresa-c=127.0.0.1:9003"), "IDs e endereços TCP")
	flag.StringVar(&config.empresasREST, "rest", valorAmbienteSimulador("VAIJUNTO_SIM_REST", "empresa-a=http://127.0.0.1:8081,empresa-b=http://127.0.0.1:8082,empresa-c=http://127.0.0.1:8083"), "IDs e URLs REST")
	flag.StringVar(&config.usuario, "usuario", "cliente", "usuário passageiro existente em todas as empresas")
	flag.StringVar(&config.senha, "senha", "1234", "senha do passageiro")
	flag.StringVar(&config.origem, "origem", "Salvador", "cidade de origem")
	flag.StringVar(&config.destino, "destino", "Montes Claros", "cidade de destino")
	flag.StringVar(&config.data, "data", "2026-11-05", "data da viagem no formato AAAA-MM-DD")
	flag.IntVar(&config.requisicoes, "requisicoes", 100, "quantidade total de passageiros simulados")
	flag.IntVar(&config.concorrencia, "concorrencia", 20, "quantidade máxima de operações simultâneas")
	flag.DurationVar(&config.duracao, "duracao", 0, "duração do teste; quando positiva substitui requisicoes")
	flag.IntVar(&config.capacidadeEsperada, "capacidade", 1, "capacidade esperada por trecho para detectar venda dupla")
	flag.DurationVar(&config.esperaRecuperacao, "espera-recuperacao", 17*time.Second, "tempo para recuperação e expiração de preparações")
	flag.StringVar(&config.servicoFalha, "falha-servico", "", "serviço Docker a interromper, por exemplo empresa-b")
	flag.DurationVar(&config.falhaApos, "falha-apos", 2*time.Second, "tempo até interromper o serviço")
	flag.DurationVar(&config.falhaDuracao, "falha-duracao", 3*time.Second, "tempo que o serviço permanece parado")
	flag.StringVar(&config.diretorioCompose, "compose-dir", ".", "diretório que contém compose.yaml")
	flag.StringVar(&config.saidaJSON, "json", "relatorios/simulacao.json", "arquivo de relatório JSON")
	flag.StringVar(&config.saidaCSV, "csv", "relatorios/simulacao.csv", "arquivo de relatório CSV")
	flag.Parse()

	if err := config.validar(); err != nil {
		fmt.Fprintln(os.Stderr, "Configuração inválida:", err)
		os.Exit(2)
	}
	ctx := context.Background()
	relatorio, err := executarSimulacao(ctx, config)
	if erroRelatorio := salvarRelatorios(config, relatorio); erroRelatorio != nil {
		fmt.Fprintln(os.Stderr, "Não foi possível salvar os relatórios:", erroRelatorio)
		os.Exit(1)
	}
	imprimirResumo(relatorio)
	if err != nil || len(relatorio.Inconsistencias) > 0 {
		if err != nil {
			fmt.Fprintln(os.Stderr, "Simulação terminou com erro:", err)
		}
		os.Exit(1)
	}
}

func valorAmbienteSimulador(chave, padrao string) string {
	if valor := os.Getenv(chave); valor != "" {
		return valor
	}
	return padrao
}
