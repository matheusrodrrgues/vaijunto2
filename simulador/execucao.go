package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os/exec"
	"sort"
	"strings"
	"sync"
	"time"

	"vaijunto/comum"
)

type resultadoOperacao struct {
	Empresa   string
	Categoria string
	Duracao   time.Duration
	Parcial   bool
	Reserva   *comum.Reserva
	Erro      string
}

type coletor struct {
	mutex      sync.Mutex
	resultados []resultadoOperacao
	falhas     []string
	errosFalha []string
}

func (c *coletor) adicionar(resultado resultadoOperacao) {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	c.resultados = append(c.resultados, resultado)
}

func (c *coletor) registrarFalha(mensagem string) {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	c.falhas = append(c.falhas, mensagem)
}

func (c *coletor) registrarErroFalha(mensagem string) {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	c.errosFalha = append(c.errosFalha, mensagem)
}

func executarSimulacao(ctx context.Context, config configuracao) (Relatorio, error) {
	alvos, err := config.alvos()
	if err != nil {
		return Relatorio{}, err
	}
	inicio := time.Now()
	coletor := &coletor{}
	ctxCarga, cancelarCarga := context.WithCancel(ctx)
	if config.duracao > 0 {
		ctxCarga, cancelarCarga = context.WithTimeout(ctx, config.duracao)
	}
	defer cancelarCarga()

	fimFalha := make(chan error, 1)
	if config.servicoFalha != "" {
		go func() {
			fimFalha <- injetarFalha(ctxCarga, config, coletor)
		}()
	} else {
		close(fimFalha)
	}

	trabalhos := make(chan int, config.concorrencia*2)
	var grupo sync.WaitGroup
	for i := 0; i < config.concorrencia; i++ {
		grupo.Add(1)
		go func() {
			defer grupo.Done()
			for numero := range trabalhos {
				alvo := alvos[numero%len(alvos)]
				coletor.adicionar(executarPassageiro(ctxCarga, config, alvo, numero))
			}
		}()
	}

	gerarTrabalhos(ctxCarga, config, trabalhos)
	close(trabalhos)
	grupo.Wait()
	cancelarCarga()
	if config.servicoFalha != "" {
		if err := <-fimFalha; err != nil {
			coletor.registrarErroFalha(err.Error())
		}
	}
	fimCarga := time.Now()
	relatorio := montarRelatorio(config, alvos, inicio, fimCarga, coletor)
	if config.esperaRecuperacao > 0 {
		time.Sleep(config.esperaRecuperacao)
	}
	verificarResultado(config, alvos, &relatorio)
	if len(relatorio.Inconsistencias) > 0 {
		return relatorio, errors.New("foram encontradas inconsistências")
	}
	return relatorio, nil
}

func gerarTrabalhos(ctx context.Context, config configuracao, trabalhos chan<- int) {
	if config.duracao > 0 {
		for numero := 0; ; numero++ {
			select {
			case <-ctx.Done():
				return
			case trabalhos <- numero:
			}
		}
	}
	for numero := 0; numero < config.requisicoes; numero++ {
		select {
		case <-ctx.Done():
			return
		case trabalhos <- numero:
		}
	}
}

