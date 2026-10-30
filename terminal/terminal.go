package terminal

import (
	"bufio"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"
)

type Entrada struct {
	leitor *bufio.Reader
}

func NovaEntrada(origem io.Reader) *Entrada {
	return &Entrada{leitor: bufio.NewReader(origem)}
}

func (e *Entrada) Texto(mensagem string) string {
	fmt.Print(mensagem)
	texto, _ := e.leitor.ReadString('\n')
	return strings.TrimSpace(texto)
}

func (e *Entrada) InteiroMinimo(mensagem string, minimo int) int {
	for {
		valor, err := strconv.Atoi(e.Texto(mensagem))
		if err == nil && valor >= minimo {
			return valor
		}
		fmt.Printf("Digite um número inteiro maior ou igual a %d.\n", minimo)
	}
}

func (e *Entrada) Opcao(mensagem string, maximo int) int {
	for {
		valor, err := strconv.Atoi(e.Texto(mensagem))
		if err == nil && valor >= 0 && valor <= maximo {
			return valor
		}
		fmt.Printf("Escolha uma opção entre 0 e %d.\n", maximo)
	}
}

// DinheiroCentavos aceita vírgula ou ponto e trabalha com inteiros.
// R$ 30,50 vira 3050, sem depender de arredondamento ao somar os preços.
func (e *Entrada) DinheiroCentavos(mensagem string) int {
	for {
		texto := strings.ReplaceAll(e.Texto(mensagem), ",", ".")
		partes := strings.Split(texto, ".")
		if len(partes) >= 1 && len(partes) <= 2 {
			reais, erroReais := strconv.Atoi(partes[0])
			centavos := 0
			var erroCentavos error
			if len(partes) == 2 {
				if len(partes[1]) == 1 {
					partes[1] += "0"
				}
				if len(partes[1]) != 2 {
					erroCentavos = fmt.Errorf("quantidade inválida de casas decimais")
				} else {
					centavos, erroCentavos = strconv.Atoi(partes[1])
				}
			}
			if erroReais == nil && erroCentavos == nil && reais >= 0 && centavos >= 0 {
				return reais*100 + centavos
			}
		}
		fmt.Println("Digite um valor como 30, 30,50 ou 30.50.")
	}
}

// Data deixa a pessoa digitar dia/mês/ano, como costuma usar.
// Antes de enviar, converte para ano-mês-dia, que é o formato combinado no JSON.
func (e *Entrada) Data(mensagem string) string {
	for {
		texto := e.Texto(mensagem)
		data, err := time.Parse("02/01/2006", texto)
		if err == nil {
			return data.Format("2006-01-02")
		}
		fmt.Println("Digite a data no formato DIA/MÊS/ANO. Exemplo: 15/09/2026.")
	}
}

func (e *Entrada) Pausa() {
	e.Texto("\nPressione ENTER para continuar...")
}

func FormatarDinheiro(centavos int) string {
	return fmt.Sprintf("R$ %d,%02d", centavos/100, centavos%100)
}

func FormatarData(dataInterna string) string {
	data, err := time.Parse("2006-01-02", dataInterna)
	if err != nil {
		return dataInterna
	}
	return data.Format("02/01/2006")
}
