package principal

import (
	"context"
	"fmt"

	"ecommerce/internal/evento"
)

// Servico reage aos eventos que chegam na fila do Principal: atualiza o status
// do pedido e, em estoque indisponível ou pagamento recusado, publica
// pedido.excluido.
type Servico struct {
	registro   *Registro
	publicador Publicador
	avisar     func(texto string)
}

// NovoServico monta o tratador de eventos.
func NovoServico(registro *Registro, publicador Publicador, avisar func(string)) *Servico {
	if avisar == nil {
		avisar = func(string) {}
	}
	return &Servico{registro: registro, publicador: publicador, avisar: avisar}
}

// Tratar é o ponto de entrada do consumidor: despacha pela routing key.
func (s *Servico) Tratar(ctx context.Context, env evento.Envelope) error {
	switch env.Tipo {
	case evento.PedidoEstoqueOK:
		return s.aoEstoqueOK(env)
	case evento.EstoqueIndisponivel:
		return s.aoEstoqueIndisponivel(ctx, env)
	case evento.PagamentoAprovado:
		return s.aoPagamentoAprovado(env)
	case evento.PagamentoRecusado:
		return s.aoPagamentoRecusado(ctx, env)
	case evento.PedidoEnviado:
		return s.aoPedidoEnviado(env)
	default:
		return nil
	}
}

func (s *Servico) aoEstoqueOK(env evento.Envelope) error {
	var dados evento.DadosPedidoEstoqueOK
	if err := env.DecodificarDados(&dados); err != nil {
		return err
	}

	s.mudarStatus(dados.PedidoID, StatusEstoqueReservado, "itens reservados, seguindo para o pagamento")
	return nil
}

// aoEstoqueIndisponivel cancela o pedido e publica pedido.excluido.
func (s *Servico) aoEstoqueIndisponivel(ctx context.Context, env evento.Envelope) error {
	var dados evento.DadosEstoqueIndisponivel
	if err := env.DecodificarDados(&dados); err != nil {
		return err
	}

	s.mudarStatus(dados.PedidoID, StatusSemEstoque, dados.Motivo)
	return s.excluir(ctx, dados.PedidoID, "estoque indisponível: "+dados.Motivo)
}

func (s *Servico) aoPagamentoAprovado(env evento.Envelope) error {
	var dados evento.DadosPagamentoAprovado
	if err := env.DecodificarDados(&dados); err != nil {
		return err
	}

	s.mudarStatus(dados.PedidoID, StatusPagamentoAprovado,
		fmt.Sprintf("transação %s, R$ %.2f", dados.TransacaoID, dados.Valor))
	return nil
}

// aoPagamentoRecusado cancela o pedido e publica pedido.excluido.
func (s *Servico) aoPagamentoRecusado(ctx context.Context, env evento.Envelope) error {
	var dados evento.DadosPagamentoRecusado
	if err := env.DecodificarDados(&dados); err != nil {
		return err
	}

	s.mudarStatus(dados.PedidoID, StatusPagamentoRecusado, dados.Motivo)
	return s.excluir(ctx, dados.PedidoID, "pagamento recusado: "+dados.Motivo)
}

func (s *Servico) aoPedidoEnviado(env evento.Envelope) error {
	var dados evento.DadosPedidoEnviado
	if err := env.DecodificarDados(&dados); err != nil {
		return err
	}

	s.mudarStatus(dados.PedidoID, StatusEnviado,
		fmt.Sprintf("nota %s, %s, rastreio %s", dados.NotaFiscal, dados.Transportadora, dados.CodigoRastreio))
	return nil
}

// mudarStatus atualiza o pedido e avisa o usuário. Pedido desconhecido só
// gera aviso, não erro.
func (s *Servico) mudarStatus(pedidoID string, status Status, detalhe string) {
	if _, existe := s.registro.Atualizar(pedidoID, status, detalhe); !existe {
		s.avisar(fmt.Sprintf("evento para o pedido %s, que não é desta sessão", pedidoID))
		return
	}

	texto := fmt.Sprintf("%s → %s", pedidoID, status)
	if detalhe != "" {
		texto += " (" + detalhe + ")"
	}
	s.avisar(texto)
}

// excluir publica pedido.excluido, mesmo para pedido que esta sessão não
// conhece.
func (s *Servico) excluir(ctx context.Context, pedidoID, motivo string) error {
	return s.publicador.PublicarECommerce(ctx, evento.PedidoExcluido, evento.DadosPedidoExcluido{
		PedidoID: pedidoID,
		Motivo:   motivo,
	})
}
