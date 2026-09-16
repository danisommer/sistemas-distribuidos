package estoque

import (
	"context"
	"testing"

	"ecommerce/internal/evento"
)

type publicadorFalso struct {
	publicados []publicado
}

type publicado struct {
	chave string
	dados any
}

func (p *publicadorFalso) PublicarECommerce(_ context.Context, chave string, dados any) error {
	p.publicados = append(p.publicados, publicado{chave: chave, dados: dados})
	return nil
}

func (p *publicadorFalso) ultimo(t *testing.T) publicado {
	t.Helper()
	if len(p.publicados) == 0 {
		t.Fatal("o serviço não publicou nada")
	}
	return p.publicados[len(p.publicados)-1]
}

func montar(t *testing.T) (*Servico, *publicadorFalso) {
	t.Helper()
	pub := &publicadorFalso{}
	return NovoServico(pub), pub
}

func pedidoCriado(t *testing.T, pedidoID string, itens ...evento.ItemPedido) evento.Envelope {
	t.Helper()
	env, err := evento.NovoEnvelope(evento.PedidoCriado, evento.ServicoPrincipal, evento.DadosPedidoCriado{
		PedidoID: pedidoID,
		Cliente:  "cliente-teste",
		Itens:    itens,
	})
	if err != nil {
		t.Fatalf("montando pedido.criado: %v", err)
	}
	return env
}

func pedidoExcluido(t *testing.T, pedidoID, motivo string) evento.Envelope {
	t.Helper()
	env, err := evento.NovoEnvelope(evento.PedidoExcluido, evento.ServicoPrincipal, evento.DadosPedidoExcluido{
		PedidoID: pedidoID,
		Motivo:   motivo,
	})
	if err != nil {
		t.Fatalf("montando pedido.excluido: %v", err)
	}
	return env
}

func item(produtoID string, quantidade int) evento.ItemPedido {
	return evento.ItemPedido{ProdutoID: produtoID, Quantidade: quantidade}
}

func tratar(t *testing.T, s *Servico, env evento.Envelope) {
	t.Helper()
	if err := s.Tratar(context.Background(), env); err != nil {
		t.Fatalf("tratando %s: %v", env.Tipo, err)
	}
}

func exigirSaldo(t *testing.T, s *Servico, produtoID string, esperado int) {
	t.Helper()
	if s.saldo[produtoID] != esperado {
		t.Fatalf("saldo de %s: esperava %d, veio %d", produtoID, esperado, s.saldo[produtoID])
	}
}

func TestPedidoDisponivelBaixaEstoqueEPublicaEstoqueOK(t *testing.T) {
	s, pub := montar(t)

	tratar(t, s, pedidoCriado(t, "PED-1", item("P001", 2), item("P004", 3)))

	if chave := pub.ultimo(t).chave; chave != evento.PedidoEstoqueOK {
		t.Fatalf("esperava %s, veio %s", evento.PedidoEstoqueOK, chave)
	}
	exigirSaldo(t, s, "P001", 8)
	exigirSaldo(t, s, "P004", 12)
}

func TestPedidoAcimaDoEstoquePublicaIndisponivelESemBaixa(t *testing.T) {
	s, pub := montar(t)

	tratar(t, s, pedidoCriado(t, "PED-2", item("P006", 99)))

	ultimo := pub.ultimo(t)
	if ultimo.chave != evento.EstoqueIndisponivel {
		t.Fatalf("esperava %s, veio %s", evento.EstoqueIndisponivel, ultimo.chave)
	}

	dados, ok := ultimo.dados.(evento.DadosEstoqueIndisponivel)
	if !ok {
		t.Fatalf("payload inesperado: %T", ultimo.dados)
	}
	if dados.ProdutoID != "P006" || dados.Solicitado != 99 || dados.Disponivel != 8 {
		t.Fatalf("payload não descreve a falta: %+v", dados)
	}
	exigirSaldo(t, s, "P006", 8)
}

