package main

import (
	"fmt"
	"log"
	"net"
	"os"
)

func main() {
	arquivoLog := os.Getenv("VAIJUNTO_LOG")
	if arquivoLog == "" {
		arquivoLog = "logs/servidor.log"
	}
	logAberto, err := configurarRegistro(arquivoLog)
	if err != nil {
		log.Fatal("Não foi possível abrir o arquivo de log: ", err)
	}
	defer logAberto.Close()

	arquivoUsuarios := os.Getenv("VAIJUNTO_USUARIOS")
	if arquivoUsuarios == "" {
		arquivoUsuarios = "dados/usuarios.json"
	}
	arquivoEstado := os.Getenv("VAIJUNTO_ESTADO")
	if arquivoEstado == "" {
		arquivoEstado = "dados/estado.json"
	}
	estado, err := NovoEstadoComArquivos(arquivoUsuarios, arquivoEstado)
	if err != nil {
		log.Fatal("Não foi possível carregar os dados: ", err)
	}

	endereco := os.Getenv("VAIJUNTO_ADDR")
	if endereco == "" {
		endereco = ":9000"
	}

	listener, err := net.Listen("tcp", endereco)
	if err != nil {
		log.Fatal("Não foi possível iniciar o servidor: ", err)
	}
	defer listener.Close()

	_, porta, _ := net.SplitHostPort(listener.Addr().String())
	fmt.Println()
	fmt.Println("====================================")
	fmt.Println("       SERVIDOR VAIJUNTO")
	fmt.Println("====================================")
	fmt.Println("Status   : ONLINE")
	fmt.Println("Porta TCP:", porta)
	fmt.Println("Usuários :", arquivoUsuarios)
	fmt.Println("Estado   :", arquivoEstado)
	fmt.Println("Logs     :", arquivoLog)
	fmt.Println("------------------------------------")
	fmt.Println("Aguardando conexões...")
	fmt.Println()
	registrar("SERVIDOR_INICIADO porta=%s", porta)

	if err := ExecutarServidor(listener, estado); err != nil {
		log.Fatal(err)
	}
}
