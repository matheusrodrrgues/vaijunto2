package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"vaijunto/comum"
)

type usuarioArquivo struct {
	Nome      string `json:"nome"`
	Usuario   string `json:"usuario"`
	SenhaHash string `json:"senha_hash"`
	Perfil    string `json:"perfil"`
}

// NovoEstadoComArquivo carrega as contas antes de abrir o servidor.
// As contas iniciais só são gravadas quando o arquivo ainda não existe.
func NovoEstadoComArquivo(caminho string) (*Estado, error) {
	return novoEstadoEmpresaComArquivo(empresaPadraoID, empresaPadraoNome, caminho)
}

func novoEstadoEmpresaComArquivo(empresaID, empresaNome, caminho string) (*Estado, error) {
	estado := NovoEstadoEmpresa(empresaID, empresaNome)
	estado.arquivoUsuarios = caminho

	conteudo, err := os.ReadFile(caminho)
	if errors.Is(err, os.ErrNotExist) {
		if err := estado.salvarUsuariosSemLock(); err != nil {
			return nil, err
		}
		return estado, nil
	}
	if err != nil {
		return nil, err
	}

	var registros []usuarioArquivo
	if err := json.Unmarshal(conteudo, &registros); err != nil {
		return nil, err
	}

	// O arquivo passa a ser a fonte das contas, sem misturar os usuários
	// carregados com as contas de demonstração criadas em NovoEstado.
	estado.usuarios = make(map[string]usuario)
	for _, registro := range registros {
		login := strings.ToLower(strings.TrimSpace(registro.Usuario))
		if login == "" || strings.TrimSpace(registro.Nome) == "" {
			return nil, errors.New("usuário inválido no arquivo de usuários")
		}
		if registro.Perfil != comum.PerfilMotorista && registro.Perfil != comum.PerfilPassageiro {
			return nil, errors.New("perfil inválido no arquivo de usuários")
		}
		if _, repetido := estado.usuarios[login]; repetido {
			return nil, errors.New("usuário repetido no arquivo de usuários")
		}
		hashBytes, err := hex.DecodeString(registro.SenhaHash)
		if err != nil || len(hashBytes) != sha256.Size {
			return nil, errors.New("senha_hash inválida no arquivo de usuários")
		}
		var hash [sha256.Size]byte
		copy(hash[:], hashBytes)
		estado.usuarios[login] = usuario{
			Nome: registro.Nome, Login: login,
			SenhaHash: hash, Perfil: registro.Perfil,
		}
	}
	return estado, nil
}

// Esta função deve ser chamada com o mutex do Estado travado, exceto durante
// a criação do servidor, quando nenhuma conexão ainda foi aceita.
func (e *Estado) salvarUsuariosSemLock() error {
	if e.arquivoUsuarios == "" {
		return nil
	}

	registros := make([]usuarioArquivo, 0, len(e.usuarios))
	for _, u := range e.usuarios {
		registros = append(registros, usuarioArquivo{
			Nome: u.Nome, Usuario: u.Login,
			SenhaHash: hex.EncodeToString(u.SenhaHash[:]), Perfil: u.Perfil,
		})
	}
	sort.Slice(registros, func(i, j int) bool {
		return registros[i].Usuario < registros[j].Usuario
	})
	conteudo, err := json.MarshalIndent(registros, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(e.arquivoUsuarios), 0755); err != nil {
		return err
	}
	return os.WriteFile(e.arquivoUsuarios, append(conteudo, '\n'), 0600)
}
