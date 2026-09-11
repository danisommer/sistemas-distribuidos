// Comando consumidor-c2 é o processo consumidor de promoções C2.
//
// ============================================================================
//
//	PARTE DA DUPLA — ainda não implementado
//
// ============================================================================
//
// C2 registra interesse em todas as categorias. Como C1, ele não pode chamar
// nenhum microsserviço: fala apenas com o RabbitMQ.
//
//	Fila: evento.FilaPromocaoC2
//	Exchange: evento.ExchangePromocoes (topic)
//	Binding key: evento.BindingC2TodasCategorias ("promocao.categoria.#")
//
// Uma binding só. O # casa zero ou mais palavras, então toda categoria
// existente e qualquer categoria futura caem nesta fila. É o contraste com o
// C1, que precisa de uma binding por categoria.
//
// Ao receber, basta imprimir a promoção (payload evento.DadosPromocao).
// Não publica nada.
package main

import (
	"fmt"
	"os"
)

func main() {
	fmt.Fprintln(os.Stderr, "consumidor C2 de promoções ainda não implementado")
	fmt.Fprintln(os.Stderr, "veja o contrato no comentário de cmd/consumidor-c2/main.go")
	os.Exit(1)
}
