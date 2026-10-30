package main

import (
	"errors"
	"fmt"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"
)

type empresaAlvo struct {
	ID      string
	TCP     string
	URLREST string
}

type configuracao struct {
	empresasTCP        string
	empresasREST       string
	usuario            string
	senha              string
	origem             string
	destino            string
	data               string
	requisicoes        int
	concorrencia       int
	duracao            time.Duration
	capacidadeEsperada int
	esperaRecuperacao  time.Duration
	servicoFalha       string
	falhaApos          time.Duration
	falhaDuracao       time.Duration
	diretorioCompose   string
	saidaJSON          string
	saidaCSV           string
}

func (c configuracao) validar() error {
	if strings.TrimSpace(c.usuario) == "" || strings.TrimSpace(c.senha) == "" {
		return errors.New("usuário e senha são obrigatórios")
	}
	if strings.TrimSpace(c.origem) == "" || strings.TrimSpace(c.destino) == "" || c.origem == c.destino {
		return errors.New("origem e destino precisam ser diferentes")
	}
	if _, err := time.Parse("2006-01-02", c.data); err != nil {
		return errors.New("data deve estar no formato AAAA-MM-DD")
	}
	if c.requisicoes < 1 && c.duracao <= 0 {
		return errors.New("requisicoes deve ser positiva quando duracao não for usada")
	}
	if c.concorrencia < 1 || c.capacidadeEsperada < 1 || c.esperaRecuperacao < 0 {
		return errors.New("concorrência, capacidade e espera precisam ser válidas")
	}
	if c.servicoFalha != "" && (c.falhaApos < 0 || c.falhaDuracao <= 0) {
		return errors.New("os tempos da injeção de falha precisam ser positivos")
	}
	if strings.TrimSpace(c.saidaJSON) == "" || strings.TrimSpace(c.saidaCSV) == "" {
		return errors.New("os caminhos dos relatórios são obrigatórios")
	}
	_, err := c.alvos()
	return err
}

func (c configuracao) alvos() ([]empresaAlvo, error) {
	tcp, err := interpretarMapa(c.empresasTCP, false)
	if err != nil {
		return nil, fmt.Errorf("empresas: %w", err)
	}
	rest, err := interpretarMapa(c.empresasREST, true)
	if err != nil {
		return nil, fmt.Errorf("rest: %w", err)
	}
	ids := make([]string, 0, len(tcp))
	for id := range tcp {
		if _, ok := rest[id]; !ok {
			return nil, fmt.Errorf("empresa %s não possui URL REST", id)
		}
		ids = append(ids, id)
	}
	if len(ids) == 0 {
		return nil, errors.New("nenhuma empresa configurada")
	}
	sort.Strings(ids)
	alvos := make([]empresaAlvo, 0, len(ids))
	for _, id := range ids {
		alvos = append(alvos, empresaAlvo{ID: id, TCP: tcp[id], URLREST: rest[id]})
	}
	return alvos, nil
}

func interpretarMapa(valor string, validarURL bool) (map[string]string, error) {
	resultado := make(map[string]string)
	for _, item := range strings.Split(valor, ",") {
		partes := strings.SplitN(strings.TrimSpace(item), "=", 2)
		if len(partes) != 2 || strings.TrimSpace(partes[0]) == "" || strings.TrimSpace(partes[1]) == "" {
			return nil, fmt.Errorf("item inválido %q; use id=endereço", item)
		}
		id, endereco := strings.TrimSpace(partes[0]), strings.TrimRight(strings.TrimSpace(partes[1]), "/")
		if _, repetido := resultado[id]; repetido {
			return nil, fmt.Errorf("empresa repetida: %s", id)
		}
		if validarURL {
			u, err := url.Parse(endereco)
			if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
				return nil, fmt.Errorf("URL inválida para %s", id)
			}
		} else {
			partesEndereco := strings.Split(endereco, ":")
			if len(partesEndereco) < 2 {
				return nil, fmt.Errorf("endereço TCP inválido para %s", id)
			}
			if _, err := strconv.Atoi(partesEndereco[len(partesEndereco)-1]); err != nil {
				return nil, fmt.Errorf("porta TCP inválida para %s", id)
			}
		}
		resultado[id] = endereco
	}
	return resultado, nil
}
