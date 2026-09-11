package evento

// Nomes das filas do sistema.
//
// Cada consumidor tem a sua própria fila, como pede o enunciado. Se dois
// consumidores dividissem uma fila, o broker distribuiria as mensagens entre
// eles (round-robin) em vez de entregar uma cópia a cada um, e por exemplo o
// Principal perderia metade das atualizações de status.
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
	// C1 registra interesse só nas categorias A e B, com duas bindings
	// exatas na mesma fila.
	BindingC1CategoriaA = PrefixoPromocao + "A"
	BindingC1CategoriaB = PrefixoPromocao + "B"

	// C2 registra interesse em todas as categorias. O # casa zero ou mais
	// palavras depois de "promocao.categoria.", então uma categoria nova
	// passa a chegar sem precisar de binding nova.
	BindingC2TodasCategorias = PrefixoPromocao + "#"
)
