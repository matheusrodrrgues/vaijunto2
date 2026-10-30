package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"vaijunto/comum"
)

func TestEmpresasMantemUsuariosCaronasEReservasIsolados(t *testing.T) {
	raiz := t.TempDir()
	empresaA := criarEmpresaTeste(t, raiz, "empresa-a", "Empresa A")
	empresaB := criarEmpresaTeste(t, raiz, "empresa-b", "Empresa B")
	empresaC := criarEmpresaTeste(t, raiz, "empresa-c", "Empresa C")

	if err := empresaA.Cadastrar(comum.CadastroDados{
		Nome: "Passageiro A", Usuario: "somente-a", Senha: "1234", Perfil: comum.PerfilPassageiro,
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := empresaB.Login("somente-a", "1234"); err == nil {
		t.Fatal("usuário cadastrado na empresa A apareceu na empresa B")
	}

	publicarCaronaEmpresaTeste(t, empresaA, []string{"Salvador", "Feira de Santana"})
	publicarCaronaEmpresaTeste(t, empresaB, []string{"Feira de Santana", "Vitória da Conquista"})

	if len(empresaA.caronas) != 1 || len(empresaB.caronas) != 1 || len(empresaC.caronas) != 0 {
		t.Fatalf("caronas misturadas: A=%d B=%d C=%d", len(empresaA.caronas), len(empresaB.caronas), len(empresaC.caronas))
	}
	for _, carona := range empresaA.caronas {
		if carona.EmpresaID != "empresa-a" {
			t.Fatalf("carona da empresa A identificada como %q", carona.EmpresaID)
		}
	}
	for _, carona := range empresaB.caronas {
		if carona.EmpresaID != "empresa-b" {
			t.Fatalf("carona da empresa B identificada como %q", carona.EmpresaID)
		}
	}

	validarArquivoPertenceEmpresa(t, filepath.Join(raiz, "empresa-a", "estado.json"), "empresa-a")
	validarArquivoPertenceEmpresa(t, filepath.Join(raiz, "empresa-b", "estado.json"), "empresa-b")
	validarArquivoPertenceEmpresa(t, filepath.Join(raiz, "empresa-c", "estado.json"), "empresa-c")

	if _, err := NovoEstadoEmpresaComArquivos(
		"empresa-b", "Empresa B",
		filepath.Join(raiz, "empresa-b", "usuarios.json"),
		filepath.Join(raiz, "empresa-a", "estado.json"),
	); err == nil {
		t.Fatal("servidor aceitou arquivo de estado pertencente a outra empresa")
	}
}

func criarEmpresaTeste(t *testing.T, raiz, id, nome string) *Estado {
	t.Helper()
	pasta := filepath.Join(raiz, id)
	estado, err := NovoEstadoEmpresaComArquivos(
		id, nome,
		filepath.Join(pasta, "usuarios.json"),
		filepath.Join(pasta, "estado.json"),
	)
	if err != nil {
		t.Fatal(err)
	}
	return estado
}

func publicarCaronaEmpresaTeste(t *testing.T, estado *Estado, rota []string) {
	t.Helper()
	motorista, err := estado.Login("motorista", "1234")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := estado.PublicarCarona(motorista.Token, comum.PublicarCaronaDados{
		Rota: rota, Data: "2026-11-10", Horario: "08:00", Assentos: 2,
		PrecosCentavos: []int{3000}, DuracoesMin: []int{60},
	}); err != nil {
		t.Fatal(err)
	}
}

func validarArquivoPertenceEmpresa(t *testing.T, caminho, empresaID string) {
	t.Helper()
	conteudo, err := os.ReadFile(caminho)
	if err != nil {
		t.Fatal(err)
	}
	var salvo estadoArquivo
	if err := json.Unmarshal(conteudo, &salvo); err != nil {
		t.Fatal(err)
	}
	if salvo.EmpresaID != empresaID {
		t.Fatalf("arquivo %s pertence a %q, esperado %q", caminho, salvo.EmpresaID, empresaID)
	}
}
