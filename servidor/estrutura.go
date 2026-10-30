package main

import (
	"sync"

	"vaijunto/comum"
)

type usuario struct {
	Nome      string
	Login     string
	SenhaHash [32]byte
	Perfil    string
}

type itinerarioPendente struct {
	Passageiro string
	Itinerario comum.Itinerario
	Usado      bool
}

// Estado concentra os dados compartilhados entre os clientes.
// Um mutex só resolve o ponto principal: enquanto uma reserva confere e
// desconta as vagas, outra não pode pegar o mesmo assento no meio do caminho.
type Estado struct {
	mutex           sync.Mutex
	empresaID       string
	empresaNome     string
	arquivoUsuarios string
	arquivoEstado   string
	usuarios        map[string]usuario
	sessoes         map[string]string
	caronas         map[string]*comum.Carona
	reservas        map[string]*comum.Reserva
	itinerarios     map[string]*itinerarioPendente

	proximaCarona     int
	proximoItinerario int
	proximaReserva    int
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
		empresaID:   empresaID,
		empresaNome: empresaNome,
		usuarios:    make(map[string]usuario),
		sessoes:     make(map[string]string),
		caronas:     make(map[string]*comum.Carona),
		reservas:    make(map[string]*comum.Reserva),
		itinerarios: make(map[string]*itinerarioPendente),
	}

	estado.adicionarUsuario("João Motorista", "motorista", "1234", comum.PerfilMotorista)
	estado.adicionarUsuario("Maria Passageira", "cliente", "1234", comum.PerfilPassageiro)
	return estado
}
