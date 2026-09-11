// Comando promocoes sobe o microsserviço gerador de promoções.
//
// ============================================================================
//
//	PARTE DA DUPLA — ainda não implementado
//
// ============================================================================
//
// Responsabilidades, conforme o enunciado:
//
//	Não consome nada. De tempos em tempos sorteia um produto de
//	catalogo.Produtos, sorteia um desconto e publica na exchange
//	evento.ExchangePromocoes (topic) usando a routing key da categoria do
//	produto:
//	  - promocao.categoria.A, promocao.categoria.B, promocao.categoria.C
//
//	O publicador já monta a routing key sozinho:
//	  publicador.PublicarPromocao(ctx, produto.Categoria, dados)
//	com dados do tipo evento.DadosPromocao.
//
// O esqueleto é mais curto que o dos outros: carregar o chaveiro de
// evento.ServicoPromocoes, conectar, declarar as exchanges, montar o
// publicador e rodar um time.Ticker publicando até o contexto ser cancelado.
// Sem consumidor.
package main

import (
	"fmt"
	"os"
)

func main() {
	fmt.Fprintln(os.Stderr, "microsserviço Promoções ainda não implementado")
	fmt.Fprintln(os.Stderr, "veja o contrato no comentário de cmd/promocoes/main.go")
	os.Exit(1)
}
