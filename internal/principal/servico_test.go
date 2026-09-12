package principal

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

func (p *publicadorFalso) chaves() []string {
	var lista []string
	for _, pub := range p.publicados {
		lista = append(lista, pub.chave)
	}
	return lista
}

func montarServico(t *testing.T) (*Servico, *Registro, *publicadorFalso) {
	t.Helper()
	registro := NovoRegistro()
	pub := &publicadorFalso{}
	return NovoServico(registro, pub, nil), registro, pub
}

// pedidoNoRegistro cria um pedido e devolve o ID dele.
func pedidoNoRegistro(t *testing.T, registro *Registro) string {
	t.Helper()
	pedido := registro.Criar("Daniel", []evento.ItemPedido{
		{ProdutoID: "P001", Nome: "Notebook Gamer 16GB", Quantidade: 1, PrecoUnit: 4500},
	})
	return pedido.ID
}

func entregar(t *testing.T, s *Servico, tipo, produtor string, dados any) {
	t.Helper()
	env, err := evento.NovoEnvelope(tipo, produtor, dados)
	if err != nil {
		t.Fatalf("montando %s: %v", tipo, err)
	}
	if err := s.Tratar(context.Background(), env); err != nil {
		t.Fatalf("tratando %s: %v", tipo, err)
	}
}

func exigirStatus(t *testing.T, registro *Registro, pedidoID string, esperado Status) {
	t.Helper()
	pedido, existe := registro.Buscar(pedidoID)
	if !existe {
		t.Fatalf("pedido %s sumiu do registro", pedidoID)
	}
	if pedido.Status != esperado {
		t.Fatalf("status de %s: esperava %q, veio %q", pedidoID, esperado, pedido.Status)
	}
}

func TestEstoqueOKMarcaReservadoENaoCancela(t *testing.T) {
	s, registro, pub := montarServico(t)
	id := pedidoNoRegistro(t, registro)

	entregar(t, s, evento.PedidoEstoqueOK, evento.ServicoEstoque, evento.DadosPedidoEstoqueOK{
		PedidoID: id,
		Total:    4500,
	})

	exigirStatus(t, registro, id, StatusEstoqueReservado)
	if len(pub.publicados) != 0 {
		t.Fatalf("não deveria publicar nada, publicou %v", pub.chaves())
	}
}

// Um dos dois caminhos de cancelamento exigidos pelo enunciado.
func TestEstoqueIndisponivelCancelaEPublicaPedidoExcluido(t *testing.T) {
	s, registro, pub := montarServico(t)
	id := pedidoNoRegistro(t, registro)

	entregar(t, s, evento.EstoqueIndisponivel, evento.ServicoEstoque, evento.DadosEstoqueIndisponivel{
		PedidoID: id,
		Motivo:   "estoque insuficiente de Air Fryer 5L",
	})

	exigirStatus(t, registro, id, StatusSemEstoque)

	if len(pub.publicados) != 1 || pub.publicados[0].chave != evento.PedidoExcluido {
		t.Fatalf("esperava publicar %s, publicou %v", evento.PedidoExcluido, pub.chaves())
	}
	dados, ok := pub.publicados[0].dados.(evento.DadosPedidoExcluido)
	if !ok {
		t.Fatalf("payload inesperado: %T", pub.publicados[0].dados)
	}
	if dados.PedidoID != id {
		t.Fatalf("excluiu o pedido errado: %q", dados.PedidoID)
	}
}

func TestPagamentoAprovadoMarcaAprovadoENaoCancela(t *testing.T) {
	s, registro, pub := montarServico(t)
	id := pedidoNoRegistro(t, registro)

	entregar(t, s, evento.PagamentoAprovado, evento.ServicoPagamento, evento.DadosPagamentoAprovado{
		PedidoID:    id,
		TransacaoID: "TX-abc123",
		Valor:       4500,
	})

	exigirStatus(t, registro, id, StatusPagamentoAprovado)
	if len(pub.publicados) != 0 {
		t.Fatalf("não deveria publicar nada, publicou %v", pub.chaves())
	}
}

// O outro caminho de cancelamento. É este evento que faz o Estoque devolver
// o que estava reservado, então deixar de publicar prenderia estoque.
func TestPagamentoRecusadoCancelaEPublicaPedidoExcluido(t *testing.T) {
	s, registro, pub := montarServico(t)
	id := pedidoNoRegistro(t, registro)

	entregar(t, s, evento.PagamentoRecusado, evento.ServicoPagamento, evento.DadosPagamentoRecusado{
		PedidoID: id,
		Valor:    4500,
		Motivo:   "saldo insuficiente",
	})

	exigirStatus(t, registro, id, StatusPagamentoRecusado)
	if len(pub.publicados) != 1 || pub.publicados[0].chave != evento.PedidoExcluido {
		t.Fatalf("esperava publicar %s, publicou %v", evento.PedidoExcluido, pub.chaves())
	}
}

