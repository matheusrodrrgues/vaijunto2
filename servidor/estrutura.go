package main

import (
	"sync"
	"time"

	"vaijunto/comum"
)

type usuario struct {
	Nome      string
	Login     string
	SenhaHash [32]byte
	Perfil    string
}

type itinerarioPendente struct {
	Passageiro      string
	Itinerario      comum.Itinerario
	Usado           bool
	EmProcessamento bool
}

type transacaoParticipante struct {
	ID            string                      `json:"id"`
	CoordenadorID string                      `json:"coordenador_id"`
	Status        string                      `json:"status"`
	Trechos       []comum.TrechoTransacaoREST `json:"trechos"`
	ExpiraEm      time.Time                   `json:"expira_em"`
}

const (
	statusCoordenacaoCommit    = "decisao_commit"
	statusCoordenacaoAbort     = "decisao_abort"
	statusCoordenacaoConcluida = "concluida"
	statusCoordenacaoAbortada  = "abortada"
)

type participanteCoordenacao struct {
	EmpresaID string `json:"empresa_id"`
	URLBase   string `json:"url_base,omitempty"`
	Local     bool   `json:"local"`
}

type transacaoCoordenador struct {
	ID              string                    `json:"id"`
	Status          string                    `json:"status"`
	ItinerarioID    string                    `json:"itinerario_id,omitempty"`
	Reserva         comum.Reserva             `json:"reserva"`
	Participantes   []participanteCoordenacao `json:"participantes"`
	UltimoErro      string                    `json:"ultimo_erro,omitempty"`
	AtualizadaEm    time.Time                 `json:"atualizada_em"`
	EmProcessamento bool                      `json:"-"`
}

// Estado concentra os dados compartilhados entre os clientes.
// Um mutex só resolve o ponto principal: enquanto uma reserva confere e
// desconta as vagas, outra não pode pegar o mesmo assento no meio do caminho.
type Estado struct {
	mutex               sync.Mutex
	empresaID           string
	empresaNome         string
	arquivoUsuarios     string
	arquivoEstado       string
	usuarios            map[string]usuario
	sessoes             map[string]string
	caronas             map[string]*comum.Carona
	reservas            map[string]*comum.Reserva
	itinerarios         map[string]*itinerarioPendente
	transacoes          map[string]*transacaoParticipante
	coordenacoes        map[string]*transacaoCoordenador
	pares               []EmpresaRemota
	clienteREST         *ClienteRESTConsorcio
	timeoutREST         time.Duration
	recuperacaoIniciada bool
	ganchoAposCommit    func(int) error

	proximaCarona     int
	proximoItinerario int
	proximaReserva    int
}

// ConfigurarBuscaDistribuida informa ao estado quais empresas participam da
// busca. Cada servidor continua dono apenas dos dados de sua própria empresa.
func (e *Estado) ConfigurarBuscaDistribuida(pares []EmpresaRemota, timeout time.Duration) {
	e.mutex.Lock()
	e.pares = append([]EmpresaRemota(nil), pares...)
	e.timeoutREST = timeout
	e.clienteREST = NovoClienteRESTConsorcio(timeout)
	e.mutex.Unlock()
	e.iniciarRecuperacaoCoordenador()
}

// NovoEstado prepara um servidor em memória, útil também nos testes.
// No programa principal, os arquivos salvos são carregados depois desta etapa.
func NovoEstado() *Estado {
	return NovoEstadoEmpresa(empresaPadraoID, empresaPadraoNome)
}

// NovoEstadoEmpresa cria o estado isolado de uma única empresa. O servidor
// nunca recebe mapas ou arquivos compartilhados com outra instância.
func NovoEstadoEmpresa(empresaID, empresaNome string) *Estado {
	estado := &Estado{
		empresaID:    empresaID,
		empresaNome:  empresaNome,
		usuarios:     make(map[string]usuario),
		sessoes:      make(map[string]string),
		caronas:      make(map[string]*comum.Carona),
		reservas:     make(map[string]*comum.Reserva),
		itinerarios:  make(map[string]*itinerarioPendente),
		transacoes:   make(map[string]*transacaoParticipante),
		coordenacoes: make(map[string]*transacaoCoordenador),
		timeoutREST:  2 * time.Second,
	}
	estado.clienteREST = NovoClienteRESTConsorcio(estado.timeoutREST)

	estado.adicionarUsuario("João Motorista", "motorista", "1234", comum.PerfilMotorista)
	estado.adicionarUsuario("Maria Passageira", "cliente", "1234", comum.PerfilPassageiro)
	return estado
}
