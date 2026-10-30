package main

import (
	"fmt"

	"vaijunto/comum"
	"vaijunto/terminal"
)

func buscarItinerarios(conexao *comum.ConexaoTCP) {
	if !estaLogado(conexao) {
		return
	}
	// Primeiro pergunta as cidades. A data vem das opções que o servidor achar,
	// para o passageiro não precisar adivinhar em qual dia existe uma carona.
	dados := comum.BuscarItinerariosDados{
		Origem:  entrada.Texto("Cidade de origem: "),
		Destino: entrada.Texto("Cidade de destino: "),
	}
	resposta, err := conexao.Enviar(comum.AcaoBuscarItinerarios, dados)
	if !mostrarErro(resposta, err) {
		return
	}
	itinerarios, err := comum.LerDados[[]comum.Itinerario](resposta)
	if err != nil {
		fmt.Println("Resposta do servidor inválida:", err)
		return
	}
	if len(itinerarios) == 0 {
		fmt.Printf("Nenhuma corrida com vagas encontrada de %s para %s.\n",
			dados.Origem, dados.Destino)
		return
	}

	// Várias corridas podem cair no mesmo dia; o menu mostra cada data uma vez.
	datas := make([]string, 0)
	datasAdicionadas := make(map[string]bool)
	for _, itinerario := range itinerarios {
		if !datasAdicionadas[itinerario.Data] {
			datas = append(datas, itinerario.Data)
			datasAdicionadas[itinerario.Data] = true
		}
	}

	fmt.Println("\n--- Datas disponíveis ---")
	for i, data := range datas {
		fmt.Printf("%d) %s\n", i+1, terminal.FormatarData(data))
	}
	escolhaData := entrada.Opcao("Escolha uma data (0 para voltar): ", len(datas))
	if escolhaData == 0 {
		fmt.Println("Busca encerrada.")
		return
	}
	dataEscolhida := datas[escolhaData-1]

	corridasDaData := make([]comum.Itinerario, 0)
	for _, itinerario := range itinerarios {
		if itinerario.Data == dataEscolhida {
			corridasDaData = append(corridasDaData, itinerario)
		}
	}
	itinerarios = corridasDaData

	fmt.Printf("\n--- Corridas de %s ---\n", terminal.FormatarData(dataEscolhida))
	for i, itinerario := range itinerarios {
		fmt.Printf("\n%d) %s às %s\n", i+1, terminal.FormatarData(itinerario.Data), itinerario.Trechos[0].Saida.Format("15:04"))
		fmt.Printf("   %s -> %s\n", itinerario.Origem, itinerario.Destino)
		fmt.Printf("   Total: %s | Conexões: %d | Vagas: %d\n",
			terminal.FormatarDinheiro(itinerario.PrecoCentavos), itinerario.Conexoes, itinerario.Vagas)
		for _, trecho := range itinerario.Trechos {
			fmt.Printf("     %s -> %s | %s até %s | %s | %s\n",
				trecho.Origem, trecho.Destino,
				trecho.Saida.Format("15:04"), trecho.Chegada.Format("15:04"),
				terminal.FormatarDinheiro(trecho.PrecoCentavos), trecho.CaronaID)
		}
	}

	escolha := entrada.Opcao("\nEscolha uma corrida (0 para voltar): ", len(itinerarios))
	if escolha == 0 {
		fmt.Println("Reserva não realizada.")
		return
	}
	confirmarReserva(conexao, itinerarios[escolha-1])
}