func TestPedidoEnviadoMarcaEnviado(t *testing.T) {
	s, registro, pub := montarServico(t)
	id := pedidoNoRegistro(t, registro)

	entregar(t, s, evento.PedidoEnviado, evento.ServicoEntrega, evento.DadosPedidoEnviado{
		PedidoID:       id,
		NotaFiscal:     "NF-0001",
		Transportadora: "Correios",
		CodigoRastreio: "BR123456789",
	})

	exigirStatus(t, registro, id, StatusEnviado)
	if len(pub.publicados) != 0 {
		t.Fatalf("não deveria publicar nada, publicou %v", pub.chaves())
	}
}

// Se o Principal for reiniciado, a fila dele ainda tem eventos de pedidos
// que a memória desta sessão não conhece. O cancelamento tem de sair mesmo
// assim, senão o Estoque fica com a reserva presa para sempre.
func TestPedidoDesconhecidoAindaAssimPublicaExclusao(t *testing.T) {
	s, _, pub := montarServico(t)

	entregar(t, s, evento.PagamentoRecusado, evento.ServicoPagamento, evento.DadosPagamentoRecusado{
		PedidoID: "PED-de-outra-sessao",
		Motivo:   "cartão expirado",
	})

	if len(pub.publicados) != 1 || pub.publicados[0].chave != evento.PedidoExcluido {
		t.Fatalf("esperava publicar %s, publicou %v", evento.PedidoExcluido, pub.chaves())
	}
}

func TestEventoDeOutroAssuntoEIgnorado(t *testing.T) {
	s, registro, pub := montarServico(t)
	id := pedidoNoRegistro(t, registro)

	entregar(t, s, evento.PedidoCriado, evento.ServicoPrincipal, evento.DadosPedidoCriado{PedidoID: id})

	exigirStatus(t, registro, id, StatusAguardandoEstoque)
	if len(pub.publicados) != 0 {
		t.Fatalf("não deveria publicar nada, publicou %v", pub.chaves())
	}
}

func TestHistoricoGuardaACadeiaDeStatus(t *testing.T) {
	s, registro, _ := montarServico(t)
	id := pedidoNoRegistro(t, registro)

	entregar(t, s, evento.PedidoEstoqueOK, evento.ServicoEstoque, evento.DadosPedidoEstoqueOK{PedidoID: id})
	entregar(t, s, evento.PagamentoAprovado, evento.ServicoPagamento, evento.DadosPagamentoAprovado{PedidoID: id})
	entregar(t, s, evento.PedidoEnviado, evento.ServicoEntrega, evento.DadosPedidoEnviado{PedidoID: id})

	pedido, _ := registro.Buscar(id)
	esperado := []Status{
		StatusAguardandoEstoque,
		StatusEstoqueReservado,
		StatusPagamentoAprovado,
		StatusEnviado,
	}

	if len(pedido.Historico) != len(esperado) {
		t.Fatalf("esperava %d marcas no histórico, vieram %d", len(esperado), len(pedido.Historico))
	}
	for i, status := range esperado {
		if pedido.Historico[i].Status != status {
			t.Fatalf("marca %d: esperava %q, veio %q", i, status, pedido.Historico[i].Status)
		}
	}
}

// Listar devolve cópias, então mexer no que voltou não pode alterar o
// registro. O menu percorre essa lista sem segurar o mutex enquanto o
// consumidor atualiza status em paralelo.
func TestListarDevolveCopias(t *testing.T) {
	registro := NovoRegistro()
	id := pedidoNoRegistro(t, registro)

	lista := registro.Listar()
	lista[0].Status = StatusEnviado

	exigirStatus(t, registro, id, StatusAguardandoEstoque)
}

func TestIdsDePedidoNaoSeRepetem(t *testing.T) {
	registro := NovoRegistro()
	vistos := make(map[string]bool)

	for i := 0; i < 500; i++ {
		pedido := registro.Criar("Daniel", nil)
		if vistos[pedido.ID] {
			t.Fatalf("identificador repetido: %s", pedido.ID)
		}
		vistos[pedido.ID] = true
	}
}