func executarPassageiro(ctx context.Context, config configuracao, alvo empresaAlvo, numero int) resultadoOperacao {
	inicio := time.Now()
	resultado := resultadoOperacao{Empresa: alvo.ID}
	if err := ctx.Err(); err != nil {
		resultado.Categoria, resultado.Erro = "cancelada", err.Error()
		return resultado
	}
	conexao, err := comum.Conectar(alvo.TCP)
	if err != nil {
		resultado.Categoria, resultado.Erro = "erro_conexao", err.Error()
		resultado.Duracao = time.Since(inicio)
		return resultado
	}
	defer conexao.Fechar()
	_ = conexao.DefinirPrazo(time.Now().Add(15 * time.Second))

	resposta, err := conexao.Enviar(comum.AcaoLogin, comum.LoginDados{Usuario: config.usuario, Senha: config.senha})
	if err != nil || !resposta.Sucesso {
		resultado.Categoria, resultado.Erro = "erro_login", mensagemResposta(resposta, err)
		resultado.Duracao = time.Since(inicio)
		return resultado
	}
	login, err := comum.LerDados[comum.LoginResposta](resposta)
	if err != nil {
		resultado.Categoria, resultado.Erro = "erro_protocolo", err.Error()
		resultado.Duracao = time.Since(inicio)
		return resultado
	}
	conexao.Token = login.Token

	resposta, err = conexao.Enviar(comum.AcaoBuscarItinerarios, comum.BuscarItinerariosDados{
		Origem: config.origem, Destino: config.destino, Data: config.data,
	})
	if err != nil || !resposta.Sucesso {
		resultado.Categoria, resultado.Erro = "erro_busca", mensagemResposta(resposta, err)
		resultado.Duracao = time.Since(inicio)
		return resultado
	}
	busca, err := comum.LerDados[comum.ResultadoBuscaItinerarios](resposta)
	if err != nil {
		resultado.Categoria, resultado.Erro = "erro_protocolo", err.Error()
		resultado.Duracao = time.Since(inicio)
		return resultado
	}
	resultado.Parcial = busca.BuscaParcial
	if len(busca.Itinerarios) == 0 {
		resultado.Categoria = "sem_itinerario"
		resultado.Duracao = time.Since(inicio)
		return resultado
	}
	escolhida := busca.Itinerarios[numero%len(busca.Itinerarios)]
	resposta, err = conexao.Enviar(comum.AcaoConfirmarReserva, comum.IDDados{ID: escolhida.ID})
	if err != nil || !resposta.Sucesso {
		resultado.Categoria, resultado.Erro = "reserva_recusada", mensagemResposta(resposta, err)
		resultado.Duracao = time.Since(inicio)
		return resultado
	}
	reserva, err := comum.LerDados[comum.Reserva](resposta)
	if err != nil {
		resultado.Categoria, resultado.Erro = "erro_protocolo", err.Error()
		resultado.Duracao = time.Since(inicio)
		return resultado
	}
	resultado.Categoria = "reserva_confirmada"
	resultado.Reserva = &reserva
	resultado.Duracao = time.Since(inicio)
	return resultado
}

func mensagemResposta(resposta comum.Resposta, err error) string {
	if err != nil {
		return err.Error()
	}
	return resposta.Mensagem
}

func injetarFalha(ctx context.Context, config configuracao, coletor *coletor) error {
	temporizador := time.NewTimer(config.falhaApos)
	defer temporizador.Stop()
	select {
	case <-ctx.Done():
		return errors.New("a carga terminou antes da injeção de falha")
	case <-temporizador.C:
	}
	if err := executarDockerCompose(config.diretorioCompose, "stop", config.servicoFalha); err != nil {
		return fmt.Errorf("não foi possível interromper %s: %w", config.servicoFalha, err)
	}
	coletor.registrarFalha("serviço " + config.servicoFalha + " interrompido")
	time.Sleep(config.falhaDuracao)
	if err := executarDockerCompose(config.diretorioCompose, "start", config.servicoFalha); err != nil {
		return fmt.Errorf("não foi possível restaurar %s: %w", config.servicoFalha, err)
	}
	coletor.registrarFalha("serviço " + config.servicoFalha + " restaurado")
	return nil
}

func executarDockerCompose(diretorio string, argumentos ...string) error {
	args := append([]string{"compose"}, argumentos...)
	comando := exec.Command("docker", args...)
	comando.Dir = diretorio
	saida, err := comando.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%w: %s", err, strings.TrimSpace(string(saida)))
	}
	return nil
}

func verificarResultado(config configuracao, alvos []empresaAlvo, relatorio *Relatorio) {
	for _, erroFalha := range relatorio.ErrosInjecao {
		relatorio.Inconsistencias = append(relatorio.Inconsistencias, "injeção de falha: "+erroFalha)
	}
	porID := make(map[string]empresaAlvo, len(alvos))
	for _, alvo := range alvos {
		porID[alvo.ID] = alvo
	}
	relatorio.reservasConfirmadas = coletarReservasFinais(config, alvos, relatorio)
	relatorio.ReservasConfirmadasFinais = len(relatorio.reservasConfirmadas)
	ocupacao := make(map[string]int)
	for _, reserva := range relatorio.reservasConfirmadas {
		for _, trecho := range reserva.Itinerario.Trechos {
			chave := trecho.EmpresaID + "|" + trecho.CaronaID + "|" + fmt.Sprint(trecho.Indice)
			ocupacao[chave]++
			if ocupacao[chave] > config.capacidadeEsperada {
				relatorio.Inconsistencias = append(relatorio.Inconsistencias,
					fmt.Sprintf("trecho %s excedeu a capacidade: %d > %d", chave, ocupacao[chave], config.capacidadeEsperada))
			}
		}
		if reserva.TransacaoID != "" {
			verificarReservaDistribuida(reserva, porID, relatorio)
		}
	}
	for _, alvo := range alvos {
		resumo, err := consultarJSON[comum.ResumoTransacoesREST](alvo.URLREST + "/api/v1/diagnostics/transactions")
		if err != nil {
			relatorio.Inconsistencias = append(relatorio.Inconsistencias,
				fmt.Sprintf("não foi possível consultar diagnóstico de %s: %v", alvo.ID, err))
			continue
		}
		relatorio.Diagnosticos = append(relatorio.Diagnosticos, resumo)
		if resumo.Preparadas > 0 || resumo.CoordenacoesPendentes > 0 {
			relatorio.Inconsistencias = append(relatorio.Inconsistencias,
				fmt.Sprintf("%s terminou com preparadas=%d e coordenações pendentes=%d",
					alvo.ID, resumo.Preparadas, resumo.CoordenacoesPendentes))
		}
	}
	sort.Strings(relatorio.Inconsistencias)
	relatorio.Aprovado = len(relatorio.Inconsistencias) == 0
}

