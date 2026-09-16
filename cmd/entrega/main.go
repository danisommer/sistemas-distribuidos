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
	chaveiro, err := cripto.CarregarChaveiro(mensageria.DiretorioChaves(), evento.ServicoEntrega)
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

	consumidor, err := mensageria.NovoConsumidor(conexao, evento.FilaEntrega, chaveiro)
	if err != nil {
		return err
	}

	if err := consumidor.Vincular(evento.ExchangeECommerce, evento.PagamentoAprovado); err != nil {
		return err
	}

	return consumidor.Consumir(ctx, servico.Tratar)
}
