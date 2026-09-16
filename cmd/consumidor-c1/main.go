// Comando consumidor-c1 é o processo consumidor de promoções C1.
//
//	go run ./cmd/consumidor-c1
//
// Consome promocao.categoria.A e promocao.categoria.B da exchange promocoes.
package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"ecommerce/internal/consumidor"
	"ecommerce/internal/cripto"
	"ecommerce/internal/evento"
	"ecommerce/internal/mensageria"
)

var TIPO_PROMOCOES = []string{evento.BindingC1CategoriaA, evento.BindingC1CategoriaB}

func main() {
	log.SetPrefix("[consumidor1] ")
	log.SetFlags(log.Ltime)

	ctx, parar := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer parar()

	if err := executar(ctx); err != nil {
		log.Fatalf("erro: %v", err)
	}
	log.Println("encerrado")
}

func executar(ctx context.Context) error {
	chaveiro, err := cripto.CarregarChaveiroPublico(mensageria.DiretorioChaves(), evento.ServicoConsumidores)
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

	mensagariaConsumidor, err := mensageria.NovoConsumidor(conexao, evento.FilaPromocaoC1, chaveiro)
	if err != nil {
		return err
	}

	if err := mensagariaConsumidor.Vincular(evento.ExchangePromocoes, TIPO_PROMOCOES...); err != nil {
		return err
	}

	servico := consumidor.NovoServico()

	return mensagariaConsumidor.Consumir(ctx, servico.Tratar)
}
