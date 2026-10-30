package comum

import "encoding/json"

const (
	AcaoLogin                = "login"
	AcaoCadastro             = "cadastro"
	AcaoPublicarCarona       = "publicar_carona"
	AcaoListarCaronas        = "listar_caronas"
	AcaoListarPassageiros    = "listar_passageiros"
	AcaoCancelarCarona       = "cancelar_carona"
	AcaoBuscarItinerarios    = "buscar_itinerarios"
	AcaoConfirmarReserva     = "confirmar_reserva"
	AcaoListarReservas       = "listar_reservas"
	AcaoCancelarReserva      = "cancelar_reserva"
	PerfilMotorista          = "motorista"
	PerfilPassageiro         = "passageiro"
	StatusReservaConfirmada  = "confirmada"
	StatusReservaCancelada   = "cancelada pelo passageiro"
	StatusCanceladaMotorista = "cancelada pelo motorista"
)

// Pedido e Resposta mantêm a mesma estrutura em todas as mensagens TCP.
// Dados fica como JSON até sabermos a ação: login pede usuário e senha,
// enquanto uma publicação precisa de rota, preços, horários e assentos.
type Pedido struct {
	ID    string          `json:"id"`
	Acao  string          `json:"acao"`
	Token string          `json:"token,omitempty"`
	Dados json.RawMessage `json:"dados,omitempty"`
}

type Resposta struct {
	ID       string          `json:"id"`
	Sucesso  bool            `json:"sucesso"`
	Mensagem string          `json:"mensagem"`
	Dados    json.RawMessage `json:"dados,omitempty"`
}

type LoginDados struct {
	Usuario string `json:"usuario"`
	Senha   string `json:"senha"`
}

type CadastroDados struct {
	Nome    string `json:"nome"`
	Usuario string `json:"usuario"`
	Senha   string `json:"senha"`
	Perfil  string `json:"perfil"`
}

type LoginResposta struct {
	Token  string `json:"token"`
	Nome   string `json:"nome"`
	Perfil string `json:"perfil"`
}

type PublicarCaronaDados struct {
	Rota           []string `json:"rota"`
	Data           string   `json:"data"`
	Horario        string   `json:"horario"`
	Assentos       int      `json:"assentos"`
	PrecosCentavos []int    `json:"precos_centavos"`
	DuracoesMin    []int    `json:"duracoes_min"`
}

type BuscarItinerariosDados struct {
	Origem  string `json:"origem"`
	Destino string `json:"destino"`
	Data    string `json:"data"`
}

type IDDados struct {
	ID string `json:"id"`
}
