// Comando entrega sobe o microsserviço de emissão de nota e expedição.
//
// ============================================================================
//
//	PARTE DA DUPLA — ainda não implementado
//
// ============================================================================
//
// Responsabilidades, conforme o enunciado:
//
//	Consome na fila evento.FilaEntrega, vinculada à exchange
//	evento.ExchangeECommerce com uma binding key:
//	  - evento.PagamentoAprovado, payload evento.DadosPagamentoAprovado
//	    (os itens do pedido vêm junto, para a nota)
//
//	Ao receber o evento: simula a emissão da nota fiscal e o preparo da
//	entrega, e então publica
//	  - evento.PedidoEnviado, payload evento.DadosPedidoEnviado
//	    (número da nota, transportadora e código de rastreio)
//
// Use cmd/pagamento/main.go como modelo: ele também tem uma binding key só e
// publica um evento em resposta. O chaveiro a carregar é o de
// evento.ServicoEntrega.
package main

import (
	"fmt"
	"os"
)

func main() {
	fmt.Fprintln(os.Stderr, "microsserviço Entrega ainda não implementado")
	fmt.Fprintln(os.Stderr, "veja o contrato no comentário de cmd/entrega/main.go")
	os.Exit(1)
}