func coletarReservasFinais(config configuracao, alvos []empresaAlvo, relatorio *Relatorio) []comum.Reserva {
	reservas := make([]comum.Reserva, 0)
	for _, alvo := range alvos {
		conexao, err := comum.Conectar(alvo.TCP)
		if err != nil {
			relatorio.Inconsistencias = append(relatorio.Inconsistencias,
				fmt.Sprintf("não foi possível listar reservas de %s: %v", alvo.ID, err))
			continue
		}
		_ = conexao.DefinirPrazo(time.Now().Add(10 * time.Second))
		resposta, err := conexao.Enviar(comum.AcaoLogin, comum.LoginDados{Usuario: config.usuario, Senha: config.senha})
		if err != nil || !resposta.Sucesso {
			conexao.Fechar()
			relatorio.Inconsistencias = append(relatorio.Inconsistencias, "login de auditoria falhou em "+alvo.ID)
			continue
		}
		login, err := comum.LerDados[comum.LoginResposta](resposta)
		if err != nil {
			conexao.Fechar()
			relatorio.Inconsistencias = append(relatorio.Inconsistencias, "login de auditoria inválido em "+alvo.ID)
			continue
		}
		conexao.Token = login.Token
		resposta, err = conexao.Enviar(comum.AcaoListarReservas, struct{}{})
		conexao.Fechar()
		if err != nil || !resposta.Sucesso {
			relatorio.Inconsistencias = append(relatorio.Inconsistencias, "listagem de reservas falhou em "+alvo.ID)
			continue
		}
		lista, err := comum.LerDados[[]comum.Reserva](resposta)
		if err != nil {
			relatorio.Inconsistencias = append(relatorio.Inconsistencias, "listagem de reservas inválida em "+alvo.ID)
			continue
		}
		for _, reserva := range lista {
			if reserva.Status == comum.StatusReservaConfirmada {
				reservas = append(reservas, reserva)
			}
		}
	}
	return reservas
}

func verificarReservaDistribuida(reserva comum.Reserva, alvos map[string]empresaAlvo, relatorio *Relatorio) {
	empresas := make(map[string]bool)
	for _, trecho := range reserva.Itinerario.Trechos {
		empresas[trecho.EmpresaID] = true
	}
	for empresaID := range empresas {
		alvo, ok := alvos[empresaID]
		if !ok {
			relatorio.Inconsistencias = append(relatorio.Inconsistencias, "empresa sem endpoint REST: "+empresaID)
			continue
		}
		url := alvo.URLREST + "/api/v1/reservations/status?transaction_id=" + reserva.TransacaoID
		status, err := consultarJSON[comum.RespostaTransacaoREST](url)
		if err != nil || status.Status != comum.StatusTransacaoConfirmada {
			relatorio.Inconsistencias = append(relatorio.Inconsistencias,
				fmt.Sprintf("reserva %s não está confirmada em %s", reserva.ID, empresaID))
		}
	}
}

func consultarJSON[T any](url string) (T, error) {
	var resultado T
	cliente := &http.Client{Timeout: 3 * time.Second}
	resposta, err := cliente.Get(url)
	if err != nil {
		return resultado, err
	}
	defer resposta.Body.Close()
	if resposta.StatusCode != http.StatusOK {
		return resultado, fmt.Errorf("HTTP %d", resposta.StatusCode)
	}
	if err := json.NewDecoder(resposta.Body).Decode(&resultado); err != nil {
		return resultado, err
	}
	return resultado, nil
}
