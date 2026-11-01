package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"vaijunto/comum"
)

type trechoDemonstracao struct {
	EmpresaID string
	Rota      []string
	Horario   string
	Duracao   int
	Preco     int
}

func main() {
	var empresas, data, usuario, senha string
	var assentos int
	flag.StringVar(&empresas, "empresas", valorAmbiente("VAIJUNTO_PREP_EMPRESAS", "empresa-a=127.0.0.1:9001,empresa-b=127.0.0.1:9002,empresa-c=127.0.0.1:9003"), "IDs e endereços TCP")
	flag.StringVar(&data, "data", "2026-11-05", "data da viagem no formato AAAA-MM-DD")
	flag.StringVar(&usuario, "usuario", "motorista", "usuário motorista existente nas empresas")
	flag.StringVar(&senha, "senha", "1234", "senha do motorista")
	flag.IntVar(&assentos, "assentos", 1, "assentos de cada carona")
	flag.Parse()

	enderecos, err := interpretarEmpresas(empresas)
	if err != nil {
		encerrar(err)
	}
	if _, err := time.Parse("2006-01-02", data); err != nil {
		encerrar(errors.New("data deve estar no formato AAAA-MM-DD"))
	}
	if assentos < 1 {
		encerrar(errors.New("assentos deve ser maior que zero"))
	}

	cenario := []trechoDemonstracao{
		{EmpresaID: "empresa-a", Rota: []string{"Salvador", "Feira de Santana"}, Horario: "08:00", Duracao: 60, Preco: 2500},
		{EmpresaID: "empresa-b", Rota: []string{"Feira de Santana", "Vitória da Conquista"}, Horario: "09:20", Duracao: 120, Preco: 3500},
		{EmpresaID: "empresa-c", Rota: []string{"Vitória da Conquista", "Montes Claros"}, Horario: "11:40", Duracao: 180, Preco: 4500},
	}

	fmt.Printf("Preparando itinerário distribuído para %s, com %d assento(s) por trecho.\n", data, assentos)
	for _, trecho := range cenario {
		endereco, existe := enderecos[trecho.EmpresaID]
		if !existe {
			encerrar(fmt.Errorf("endereço de %s não foi informado", trecho.EmpresaID))
		}
		carona, criada, err := garantirCarona(endereco, usuario, senha, data, assentos, trecho)
		if err != nil {
			encerrar(fmt.Errorf("%s: %w", trecho.EmpresaID, err))
		}
		acao := "reutilizada"
		if criada {
			acao = "criada"
		}
		fmt.Printf("- %s: %s %s (%s → %s, %s)\n", trecho.EmpresaID, carona.ID, acao, trecho.Rota[0], trecho.Rota[1], trecho.Horario)
	}
	fmt.Println("Cenário pronto: Salvador → Feira de Santana → Vitória da Conquista → Montes Claros.")
}

func garantirCarona(endereco, usuario, senha, data string, assentos int, trecho trechoDemonstracao) (comum.Carona, bool, error) {
	conexao, err := comum.Conectar(endereco)
	if err != nil {
		return comum.Carona{}, false, fmt.Errorf("conexão TCP: %w", err)
	}
	defer conexao.Fechar()
	_ = conexao.DefinirPrazo(time.Now().Add(10 * time.Second))

	resposta, err := conexao.Enviar(comum.AcaoLogin, comum.LoginDados{Usuario: usuario, Senha: senha})
	if err != nil || !resposta.Sucesso {
		return comum.Carona{}, false, fmt.Errorf("login: %s", mensagem(resposta, err))
	}
	login, err := comum.LerDados[comum.LoginResposta](resposta)
	if err != nil {
		return comum.Carona{}, false, fmt.Errorf("resposta de login inválida: %w", err)
	}
	conexao.Token = login.Token

	resposta, err = conexao.Enviar(comum.AcaoListarCaronas, struct{}{})
	if err != nil || !resposta.Sucesso {
		return comum.Carona{}, false, fmt.Errorf("listagem: %s", mensagem(resposta, err))
	}
	cadastradas, err := comum.LerDados[[]comum.Carona](resposta)
	if err != nil {
		return comum.Carona{}, false, fmt.Errorf("listagem inválida: %w", err)
	}
	for _, carona := range cadastradas {
		if caronaIgual(carona, data, assentos, trecho) {
			return carona, false, nil
		}
	}

	resposta, err = conexao.Enviar(comum.AcaoPublicarCarona, comum.PublicarCaronaDados{
		Rota:           trecho.Rota,
		Data:           data,
		Horario:        trecho.Horario,
		Assentos:       assentos,
		PrecosCentavos: []int{trecho.Preco},
		DuracoesMin:    []int{trecho.Duracao},
	})
	if err != nil || !resposta.Sucesso {
		return comum.Carona{}, false, fmt.Errorf("publicação: %s", mensagem(resposta, err))
	}
	carona, err := comum.LerDados[comum.Carona](resposta)
	if err != nil {
		return comum.Carona{}, false, fmt.Errorf("publicação inválida: %w", err)
	}
	return carona, true, nil
}

func caronaIgual(carona comum.Carona, data string, assentos int, trecho trechoDemonstracao) bool {
	return !carona.Cancelada && carona.Data == data && carona.Horario == trecho.Horario &&
		carona.Assentos == assentos && len(carona.Rota) == 2 &&
		strings.EqualFold(carona.Rota[0], trecho.Rota[0]) && strings.EqualFold(carona.Rota[1], trecho.Rota[1]) &&
		len(carona.PrecosCentavos) == 1 && carona.PrecosCentavos[0] == trecho.Preco &&
		len(carona.DuracoesMin) == 1 && carona.DuracoesMin[0] == trecho.Duracao &&
		len(carona.DisponiveisPorTrecho) == 1 && carona.DisponiveisPorTrecho[0] == assentos
}

func interpretarEmpresas(valor string) (map[string]string, error) {
	resultado := make(map[string]string)
	for _, item := range strings.Split(valor, ",") {
		partes := strings.SplitN(strings.TrimSpace(item), "=", 2)
		if len(partes) != 2 || strings.TrimSpace(partes[0]) == "" || strings.TrimSpace(partes[1]) == "" {
			return nil, fmt.Errorf("empresa inválida %q; use id=endereço", item)
		}
		id, endereco := strings.TrimSpace(partes[0]), strings.TrimSpace(partes[1])
		if _, existe := resultado[id]; existe {
			return nil, fmt.Errorf("empresa repetida: %s", id)
		}
		resultado[id] = endereco
	}
	ids := make([]string, 0, len(resultado))
	for id := range resultado {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	if len(ids) != 3 {
		return nil, fmt.Errorf("esperadas 3 empresas, recebidas %d: %s", len(ids), strings.Join(ids, ", "))
	}
	return resultado, nil
}

func mensagem(resposta comum.Resposta, err error) string {
	if err != nil {
		return err.Error()
	}
	return resposta.Mensagem
}

func valorAmbiente(chave, padrao string) string {
	if valor := os.Getenv(chave); valor != "" {
		return valor
	}
	return padrao
}

func encerrar(err error) {
	fmt.Fprintln(os.Stderr, "Não foi possível preparar a demonstração:", err)
	os.Exit(1)
}
