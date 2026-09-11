// Comando teste-publicador é uma FERRAMENTA DE DESENVOLVIMENTO, não faz
// parte da entrega. Ele publica eventos assinados na mão, para testar o
// Estoque e o Pagamento enquanto o microsserviço Principal não existe.
//
// Exemplos:
//
//	# pedido que cabe no estoque
//	go run ./cmd/teste-publicador -itens P001:2,P004:1
//
//	# pedido que estoura o estoque do Air Fryer (só existem 8)
//	go run ./cmd/teste-publicador -itens P006:99
//
//	# cancelar um pedido e ver o estorno no Estoque
//	go run ./cmd/teste-publicador -tipo pedido.excluido -pedido PED-001 -motivo "teste"
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	"ecommerce/internal/catalogo"
	"ecommerce/internal/cripto"
	"ecommerce/internal/evento"
	"ecommerce/internal/mensageria"
)

func main() {
	log.SetPrefix("[teste] ")
	log.SetFlags(log.Ltime)

	como := flag.String("como", evento.ServicoPrincipal, "microsserviço que assina o evento")
	tipo := flag.String("tipo", evento.PedidoCriado, "routing key do evento")
	pedidoID := flag.String("pedido", "", "identificador do pedido (padrão: gerado pelo relógio)")
	cliente := flag.String("cliente", "cliente-teste", "nome do cliente")
	itens := flag.String("itens", "P001:1", "itens no formato PRODUTO:QTD,PRODUTO:QTD")
	motivo := flag.String("motivo", "cancelado no teste", "motivo, usado em pedido.excluido")
	flag.Parse()

	if *pedidoID == "" {
		*pedidoID = "PED-" + time.Now().Format("150405")
	}

	if err := executar(*como, *tipo, *pedidoID, *cliente, *itens, *motivo); err != nil {
		log.Fatalf("erro: %v", err)
	}
}

func executar(como, tipo, pedidoID, cliente, itensBrutos, motivo string) error {
	chaveiro, err := cripto.CarregarChaveiro(mensageria.DiretorioChaves(), como)
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
	ctx, cancelar := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancelar()

	switch tipo {
	case evento.PedidoCriado:
		itens, total, err := montarItens(itensBrutos)
		if err != nil {
			return err
		}
		log.Printf("pedido %s com %d item(ns), total R$ %.2f", pedidoID, len(itens), total)
		return publicador.PublicarECommerce(ctx, evento.PedidoCriado, evento.DadosPedidoCriado{
			PedidoID: pedidoID,
			Cliente:  cliente,
			Itens:    itens,
			Total:    total,
		})

	case evento.PedidoExcluido:
		return publicador.PublicarECommerce(ctx, evento.PedidoExcluido, evento.DadosPedidoExcluido{
			PedidoID: pedidoID,
			Motivo:   motivo,
		})

	default:
		return fmt.Errorf("tipo %q não é suportado por esta ferramenta (use %s ou %s)",
			tipo, evento.PedidoCriado, evento.PedidoExcluido)
	}
}

// montarItens transforma "P001:2,P004:1" na lista de itens do pedido,
// buscando nome e preço no catálogo.
func montarItens(bruto string) ([]evento.ItemPedido, float64, error) {
	var itens []evento.ItemPedido
	var total float64

	for _, parte := range strings.Split(bruto, ",") {
		parte = strings.TrimSpace(parte)
		if parte == "" {
			continue
		}

		id, qtdBruta, achou := strings.Cut(parte, ":")
		if !achou {
			return nil, 0, fmt.Errorf("item %q fora do formato PRODUTO:QTD", parte)
		}

		quantidade, err := strconv.Atoi(strings.TrimSpace(qtdBruta))
		if err != nil {
			return nil, 0, fmt.Errorf("quantidade inválida em %q: %w", parte, err)
		}

		id = strings.TrimSpace(id)
		produto, existe := catalogo.Buscar(id)
		if !existe {
			// Deixa passar de propósito: é assim que se testa o caminho de
			// produto inexistente no Estoque.
			log.Printf("aviso: %s não está no catálogo, publicando mesmo assim", id)
			itens = append(itens, evento.ItemPedido{ProdutoID: id, Quantidade: quantidade})
			continue
		}

		itens = append(itens, evento.ItemPedido{
			ProdutoID:  produto.ID,
			Nome:       produto.Nome,
			Quantidade: quantidade,
			PrecoUnit:  produto.Preco,
		})
		total += produto.Preco * float64(quantidade)
	}

	if len(itens) == 0 {
		return nil, 0, fmt.Errorf("nenhum item informado")
	}
	return itens, total, nil
}
