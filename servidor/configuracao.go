package main

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

const (
	empresaPadraoID   = "vaijunto"
	empresaPadraoNome = "VaiJunto"
)

var empresaIDValido = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,31}$`)

// ConfiguracaoServidor reúne a identidade e os recursos exclusivos de uma
// empresa. Instâncias com IDs diferentes recebem diretórios diferentes por
// padrão, evitando que usuários, caronas, reservas e logs sejam misturados.
type ConfiguracaoServidor struct {
	EmpresaID       string
	EmpresaNome     string
	EnderecoTCP     string
	EnderecoREST    string
	TimeoutREST     time.Duration
	Pares           []EmpresaRemota
	ArquivoUsuarios string
	ArquivoEstado   string
	ArquivoLog      string
}

// EmpresaRemota identifica um servidor participante do consórcio. A URLBase
// aponta para a API REST da empresa, nunca para a porta TCP dos clientes.
type EmpresaRemota struct {
	ID      string
	URLBase string
}

func carregarConfiguracaoServidor() (ConfiguracaoServidor, error) {
	empresaID := valorAmbiente("VAIJUNTO_EMPRESA_ID", empresaPadraoID)
	empresaNome := valorAmbiente("VAIJUNTO_EMPRESA_NOME", empresaPadraoNome)
	enderecoTCP := valorAmbiente("VAIJUNTO_ADDR", ":9000")
	enderecoREST := valorAmbiente("VAIJUNTO_REST_ADDR", ":8080")
	timeoutREST, err := time.ParseDuration(valorAmbiente("VAIJUNTO_REST_TIMEOUT", "2s"))
	if err != nil || timeoutREST <= 0 {
		return ConfiguracaoServidor{}, errors.New("VAIJUNTO_REST_TIMEOUT deve ser uma duração positiva, como 2s")
	}
	pares, err := interpretarPares(os.Getenv("VAIJUNTO_PARES"), empresaID)
	if err != nil {
		return ConfiguracaoServidor{}, err
	}

	diretorioDados := os.Getenv("VAIJUNTO_DADOS_DIR")
	if strings.TrimSpace(diretorioDados) == "" {
		diretorioDados = filepath.Join("dados", empresaID)
	}
	diretorioLogs := os.Getenv("VAIJUNTO_LOGS_DIR")
	if strings.TrimSpace(diretorioLogs) == "" {
		diretorioLogs = filepath.Join("logs", empresaID)
	}

	configuracao := ConfiguracaoServidor{
		EmpresaID:       empresaID,
		EmpresaNome:     empresaNome,
		EnderecoTCP:     enderecoTCP,
		EnderecoREST:    enderecoREST,
		TimeoutREST:     timeoutREST,
		Pares:           pares,
		ArquivoUsuarios: valorAmbiente("VAIJUNTO_USUARIOS", filepath.Join(diretorioDados, "usuarios.json")),
		ArquivoEstado:   valorAmbiente("VAIJUNTO_ESTADO", filepath.Join(diretorioDados, "estado.json")),
		ArquivoLog:      valorAmbiente("VAIJUNTO_LOG", filepath.Join(diretorioLogs, "servidor.log")),
	}
	if err := configuracao.validar(); err != nil {
		return ConfiguracaoServidor{}, err
	}
	return configuracao, nil
}

func (c ConfiguracaoServidor) validar() error {
	if !empresaIDValido.MatchString(c.EmpresaID) {
		return errors.New("VAIJUNTO_EMPRESA_ID deve usar somente letras minúsculas, números e hífens")
	}
	if strings.TrimSpace(c.EmpresaNome) == "" {
		return errors.New("VAIJUNTO_EMPRESA_NOME não pode ficar vazio")
	}
	if strings.TrimSpace(c.EnderecoTCP) == "" {
		return errors.New("VAIJUNTO_ADDR não pode ficar vazio")
	}
	if strings.TrimSpace(c.EnderecoREST) == "" {
		return errors.New("VAIJUNTO_REST_ADDR não pode ficar vazio")
	}
	if c.TimeoutREST <= 0 {
		return errors.New("o timeout REST deve ser positivo")
	}
	if strings.TrimSpace(c.ArquivoUsuarios) == "" || strings.TrimSpace(c.ArquivoEstado) == "" || strings.TrimSpace(c.ArquivoLog) == "" {
		return errors.New("os caminhos de usuários, estado e log são obrigatórios")
	}
	return nil
}

// interpretarPares recebe itens no formato empresa-b=http://empresa-b:8080.
// A ordenação torna logs, testes e respostas independentes da ordem do ambiente.
func interpretarPares(valor, empresaLocal string) ([]EmpresaRemota, error) {
	valor = strings.TrimSpace(valor)
	if valor == "" {
		return []EmpresaRemota{}, nil
	}

	pares := make([]EmpresaRemota, 0)
	vistos := make(map[string]bool)
	for _, item := range strings.Split(valor, ",") {
		partes := strings.SplitN(strings.TrimSpace(item), "=", 2)
		if len(partes) != 2 {
			return nil, fmt.Errorf("par REST inválido %q; use empresa-id=http://host:porta", item)
		}
		id := strings.TrimSpace(partes[0])
		if !empresaIDValido.MatchString(id) {
			return nil, fmt.Errorf("ID de empresa remota inválido: %q", id)
		}
		if id == empresaLocal {
			return nil, fmt.Errorf("a empresa local %q não pode aparecer em VAIJUNTO_PARES", id)
		}
		if vistos[id] {
			return nil, fmt.Errorf("empresa remota repetida: %q", id)
		}

		urlBase := strings.TrimRight(strings.TrimSpace(partes[1]), "/")
		u, err := url.Parse(urlBase)
		if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" || u.User != nil {
			return nil, fmt.Errorf("URL REST inválida para %s: %q", id, urlBase)
		}
		if u.RawQuery != "" || u.Fragment != "" {
			return nil, fmt.Errorf("URL REST de %s não pode conter consulta ou fragmento", id)
		}

		vistos[id] = true
		pares = append(pares, EmpresaRemota{ID: id, URLBase: urlBase})
	}
	sort.Slice(pares, func(i, j int) bool { return pares[i].ID < pares[j].ID })
	return pares, nil
}

func valorAmbiente(chave, padrao string) string {
	valor := strings.TrimSpace(os.Getenv(chave))
	if valor == "" {
		return padrao
	}
	return valor
}
