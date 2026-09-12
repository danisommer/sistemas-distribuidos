// Comando principal sobe o microsserviço Principal, a interface de terminal
// do sistema.
//
//	go run ./cmd/principal
//
// Publica pedido.criado e pedido.excluido na exchange eCommerce. Consome
// pedido.estoque_ok, estoque.indisponivel, pagamento.aprovado,
// pagamento.recusado e pedido.enviado, atualizando o status dos pedidos.
//
// O menu roda na goroutine principal e o consumidor em paralelo, então as
// mudanças de status aparecem na tela sozinhas, sem o usuário pedir.
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

// arquivoDeLog recebe o log da camada de mensageria. Ele não vai para a tela
// porque brigaria com o menu; para acompanhar o tráfego do broker ao vivo,
// abra outro terminal e rode: tail -f principal.log
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

	// Exchange direct: uma binding key por evento que interessa, todas na
	// mesma fila do Principal.
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

	// Ctrl+C chega enquanto o menu está parado lendo o teclado, e não dá
	// para interromper essa leitura. Encerrar o processo aqui é o caminho
	// curto; as filas são duráveis, então nada se perde.
	go func() {
		<-ctx.Done()
		fmt.Println("\nencerrando.")
		os.Exit(0)
	}()

	fmt.Printf("log do broker em %s (acompanhe com: tail -f %s)\n", arquivoDeLog, arquivoDeLog)
	return menu.Rodar(ctx)
}