func TestPedidoParcialmenteDisponivelNaoBaixaNada(t *testing.T) {
	s, _ := montar(t)

	tratar(t, s, pedidoCriado(t, "PED-3", item("P001", 1), item("P006", 99)))

	exigirSaldo(t, s, "P001", 10)
	exigirSaldo(t, s, "P006", 8)
}

func TestMesmoProdutoEmDuasLinhasSomaAntesDeConferir(t *testing.T) {
	s, pub := montar(t)

	tratar(t, s, pedidoCriado(t, "PED-4", item("P001", 6), item("P001", 6)))

	if chave := pub.ultimo(t).chave; chave != evento.EstoqueIndisponivel {
		t.Fatalf("12 unidades de um estoque de 10 deveriam faltar, veio %s", chave)
	}
	exigirSaldo(t, s, "P001", 10)
}

func TestProdutoForaDoCatalogoEIndisponivel(t *testing.T) {
	s, pub := montar(t)

	tratar(t, s, pedidoCriado(t, "PED-5", item("P999", 1)))

	if chave := pub.ultimo(t).chave; chave != evento.EstoqueIndisponivel {
		t.Fatalf("esperava %s, veio %s", evento.EstoqueIndisponivel, chave)
	}
}

func TestQuantidadeInvalidaEIndisponivel(t *testing.T) {
	s, pub := montar(t)

	tratar(t, s, pedidoCriado(t, "PED-6", item("P001", 0)))

	if chave := pub.ultimo(t).chave; chave != evento.EstoqueIndisponivel {
		t.Fatalf("esperava %s, veio %s", evento.EstoqueIndisponivel, chave)
	}
	exigirSaldo(t, s, "P001", 10)
}

func TestPedidoSemItensEIndisponivel(t *testing.T) {
	s, pub := montar(t)

	tratar(t, s, pedidoCriado(t, "PED-7"))

	if chave := pub.ultimo(t).chave; chave != evento.EstoqueIndisponivel {
		t.Fatalf("esperava %s, veio %s", evento.EstoqueIndisponivel, chave)
	}
}

func TestReentregaDoMesmoPedidoNaoBaixaDuasVezes(t *testing.T) {
	s, _ := montar(t)

	env := pedidoCriado(t, "PED-8", item("P001", 3))
	tratar(t, s, env)
	tratar(t, s, env)

	exigirSaldo(t, s, "P001", 7)
}

func TestPedidoExcluidoDevolveAoEstoque(t *testing.T) {
	s, _ := montar(t)

	tratar(t, s, pedidoCriado(t, "PED-9", item("P001", 4), item("P007", 10)))
	exigirSaldo(t, s, "P001", 6)

	tratar(t, s, pedidoExcluido(t, "PED-9", "pagamento recusado"))

	exigirSaldo(t, s, "P001", 10)
	exigirSaldo(t, s, "P007", 100)
}

func TestPedidoExcluidoSemReservaNaoCriaEstoque(t *testing.T) {
	s, _ := montar(t)

	tratar(t, s, pedidoCriado(t, "PED-10", item("P006", 99)))
	tratar(t, s, pedidoExcluido(t, "PED-10", "estoque indisponível"))

	exigirSaldo(t, s, "P006", 8)
}

func TestExclusaoRepetidaDevolveUmaVezSo(t *testing.T) {
	s, _ := montar(t)

	tratar(t, s, pedidoCriado(t, "PED-11", item("P002", 5)))
	exclusao := pedidoExcluido(t, "PED-11", "cancelado")
	tratar(t, s, exclusao)
	tratar(t, s, exclusao)

	exigirSaldo(t, s, "P002", 50)
}

func TestEventoDeOutroAssuntoEIgnorado(t *testing.T) {
	s, pub := montar(t)

	env, err := evento.NovoEnvelope(evento.PagamentoAprovado, evento.ServicoPagamento,
		evento.DadosPagamentoAprovado{PedidoID: "PED-12"})
	if err != nil {
		t.Fatalf("montando envelope: %v", err)
	}
	tratar(t, s, env)

	if len(pub.publicados) != 0 {
		t.Fatalf("o Estoque publicou %d evento(s) para algo que não é assunto dele", len(pub.publicados))
	}
}
