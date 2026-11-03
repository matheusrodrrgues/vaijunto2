package comum

// Abre a conexão TCP, envia JSON, espera a resposta e associa cada resposta ao pedido correto.
import (
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"sync"
	"sync/atomic"
	"time"
)

// ConexaoTCP deixa o código dos dois clientes menor e evita repetir
// a parte de enviar e receber JSON.
type ConexaoTCP struct {
	conexao    net.Conn
	encoder    *json.Encoder
	decoder    *json.Decoder
	mutex      sync.Mutex
	contador   atomic.Uint64
	Token      string
	NomeLogado string
}

// abre a conexão tcp com o servidor e retorna a conexão pronta para enviar pedidos.
func Conectar(endereco string) (*ConexaoTCP, error) {
	conexao, err := net.DialTimeout("tcp", endereco, 5*time.Second)
	if err != nil {
		return nil, err
	}

	return NovaConexao(conexao), nil
}

// NovaConexao cria uma conexão TCP a partir de uma conexão já aberta.
func NovaConexao(conexao net.Conn) *ConexaoTCP {
	return &ConexaoTCP{
		conexao: conexao,
		encoder: json.NewEncoder(conexao),
		decoder: json.NewDecoder(conexao),
	}
}

// Enviar faz um pedido e espera a resposta antes de liberar esta conexão.
// A trava evita misturar as respostas se duas goroutines usarem o mesmo cliente.
func (c *ConexaoTCP) Enviar(acao string, dados any) (Resposta, error) {
	// impede duas goroutines de enviar pedidos ao mesmo tempo, o que misturaria as respostas.
	c.mutex.Lock()         // goroutine 1: envia pedido 1
	defer c.mutex.Unlock() // goroutine 1: recebe resposta 1

	jsonDados, err := json.Marshal(dados)
	if err != nil {
		return Resposta{}, err
	}
	// cada pedido tem um identificador único, que o servidor devolve na resposta.
	id := fmt.Sprintf("REQ-%d", c.contador.Add(1))
	pedido := Pedido{ID: id, Acao: acao, Token: c.Token, Dados: jsonDados}
	if err := c.encoder.Encode(pedido); err != nil {
		return Resposta{}, err
	}
	// espera a resposta do servidor
	var resposta Resposta
	if err := c.decoder.Decode(&resposta); err != nil {
		return Resposta{}, err
	}
	// O id confere qual pedido foi respondido; não impede pedidos repetidos.
	if resposta.ID != id {
		return Resposta{}, errors.New("o servidor respondeu com um identificador diferente")
	}

	return resposta, nil
}

func (c *ConexaoTCP) Fechar() error {
	return c.conexao.Close()
}

// DefinirPrazo limita leituras e escritas da conexão. Clientes interativos não
// precisam usá-lo; simuladores evitam ficar presos a um servidor interrompido.
func (c *ConexaoTCP) DefinirPrazo(prazo time.Time) error {
	return c.conexao.SetDeadline(prazo)
}

// converte os dados da resposta para o tipo json. Se não houver dados, retorna o zero do tipo.
func LerDados[T any](resposta Resposta) (T, error) {
	var dados T
	if len(resposta.Dados) == 0 {
		return dados, nil
	}
	err := json.Unmarshal(resposta.Dados, &dados)
	return dados, err
}
