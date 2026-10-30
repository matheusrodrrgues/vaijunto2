package main

import (
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"strings"
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
	ArquivoUsuarios string
	ArquivoEstado   string
	ArquivoLog      string
}

func carregarConfiguracaoServidor() (ConfiguracaoServidor, error) {
	empresaID := valorAmbiente("VAIJUNTO_EMPRESA_ID", empresaPadraoID)
	empresaNome := valorAmbiente("VAIJUNTO_EMPRESA_NOME", empresaPadraoNome)
	enderecoTCP := valorAmbiente("VAIJUNTO_ADDR", ":9000")

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
	if strings.TrimSpace(c.ArquivoUsuarios) == "" || strings.TrimSpace(c.ArquivoEstado) == "" || strings.TrimSpace(c.ArquivoLog) == "" {
		return errors.New("os caminhos de usuários, estado e log são obrigatórios")
	}
	return nil
}

func valorAmbiente(chave, padrao string) string {
	valor := strings.TrimSpace(os.Getenv(chave))
	if valor == "" {
		return padrao
	}
	return valor
}
