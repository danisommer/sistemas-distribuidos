// Comando principal sobe o microsserviço Principal, a interface de terminal
// do sistema.
//
//	go run ./cmd/principal
//
// Publica pedido.criado e pedido.excluido na exchange eCommerce. Consome
// pedido.estoque_ok, estoque.indisponivel, pagamento.aprovado,
// pagamento.recusado e pedido.enviado, atualizando o status dos pedidos.
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"ecommerce/internal/cripto"
	"ecommerce/internal/evento"
	"ecommerce/internal/mensageria"
	"ecommerce/internal/principal"
)

// arquivoDeLog recebe o log da camada de mensageria.
const arquivoDeLog = "principal.log"

func main() {
	ctx, parar := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer parar()

	if err := executar(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "\nerro: %v\n", err)
		os.Exit(1)
	}
}

func executar(ctx context.Context) error {
	destinoLog, err := os.OpenFile(arquivoDeLog, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("abrindo %s: %w", arquivoDeLog, err)
	}
	defer destinoLog.Close()
	mensageria.DefinirLog(log.New(destinoLog, "", log.LstdFlags))

	chaveiro, err := cripto.CarregarChaveiro(mensageria.DiretorioChaves(), evento.ServicoPrincipal)
	if err != nil {
		return err
	}

	fmt.Println("conectando no broker...")
	conexao, err := mensageria.Conectar(mensageria.URLBroker())
	if err != nil {
		return err
	}
	defer conexao.Fechar()

	if err := conexao.DeclararExchanges(); err != nil {
		return err
	}

	publicador := mensageria.NovoPublicador(conexao, chaveiro)
	registro := principal.NovoRegistro()
	menu := principal.NovoMenu(registro, publicador, os.Stdin, os.Stdout)
	servico := principal.NovoServico(registro, publicador, menu.Avisar)

	consumidor, err := mensageria.NovoConsumidor(conexao, evento.FilaPrincipal, chaveiro)
	if err != nil {
		return err
	}

	err = consumidor.Vincular(evento.ExchangeECommerce,
		evento.PedidoEstoqueOK,
		evento.EstoqueIndisponivel,
		evento.PagamentoAprovado,
		evento.PagamentoRecusado,
		evento.PedidoEnviado,
	)
	if err != nil {
		return err
	}

	ctxConsumo, pararConsumo := context.WithCancel(ctx)
	defer pararConsumo()

	go func() {
		if err := consumidor.Consumir(ctxConsumo, servico.Tratar); err != nil {
			menu.Avisar("o consumo parou: " + err.Error())
		}
	}()

	go func() {
		<-ctx.Done()
		fmt.Println("\nencerrando.")
		os.Exit(0)
	}()

	fmt.Printf("log do broker em %s (acompanhe com: tail -f %s)\n", arquivoDeLog, arquivoDeLog)
	return menu.Rodar(ctx)
}
