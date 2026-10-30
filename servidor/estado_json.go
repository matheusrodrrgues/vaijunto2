package main

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sort"

	"vaijunto/comum"
)

// estadoArquivo guarda o que precisa voltar depois de reiniciar o servidor.
// Caronas e reservas ficam juntas para não salvar uma sem as vagas da outra.
// Login e buscas ficam de fora: ao voltar, o cliente entra e pesquisa novamente.
type estadoArquivo struct {
	ProximaCarona  int             `json:"proxima_carona"`
	ProximaReserva int             `json:"proxima_reserva"`
	Caronas        []comum.Carona  `json:"caronas"`
	Reservas       []comum.Reserva `json:"reservas"`
}

func NovoEstadoComArquivos(arquivoUsuarios, arquivoEstado string) (*Estado, error) {
	estado, err := NovoEstadoComArquivo(arquivoUsuarios)
	if err != nil {
		return nil, err
	}
	estado.arquivoEstado = arquivoEstado
	if err := estado.carregarEstado(); err != nil {
		return nil, err
	}
	return estado, nil
}

func (e *Estado) carregarEstado() error {
	if e.arquivoEstado == "" {
		return nil
	}
	conteudo, err := os.ReadFile(e.arquivoEstado)
	// Na primeira execução o arquivo ainda não existe. Um arquivo existente
	// com JSON inválido, por outro lado, gera erro em vez de ser apagado.
	if errors.Is(err, os.ErrNotExist) {
		return e.salvarEstadoSemLock()
	}
	if err != nil {
		return err
	}

	var salvo estadoArquivo
	if err := json.Unmarshal(conteudo, &salvo); err != nil {
		return err
	}

	e.caronas = make(map[string]*comum.Carona)
	for i := range salvo.Caronas {
		carona := salvo.Caronas[i]
		e.caronas[carona.ID] = &carona
	}
	e.reservas = make(map[string]*comum.Reserva)
	for i := range salvo.Reservas {
		reserva := salvo.Reservas[i]
		e.reservas[reserva.ID] = &reserva
	}
	e.proximaCarona = salvo.ProximaCarona
	e.proximaReserva = salvo.ProximaReserva
	return nil
}

// salvarEstadoSemLock usa a trava de quem chamou; não tenta travá-la de novo.
// Na inicialização também pode ser usada, antes de aceitar qualquer cliente.
func (e *Estado) salvarEstadoSemLock() error {
	if e.arquivoEstado == "" {
		return nil
	}

	salvo := estadoArquivo{
		ProximaCarona: e.proximaCarona, ProximaReserva: e.proximaReserva,
		Caronas:  make([]comum.Carona, 0, len(e.caronas)),
		Reservas: make([]comum.Reserva, 0, len(e.reservas)),
	}
	for _, carona := range e.caronas {
		salvo.Caronas = append(salvo.Caronas, copiarCarona(carona))
	}
	for _, reserva := range e.reservas {
		salvo.Reservas = append(salvo.Reservas, *reserva)
	}
	sort.Slice(salvo.Caronas, func(i, j int) bool { return salvo.Caronas[i].ID < salvo.Caronas[j].ID })
	sort.Slice(salvo.Reservas, func(i, j int) bool { return salvo.Reservas[i].ID < salvo.Reservas[j].ID })

	conteudo, err := json.MarshalIndent(salvo, "", "  ")
	if err != nil {
		return err
	}
	return escreverJSONSeguro(e.arquivoEstado, append(conteudo, '\n'))
}

// escreverJSONSeguro termina a escrita temporária antes de trocar o arquivo.
// Isso reduz o risco de JSON incompleto, mas a substituição abaixo ainda tem
// uma janela de perda se o processo parar entre remover e renomear.
func escreverJSONSeguro(caminho string, conteudo []byte) error {
	pasta := filepath.Dir(caminho)
	if err := os.MkdirAll(pasta, 0755); err != nil {
		return err
	}
	temporario, err := os.CreateTemp(pasta, ".vaijunto-*.tmp")
	if err != nil {
		return err
	}
	nomeTemporario := temporario.Name()
	defer os.Remove(nomeTemporario)

	if err := temporario.Chmod(0600); err != nil {
		temporario.Close()
		return err
	}
	if _, err := temporario.Write(conteudo); err != nil {
		temporario.Close()
		return err
	}
	if err := temporario.Sync(); err != nil {
		temporario.Close()
		return err
	}
	if err := temporario.Close(); err != nil {
		return err
	}

	// Esta implementação remove o anterior antes de renomear.
	// Se houver uma interrupção aqui, o arquivo de estado pode ficar ausente.
	if err := os.Remove(caminho); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return os.Rename(nomeTemporario, caminho)
}
