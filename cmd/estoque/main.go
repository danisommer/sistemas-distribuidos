// Comando estoque sobe o microsserviço de gerenciamento de estoque.
//
//	go run ./cmd/estoque
//
// Consome pedido.criado e pedido.excluido da exchange eCommerce e publica
// pedido.estoque_ok ou estoque.indisponivel.
package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"ecommerce/internal/cripto"
	"ecommerce/internal/estoque"
	"ecommerce/internal/evento"
	"ecommerce/internal/mensageria"
)

func main() {
	log.SetPrefix("[estoque] ")
	log.SetFlags(log.Ltime)

	ctx, parar := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer parar()

	if err := executar(ctx); err != nil {
		log.Fatalf("erro: %v", err)
	}
	log.Println("encerrado")
}

func executar(ctx context.Context) error {
	chaveiro, err := cripto.CarregarChaveiro(mensageria.DiretorioChaves(), evento.ServicoEstoque)
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
	servico := estoque.NovoServico(publicador)

	consumidor, err := mensageria.NovoConsumidor(conexao, evento.FilaEstoque, chaveiro)
	if err != nil {
		return err
	}

	err = consumidor.Vincular(evento.ExchangeECommerce,
		evento.PedidoCriado,
		evento.PedidoExcluido,
	)
	if err != nil {
		return err
	}

	return consumidor.Consumir(ctx, servico.Tratar)
}
