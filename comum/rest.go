package comum

import "time"

const (
	StatusEmpresaOnline   = "online"
	CabecalhoRequestID    = "X-Request-ID"
	CabecalhoIdempotencia = "Idempotency-Key"
)

// SaudeEmpresa é a resposta mínima comum a todos os servidores do consórcio.
type SaudeEmpresa struct {
	EmpresaID   string `json:"empresa_id"`
	EmpresaNome string `json:"empresa_nome"`
	Status      string `json:"status"`
}

// SaudePar registra o resultado de uma tentativa de comunicação REST. Uma
// empresa indisponível aparece na resposta, mas não derruba a empresa local.
type SaudePar struct {
	EmpresaID  string `json:"empresa_id"`
	URLBase    string `json:"url_base"`
	Disponivel bool   `json:"disponivel"`
	Status     string `json:"status,omitempty"`
	DuracaoMS  int64  `json:"duracao_ms"`
	Erro       string `json:"erro,omitempty"`
}

type SaudeConsorcio struct {
	EmpresaID string     `json:"empresa_id"`
	Status    string     `json:"status"`
	Pares     []SaudePar `json:"pares"`
}

// TrechoDisponivelREST é o catálogo mínimo compartilhado por uma empresa.
// Dados de contas, tokens e reservas não atravessam a fronteira da empresa.
type TrechoDisponivelREST struct {
	EmpresaID     string    `json:"empresa_id"`
	CaronaID      string    `json:"carona_id"`
	Indice        int       `json:"indice"`
	Data          string    `json:"data"`
	Origem        string    `json:"origem"`
	Destino       string    `json:"destino"`
	Saida         time.Time `json:"saida"`
	Chegada       time.Time `json:"chegada"`
	PrecoCentavos int       `json:"preco_centavos"`
	Vagas         int       `json:"vagas"`
}

type RespostaBuscaTrechosREST struct {
	EmpresaID string                 `json:"empresa_id"`
	Trechos   []TrechoDisponivelREST `json:"trechos"`
}

const (
	StatusTransacaoPreparada  = "preparada"
	StatusTransacaoConfirmada = "confirmada"
	StatusTransacaoAbortada   = "abortada"
)

type TrechoTransacaoREST struct {
	CaronaID string `json:"carona_id"`
	Indice   int    `json:"indice"`
}

// PrepararReservaREST inicia a fase de preparação do protocolo de reserva.
// O prazo é contado pelo relógio do participante, sem pressupor relógios
// sincronizados entre as empresas.
type PrepararReservaREST struct {
	TransacaoID       string                `json:"transacao_id"`
	CoordenadorID     string                `json:"coordenador_id"`
	DuracaoBloqueioMS int64                 `json:"duracao_bloqueio_ms"`
	Trechos           []TrechoTransacaoREST `json:"trechos"`
}

type IdentificarTransacaoREST struct {
	TransacaoID string `json:"transacao_id"`
}

type RespostaTransacaoREST struct {
	EmpresaID   string `json:"empresa_id"`
	TransacaoID string `json:"transacao_id"`
	Status      string `json:"status"`
}

type ErroREST struct {
	Codigo   string `json:"codigo"`
	Mensagem string `json:"mensagem"`
}

type RespostaErroREST struct {
	Erro ErroREST `json:"erro"`
}
