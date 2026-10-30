package comum

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

type ErroREST struct {
	Codigo   string `json:"codigo"`
	Mensagem string `json:"mensagem"`
}

type RespostaErroREST struct {
	Erro ErroREST `json:"erro"`
}
