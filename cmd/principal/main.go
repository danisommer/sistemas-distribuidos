// Comando principal sobe o microsserviço Principal, a interface de terminal
// do sistema.
//
// ============================================================================
//
//	A FAZER EM DUPLA — ainda não implementado
//
// ============================================================================
//
// Responsabilidades, conforme o enunciado:
//
//	Menu no terminal:
//	  - visualizar produtos (use catalogo.Produtos)
//	  - realizar pedidos
//	  - excluir pedidos
//	  - consultar os pedidos e os respectivos status
//
//	Publica na exchange eCommerce (evento.ExchangeECommerce):
//	  - evento.PedidoCriado ...... a cada pedido novo, com
//	                               evento.DadosPedidoCriado
//	  - evento.PedidoExcluido .... quando o estoque falta ou o pagamento é
//	                               recusado, com evento.DadosPedidoExcluido
//
//	Consome na fila evento.FilaPrincipal, com estas cinco binding keys:
//	  - evento.PagamentoAprovado   → status "pagamento aprovado"
//	  - evento.PagamentoRecusado   → status "cancelado" + publica PedidoExcluido
//	  - evento.PedidoEnviado       → status "enviado"
//	  - evento.PedidoEstoqueOK     → status "estoque reservado"
//	  - evento.EstoqueIndisponivel → status "cancelado" + publica PedidoExcluido
//
// O esqueleto é o mesmo de cmd/estoque/main.go: carregar o chaveiro,
// conectar, declarar as exchanges, montar publicador e consumidor, vincular
// as binding keys e consumir. A diferença é que aqui o laço do menu roda em
// paralelo com o consumidor, então o estado dos pedidos precisa de mutex.
package main

import (
	"fmt"
	"os"
)

func main() {
	fmt.Fprintln(os.Stderr, "microsserviço Principal ainda não implementado (parte em dupla)")
	fmt.Fprintln(os.Stderr, "veja o contrato no comentário de cmd/principal/main.go")
	os.Exit(1)
}
