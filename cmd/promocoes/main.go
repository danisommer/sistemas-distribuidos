// Comando promocoes sobe o microsserviço gerador de promoções.
//
//	go run ./cmd/promocoes
//
// Publica promocao.categoria.<X> na exchange promocoes, em intervalos
// aleatórios.
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
	"ecommerce/internal/promocoes"
)

func main() {
	log.SetPrefix("[promocoes] ")
	log.SetFlags(log.Ltime)

	ctx, parar := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer parar()

	if err := executar(ctx); err != nil {
		log.Fatalf("erro: %v", err)
	}
	log.Println("encerrado")
}

func executar(ctx context.Context) error {
	chaveiro, err := cripto.CarregarChaveiro(mensageria.DiretorioChaves(), evento.ServicoPromocoes)
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
	servico := promocoes.NovoServico(publicador)

	return servico.EnviarPromocoes(ctx)
}
