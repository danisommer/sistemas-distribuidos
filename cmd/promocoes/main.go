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
// Comando entrega sobe o microsserviço de emissão de nota e expedição.

package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"ecommerce/internal/cripto"
	"ecommerce/internal/entrega"
	"ecommerce/internal/evento"
	"ecommerce/internal/mensageria"
)

func main() {
	log.SetPrefix("[entrega] ")
	log.SetFlags(log.Ltime)

	ctx, parar := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer parar()

	if err := executar(ctx); err != nil {
		log.Fatalf("erro: %v", err)
	}
	log.Println("encerrado")
}

func executar(ctx context.Context) error {
	chaveiro, err := cripto.CarregarChaveiro(mensageria.DiretorioChaves(), evento.ServicoPagamento)
	if err != nil {
		return err
	}

	conexao, err := mensageria.Conectar(mensageria.URLBroker())
	if err != nil {
		return err
	}
	defer conexao.Fechar()

	if err := conexao.DeclararExchanges(); err != nil {
		return err
	}

	publicador := mensageria.NovoPublicador(conexao, chaveiro)
	servico := entrega.NovoServico(publicador)

	consumidor, err := mensageria.NovoConsumidor(conexao, evento.FilaPagamento, chaveiro)
	if err != nil {
		return err
	}

	if err := consumidor.Vincular(evento.ExchangeECommerce, evento.PagamentoAprovado); err != nil {
		return err
	}

	return consumidor.Consumir(ctx, servico.Tratar)
}
