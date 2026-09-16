package pagamento

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

// montar cria o serviço com a taxa de aprovação fixada e sem o atraso
// simulado.
func montar(t *testing.T, taxa string) (*Servico, *publicadorFalso) {
	t.Helper()
	t.Setenv("TAXA_APROVACAO", taxa)

	pub := &publicadorFalso{}
	s := NovoServico(pub)
	s.atrasoMinimo = 0
	s.atrasoMaximo = 0
	return s, pub
}

func estoqueOK(t *testing.T, pedidoID string, total float64) evento.Envelope {
	t.Helper()
	env, err := evento.NovoEnvelope(evento.PedidoEstoqueOK, evento.ServicoEstoque, evento.DadosPedidoEstoqueOK{
		PedidoID: pedidoID,
		Itens:    []evento.ItemPedido{{ProdutoID: "P001", Nome: "Notebook", Quantidade: 1, PrecoUnit: total}},
		Total:    total,
	})
	if err != nil {
		t.Fatalf("montando pedido.estoque_ok: %v", err)
	}
	return env
}

func tratar(t *testing.T, s *Servico, env evento.Envelope) {
	t.Helper()
	if err := s.Tratar(context.Background(), env); err != nil {
		t.Fatalf("tratando %s: %v", env.Tipo, err)
	}
}

func TestTaxaUmAprovaSempre(t *testing.T) {
	s, pub := montar(t, "1")

	for i := 0; i < 20; i++ {
		tratar(t, s, estoqueOK(t, "PED-"+string(rune('a'+i)), 100))
	}

	for _, p := range pub.publicados {
		if p.chave != evento.PagamentoAprovado {
			t.Fatalf("com taxa 1.0 todo pagamento deveria ser aprovado, veio %s", p.chave)
		}
	}
	if len(pub.publicados) != 20 {
		t.Fatalf("esperava 20 eventos, vieram %d", len(pub.publicados))
	}
}

func TestTaxaZeroRecusaSempre(t *testing.T) {
	s, pub := montar(t, "0")

	for i := 0; i < 20; i++ {
		tratar(t, s, estoqueOK(t, "PED-"+string(rune('a'+i)), 100))
	}

	for _, p := range pub.publicados {
		if p.chave != evento.PagamentoRecusado {
			t.Fatalf("com taxa 0.0 todo pagamento deveria ser recusado, veio %s", p.chave)
		}
	}
}

func TestPagamentoAprovadoLevaValorEItens(t *testing.T) {
	s, pub := montar(t, "1")

	tratar(t, s, estoqueOK(t, "PED-1", 4500))

	dados, ok := pub.publicados[0].dados.(evento.DadosPagamentoAprovado)
	if !ok {
		t.Fatalf("payload inesperado: %T", pub.publicados[0].dados)
	}
	if dados.PedidoID != "PED-1" {
		t.Fatalf("pedido errado no payload: %q", dados.PedidoID)
	}
	if dados.Valor != 4500 {
		t.Fatalf("valor errado no payload: %v", dados.Valor)
	}
	if dados.TransacaoID == "" {
		t.Fatal("pagamento aprovado sem identificador de transação")
	}
	if len(dados.Itens) != 1 {
		t.Fatalf("esperava 1 item repassado à Entrega, vieram %d", len(dados.Itens))
	}
}

func TestPagamentoRecusadoLevaMotivo(t *testing.T) {
	s, pub := montar(t, "0")

	tratar(t, s, estoqueOK(t, "PED-1", 4500))

	dados, ok := pub.publicados[0].dados.(evento.DadosPagamentoRecusado)
	if !ok {
		t.Fatalf("payload inesperado: %T", pub.publicados[0].dados)
	}
	if dados.Motivo == "" {
		t.Fatal("pagamento recusado sem motivo")
	}
}

func TestReentregaNaoCobraDuasVezes(t *testing.T) {
	s, pub := montar(t, "1")

	env := estoqueOK(t, "PED-1", 4500)
	tratar(t, s, env)
	tratar(t, s, env)

	if len(pub.publicados) != 1 {
		t.Fatalf("esperava 1 cobrança, vieram %d", len(pub.publicados))
	}
}

func TestEventoDeOutroAssuntoEIgnorado(t *testing.T) {
	s, pub := montar(t, "1")

	env, err := evento.NovoEnvelope(evento.PedidoCriado, evento.ServicoPrincipal,
		evento.DadosPedidoCriado{PedidoID: "PED-1"})
	if err != nil {
		t.Fatalf("montando envelope: %v", err)
	}
	tratar(t, s, env)

	if len(pub.publicados) != 0 {
		t.Fatalf("o Pagamento publicou %d evento(s) para algo que não é assunto dele", len(pub.publicados))
	}
}

func TestTaxaInvalidaCaiNoPadrao(t *testing.T) {
	s, _ := montar(t, "nao-e-numero")

	if s.taxaAprovacao != taxaAprovacaoPadrao {
		t.Fatalf("esperava o padrão %.2f, veio %.2f", taxaAprovacaoPadrao, s.taxaAprovacao)
	}
}
