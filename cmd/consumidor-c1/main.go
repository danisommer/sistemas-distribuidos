// Comando consumidor-c1 é o processo consumidor de promoções C1.
//
// ============================================================================
//
//	PARTE DA DUPLA — ainda não implementado
//
// ============================================================================
//
// C1 registra interesse só nas categorias A e B. Conforme o enunciado, ele
// não pode chamar nenhum microsserviço: fala apenas com o RabbitMQ.
//
//	Fila: evento.FilaPromocaoC1
//	Exchange: evento.ExchangePromocoes (topic)
//	Binding keys: evento.BindingC1CategoriaA e evento.BindingC1CategoriaB
//
// Duas bindings exatas na mesma fila. Sem curinga aqui: a promoção de
// categoria C existe, é publicada, e simplesmente não chega nesta fila. É
// esse o ponto a mostrar na defesa, lado a lado com o C2.
//
// Ao receber, basta imprimir a promoção (payload evento.DadosPromocao).
// Não publica nada.
package main

import (
	"fmt"
	"os"
)

func main() {
	fmt.Fprintln(os.Stderr, "consumidor C1 de promoções ainda não implementado")
	fmt.Fprintln(os.Stderr, "veja o contrato no comentário de cmd/consumidor-c1/main.go")
	os.Exit(1)
}
