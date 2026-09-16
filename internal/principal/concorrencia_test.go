package principal

import (
	"context"
	"strings"
	"sync"
	"testing"

	"ecommerce/internal/evento"
)

// Rodar com o detector de corrida:
//
//	go test -race ./internal/principal/
func TestRegistroAguentaMenuEConsumidorAoMesmoTempo(t *testing.T) {
	registro := NovoRegistro()
	servico := NovoServico(registro, &publicadorFalso{}, nil)

	const pedidosPorGoroutine = 50

	ids := make(chan string, pedidosPorGoroutine*2)

	var grupo sync.WaitGroup

	for i := 0; i < 2; i++ {
		grupo.Add(1)
		go func() {
			defer grupo.Done()
			for j := 0; j < pedidosPorGoroutine; j++ {
				pedido := registro.Criar("Daniel", []evento.ItemPedido{
					{ProdutoID: "P001", Nome: "Notebook", Quantidade: 1, PrecoUnit: 4500},
				})
				ids <- pedido.ID
			}
		}()
	}

	go func() {
		grupo.Wait()
		close(ids)
	}()

	var consumo sync.WaitGroup
	consumo.Add(3)

	go func() {
		defer consumo.Done()
		for id := range ids {
			env, err := evento.NovoEnvelope(evento.PedidoEstoqueOK, evento.ServicoEstoque,
				evento.DadosPedidoEstoqueOK{PedidoID: id})
			if err != nil {
				t.Errorf("montando envelope: %v", err)
				return
			}
			if err := servico.Tratar(context.Background(), env); err != nil {
				t.Errorf("tratando evento: %v", err)
				return
			}
		}
	}()

	for i := 0; i < 2; i++ {
		go func() {
			defer consumo.Done()
			for j := 0; j < 200; j++ {
				for _, pedido := range registro.Listar() {
					_ = pedido.Status
				}
			}
		}()
	}

	consumo.Wait()

	if registro.Quantidade() != pedidosPorGoroutine*2 {
		t.Fatalf("esperava %d pedidos, vieram %d", pedidosPorGoroutine*2, registro.Quantidade())
	}
}

func TestMenuEAvisoNaoSeAtropelamNaTela(t *testing.T) {
	registro := NovoRegistro()
	var tela strings.Builder

	menu := NovoMenu(registro, &publicadorFalso{}, strings.NewReader("Daniel\n1\n1\n1\n0\n"), &tela)

	var avisos sync.WaitGroup
	avisos.Add(1)
	pronto := make(chan struct{})

	go func() {
		defer avisos.Done()
		for i := 0; i < 300; i++ {
			menu.Avisar("PED-abc123 → pagamento aprovado")
		}
		close(pronto)
	}()

	if err := menu.Rodar(context.Background()); err != nil {
		t.Fatalf("rodando o menu: %v", err)
	}

	<-pronto
	avisos.Wait()

	if !strings.Contains(tela.String(), "pagamento aprovado") {
		t.Fatal("os avisos não chegaram na tela")
	}
}
