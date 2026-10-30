package main

import (
	"encoding/json"
	"errors"
	"io"
	"net"

	"vaijunto/comum"
)

// ExecutarServidor recebe as conexões e deixa cada cliente em seu atendimento.
// Assim, um usuário parado no menu não segura a entrada dos demais.
func ExecutarServidor(listener net.Listener, estado *Estado) error {
	for {
		conexao, err := listener.Accept()
		if err != nil {
			if errors.Is(err, net.ErrClosed) {
				return nil
			}
			return err
		}
		go atenderCliente(conexao, estado)
	}
}

func atenderCliente(conexao net.Conn, estado *Estado) {
	ip := conexao.RemoteAddr().String()
	registrar("CONEXAO_ABERTA ip=%s", ip)
	defer func() {
		registrar("CONEXAO_FECHADA ip=%s", ip)
		conexao.Close()
	}()
	// TCP entrega bytes, não mensagens prontas. O decoder junta o que chegar
	// até conseguir ler um JSON completo, mesmo que venha em partes.
	decoder := json.NewDecoder(conexao)
	encoder := json.NewEncoder(conexao)

	for {
		var pedido comum.Pedido
		// Se o fluxo não pode mais ser lido, encerra só esta conexão.
		// Um JSON quebrado não deve derrubar o atendimento dos outros clientes.
		if err := decoder.Decode(&pedido); err != nil {
			if !errors.Is(err, io.EOF) {
				registrar("JSON_INVALIDO ip=%s erro=%q", ip, err.Error())
				_ = encoder.Encode(novaResposta("", false, "mensagem JSON inválida", nil))
			}
			return
		}

		resposta := processarPedido(estado, pedido)
		registrarTecnico("REQUISICAO ip=%s id=%s acao=%s sucesso=%t mensagem=%q",
			ip, pedido.ID, pedido.Acao, resposta.Sucesso, resposta.Mensagem)
		if err := encoder.Encode(resposta); err != nil {
			return
		}
	}
}

// processarPedido traduz a ação da mensagem para uma operação do Estado.
// As regras de vagas, usuários e cancelamentos ficam nas funções chamadas aqui.
func processarPedido(estado *Estado, pedido comum.Pedido) comum.Resposta {
	if pedido.ID == "" || pedido.Acao == "" {
		return novaResposta(pedido.ID, false, "id e acao são obrigatórios", nil)
	}

	switch pedido.Acao {
	case comum.AcaoCadastro:
		var dados comum.CadastroDados
		if err := lerPedido(pedido, &dados); err != nil {
			return respostaErro(pedido.ID, err)
		}
		err := estado.Cadastrar(dados)
		return respostaResultado(pedido.ID, "conta criada com sucesso", nil, err)

	case comum.AcaoLogin:
		var dados comum.LoginDados
		if err := lerPedido(pedido, &dados); err != nil {
			return respostaErro(pedido.ID, err)
		}
		login, err := estado.Login(dados.Usuario, dados.Senha)
		return respostaResultado(pedido.ID, "login realizado", login, err)

	case comum.AcaoPublicarCarona:
		var dados comum.PublicarCaronaDados
		if err := lerPedido(pedido, &dados); err != nil {
			return respostaErro(pedido.ID, err)
		}
		carona, err := estado.PublicarCarona(pedido.Token, dados)
		return respostaResultado(pedido.ID, "carona publicada", carona, err)

	case comum.AcaoListarCaronas:
		caronas, err := estado.ListarCaronas(pedido.Token)
		return respostaResultado(pedido.ID, "caronas encontradas", caronas, err)

	case comum.AcaoListarPassageiros:
		var dados comum.IDDados
		if err := lerPedido(pedido, &dados); err != nil {
			return respostaErro(pedido.ID, err)
		}
		reservas, err := estado.ListarPassageiros(pedido.Token, dados.ID)
		return respostaResultado(pedido.ID, "passageiros encontrados", reservas, err)

	case comum.AcaoCancelarCarona:
		var dados comum.IDDados
		if err := lerPedido(pedido, &dados); err != nil {
			return respostaErro(pedido.ID, err)
		}
		err := estado.CancelarCarona(pedido.Token, dados.ID)
		return respostaResultado(pedido.ID, "carona cancelada", nil, err)

	case comum.AcaoBuscarItinerarios:
		var dados comum.BuscarItinerariosDados
		if err := lerPedido(pedido, &dados); err != nil {
			return respostaErro(pedido.ID, err)
		}
		itinerarios, err := estado.BuscarItinerarios(pedido.Token, dados)
		return respostaResultado(pedido.ID, "busca concluída", itinerarios, err)

	case comum.AcaoConfirmarReserva:
		var dados comum.IDDados
		if err := lerPedido(pedido, &dados); err != nil {
			return respostaErro(pedido.ID, err)
		}
		reserva, err := estado.ConfirmarReserva(pedido.Token, dados.ID)
		return respostaResultado(pedido.ID, "reserva confirmada", reserva, err)

	case comum.AcaoListarReservas:
		reservas, err := estado.ListarReservas(pedido.Token)
		return respostaResultado(pedido.ID, "reservas encontradas", reservas, err)

	case comum.AcaoCancelarReserva:
		var dados comum.IDDados
		if err := lerPedido(pedido, &dados); err != nil {
			return respostaErro(pedido.ID, err)
		}
		err := estado.CancelarReserva(pedido.Token, dados.ID)
		return respostaResultado(pedido.ID, "reserva cancelada", nil, err)

	default:
		return novaResposta(pedido.ID, false, "operação desconhecida", nil)
	}
}

func lerPedido(pedido comum.Pedido, destino any) error {
	if len(pedido.Dados) == 0 {
		return errors.New("os dados da operação não foram informados")
	}
	if err := json.Unmarshal(pedido.Dados, destino); err != nil {
		return errors.New("os dados da operação são inválidos")
	}
	return nil
}

func respostaResultado(id, mensagem string, dados any, err error) comum.Resposta {
	if err != nil {
		return respostaErro(id, err)
	}
	return novaResposta(id, true, mensagem, dados)
}

func respostaErro(id string, err error) comum.Resposta {
	return novaResposta(id, false, err.Error(), nil)
}

func novaResposta(id string, sucesso bool, mensagem string, dados any) comum.Resposta {
	resposta := comum.Resposta{ID: id, Sucesso: sucesso, Mensagem: mensagem}
	if dados != nil {
		resposta.Dados, _ = json.Marshal(dados)
	}
	return resposta
}
