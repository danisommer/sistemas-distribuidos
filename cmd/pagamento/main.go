// Comando pagamento sobe o microsserviço de processamento de pagamentos.
//
//	go run ./cmd/pagamento
//
// Consome pedido.estoque_ok da exchange eCommerce e publica
// pagamento.aprovado ou pagamento.recusado.
package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"ecommerce/internal/cripto"
	"ecommerce/internal/evento"
	"ecommerce/internal/mensageria"
	"ecommerce/internal/pagamento"
)

func main() {
	log.SetPrefix("[pagamento] ")
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
	servico := pagamento.NovoServico(publicador)

	consumidor, err := mensageria.NovoConsumidor(conexao, evento.FilaPagamento, chaveiro)
	if err != nil {
		return err
	}

	if err := consumidor.Vincular(evento.ExchangeECommerce, evento.PedidoEstoqueOK); err != nil {
		return err
	}

	return consumidor.Consumir(ctx, servico.Tratar)
}
