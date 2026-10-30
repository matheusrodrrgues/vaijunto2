package comum

import "time"

// Carona é a viagem publicada pelo motorista. Nas listas por trecho,
// a posição i corresponde ao deslocamento de Rota[i] para Rota[i+1].
type Carona struct {
	ID                   string   `json:"id"`
	EmpresaID            string   `json:"empresa_id"`
	Motorista            string   `json:"motorista"`
	Rota                 []string `json:"rota"`
	Data                 string   `json:"data"`
	Horario              string   `json:"horario"`
	Assentos             int      `json:"assentos"`
	PrecosCentavos       []int    `json:"precos_centavos"`
	DuracoesMin          []int    `json:"duracoes_min"`
	DisponiveisPorTrecho []int    `json:"disponiveis_por_trecho"`
	Cancelada            bool     `json:"cancelada"`
}

type TrechoItinerario struct {
	CaronaID      string    `json:"carona_id"`
	Indice        int       `json:"indice"`
	Motorista     string    `json:"motorista"`
	Origem        string    `json:"origem"`
	Destino       string    `json:"destino"`
	Saida         time.Time `json:"saida"`
	Chegada       time.Time `json:"chegada"`
	PrecoCentavos int       `json:"preco_centavos"`
}

// Itinerário é uma opção de viagem, que pode juntar mais de uma carona.
// As vagas refletem a consulta; quem confirma precisa verificar de novo.
type Itinerario struct {
	ID            string             `json:"id"`
	Origem        string             `json:"origem"`
	Destino       string             `json:"destino"`
	Data          string             `json:"data"`
	PrecoCentavos int                `json:"preco_centavos"`
	Conexoes      int                `json:"conexoes"`
	Vagas         int                `json:"vagas"`
	Trechos       []TrechoItinerario `json:"trechos"`
}

// Reserva guarda o itinerário escolhido e continua no histórico se for cancelada.
type Reserva struct {
	ID             string     `json:"id"`
	Passageiro     string     `json:"passageiro"`
	NomePassageiro string     `json:"nome_passageiro"`
	Status         string     `json:"status"`
	CriadaEm       time.Time  `json:"criada_em"`
	Itinerario     Itinerario `json:"itinerario"`
}
