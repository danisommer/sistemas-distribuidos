// Comando observador é uma FERRAMENTA DE DESENVOLVIMENTO, não faz parte da
// entrega. Ele escuta todos os eventos da exchange eCommerce e imprime cada
// um, para acompanhar o fluxo enquanto o microsserviço Principal não existe.
//
//	go run ./cmd/observador
//
// A fila dele é separada das filas dos microsserviços, então observar não
// rouba mensagem de ninguém: na exchange direct, cada fila vinculada à mesma
// routing key recebe a sua própria cópia.
package main

import (
	"context"
	"encoding/json"
	"log"
	"os"
	"os/signal"
	"syscall"

	"ecommerce/internal/cripto"
	"ecommerce/internal/evento"
	"ecommerce/internal/mensageria"
)

const nomeFila = "dev.observador"

func main() {
	log.SetPrefix("[observador] ")
	log.SetFlags(log.Ltime)

	ctx, parar := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer parar()

	if err := executar(ctx); err != nil {
		log.Fatalf("erro: %v", err)
	}
}

func executar(ctx context.Context) error {
	// Usa o chaveiro do Principal só porque ele já tem a chave pública de
	// todo mundo, que é o necessário para validar qualquer evento.
	chaveiro, err := cripto.CarregarChaveiro(mensageria.DiretorioChaves(), evento.ServicoPrincipal)
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

	consumidor, err := mensageria.NovoConsumidor(conexao, nomeFila, chaveiro)
	if err != nil {
		return err
	}

	err = consumidor.Vincular(evento.ExchangeECommerce,
		evento.PedidoCriado,
		evento.PedidoExcluido,
		evento.PedidoEstoqueOK,
		evento.EstoqueIndisponivel,
		evento.PagamentoAprovado,
		evento.PagamentoRecusado,
		evento.PedidoEnviado,
	)
	if err != nil {
		return err
	}

	return consumidor.Consumir(ctx, imprimir)
}

func imprimir(_ context.Context, env evento.Envelope) error {
	dados, err := json.MarshalIndent(json.RawMessage(env.Dados), "    ", "  ")
	if err != nil {
		dados = env.Dados
	}
	log.Printf("%s de %s\n    %s", env.Tipo, env.Produtor, dados)
	return nil
}
