package evento

// Nomes das filas do sistema. Cada consumidor tem a sua própria fila.
const (
	FilaPrincipal  = "principal.status"
	FilaEstoque    = "estoque.pedidos"
	FilaPagamento  = "pagamento.estoque_ok"
	FilaEntrega    = "entrega.pagamentos"
	FilaPromocaoC1 = "promocoes.c1"
	FilaPromocaoC2 = "promocoes.c2"
)

// Binding keys dos consumidores de promoções na exchange topic.
const (
	BindingC1CategoriaA = PrefixoPromocao + "A"
	BindingC1CategoriaB = PrefixoPromocao + "B"

	BindingC2TodasCategorias = PrefixoPromocao + "#"
)
