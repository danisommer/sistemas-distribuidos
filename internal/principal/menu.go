package principal

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"strconv"
	"strings"
	"sync"
	"unicode/utf8"

	"ecommerce/internal/catalogo"
	"ecommerce/internal/evento"
)

// Menu é a interação com o usuário pelo terminal. Toda escrita na tela passa
// pelo mutex, pois Avisar é chamado pelo consumidor em paralelo.
type Menu struct {
	mu     sync.Mutex
	saida  io.Writer
	leitor *bufio.Scanner

	registro   *Registro
	publicador Publicador
	cliente    string
}

// NovoMenu monta o menu.
func NovoMenu(registro *Registro, publicador Publicador, entrada io.Reader, saida io.Writer) *Menu {
	return &Menu{
		saida:      saida,
		leitor:     bufio.NewScanner(entrada),
		registro:   registro,
		publicador: publicador,
	}
}

// Avisar imprime uma mudança de status vinda do consumidor. É esta função
// que o Servico recebe como callback.
func (m *Menu) Avisar(texto string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	fmt.Fprintf(m.saida, "\n  >> %s\n", texto)
}

// Rodar pede o nome do cliente e entra no laço do menu, até o usuário
// escolher sair ou o contexto ser cancelado.
func (m *Menu) Rodar(ctx context.Context) error {
	m.imprimir("\n=== Loja ===\n")

	nome, ok := m.perguntar("Seu nome: ")
	if !ok {
		return nil
	}
	if nome == "" {
		nome = "cliente"
	}
	m.cliente = nome

	m.imprimir("\nOlá, %s.\n", m.cliente)

	for {
		if ctx.Err() != nil {
			return nil
		}

		m.mostrarOpcoes()
		escolha, ok := m.perguntar("Opção: ")
		if !ok {
			return nil
		}

		switch escolha {
		case "1":
			m.mostrarProdutos()
		case "2":
			if err := m.realizarPedido(ctx); err != nil {
				m.imprimir("\nNão consegui enviar o pedido: %v\n", err)
			}
		case "3":
			if err := m.excluirPedido(ctx); err != nil {
				m.imprimir("\nNão consegui excluir o pedido: %v\n", err)
			}
		case "4":
			m.consultarPedidos()
		case "0":
			m.imprimir("\nAté mais.\n")
			return nil
		case "":
		default:
			m.imprimir("\nOpção inválida.\n")
		}
	}
}

func (m *Menu) mostrarOpcoes() {
	m.imprimir(`
------------------------------
  1) Visualizar produtos
  2) Realizar pedido
  3) Excluir pedido
  4) Consultar meus pedidos
  0) Sair
------------------------------
`)
}

func (m *Menu) mostrarProdutos() {
	var b strings.Builder
	fmt.Fprintf(&b, "\nProdutos\n\n")
	for i, p := range catalogo.Produtos {
		fmt.Fprintf(&b, "  %2d) %-5s %s cat %s   %12s\n",
			i+1, p.ID, preencher(p.Nome, 22), p.Categoria, moeda(p.Preco))
	}
	fmt.Fprintf(&b, "\nA quantidade disponível é do microsserviço Estoque.\n")
	fmt.Fprintf(&b, "O Principal só descobre se o pedido cabe quando o evento de resposta chega.\n")
	m.imprimir("%s", b.String())
}

// realizarPedido monta o pedido item a item e publica pedido.criado.
func (m *Menu) realizarPedido(ctx context.Context) error {
	m.mostrarProdutos()
	m.imprimir("\nMonte o pedido. Deixe o produto em branco para fechar.\n\n")

	quantidades := make(map[string]int)
	var ordem []string

	for {
		escolha, ok := m.perguntar("  Produto (número ou código): ")
		if !ok {
			return nil
		}
		if escolha == "" {
			break
		}

		produto, achou := resolverProduto(escolha)
		if !achou {
			m.imprimir("  Produto não encontrado.\n")
			continue
		}

		bruto, ok := m.perguntar("  Quantidade: ")
		if !ok {
			return nil
		}
		quantidade, err := strconv.Atoi(strings.TrimSpace(bruto))
		if err != nil || quantidade <= 0 {
			m.imprimir("  Quantidade inválida.\n")
			continue
		}

		if _, repetido := quantidades[produto.ID]; !repetido {
			ordem = append(ordem, produto.ID)
		}
		quantidades[produto.ID] += quantidade

		m.imprimir("  + %s x%d\n", produto.Nome, quantidade)
	}

	if len(ordem) == 0 {
		m.imprimir("\nPedido vazio, nada enviado.\n")
		return nil
	}

	itens := make([]evento.ItemPedido, 0, len(ordem))
	for _, id := range ordem {
		produto, _ := catalogo.Buscar(id)
		itens = append(itens, evento.ItemPedido{
			ProdutoID:  produto.ID,
			Nome:       produto.Nome,
			Quantidade: quantidades[id],
			PrecoUnit:  produto.Preco,
		})
	}

	m.imprimir("\n%s\n", resumoDoPedido(itens))

	confirmacao, ok := m.perguntar("Confirmar pedido? (s/n): ")
	if !ok {
		return nil
	}
	if !sim(confirmacao) {
		m.imprimir("\nPedido descartado.\n")
		return nil
	}

	pedido := m.registro.Criar(m.cliente, itens)

	err := m.publicador.PublicarECommerce(ctx, evento.PedidoCriado, evento.DadosPedidoCriado{
		PedidoID: pedido.ID,
		Cliente:  pedido.Cliente,
		Itens:    pedido.Itens,
		Total:    pedido.Total,
	})
	if err != nil {
		m.registro.Atualizar(pedido.ID, StatusCanceladoPeloUso, "falha ao publicar no broker")
		return err
	}

	m.imprimir("\nPedido %s enviado. Status: %s.\n", pedido.ID, pedido.Status)
	m.imprimir("As mudanças de status vão aparecendo sozinhas, conforme os eventos chegam.\n")
	return nil
}

// excluirPedido publica pedido.excluido para o Estoque soltar a reserva.
func (m *Menu) excluirPedido(ctx context.Context) error {
	pedidos := m.registro.Listar()
	if len(pedidos) == 0 {
		m.imprimir("\nVocê ainda não fez nenhum pedido.\n")
		return nil
	}

	m.imprimir("%s", tabelaDePedidos(pedidos))

	escolha, ok := m.perguntar("Número do pedido a excluir (vazio para voltar): ")
	if !ok || escolha == "" {
		return nil
	}

	indice, err := strconv.Atoi(strings.TrimSpace(escolha))
	if err != nil || indice < 1 || indice > len(pedidos) {
		m.imprimir("\nNúmero inválido.\n")
		return nil
	}

	pedido := pedidos[indice-1]
	if pedido.Status.Encerrado() {
		m.imprimir("\nO pedido %s já está %s e não pode mais ser excluído.\n", pedido.ID, pedido.Status)
		return nil
	}

	confirmacao, ok := m.perguntar(fmt.Sprintf("Excluir o pedido %s? (s/n): ", pedido.ID))
	if !ok {
		return nil
	}
	if !sim(confirmacao) {
		return nil
	}

	err = m.publicador.PublicarECommerce(ctx, evento.PedidoExcluido, evento.DadosPedidoExcluido{
		PedidoID: pedido.ID,
		Motivo:   "excluído pelo usuário",
	})
	if err != nil {
		return err
	}

	m.registro.Atualizar(pedido.ID, StatusCanceladoPeloUso, "excluído pelo usuário")
	m.imprimir("\nPedido %s excluído. O Estoque devolve os itens reservados.\n", pedido.ID)
	return nil
}

func (m *Menu) consultarPedidos() {
	pedidos := m.registro.Listar()
	if len(pedidos) == 0 {
		m.imprimir("\nVocê ainda não fez nenhum pedido.\n")
		return
	}

	m.imprimir("%s", tabelaDePedidos(pedidos))

	escolha, ok := m.perguntar("Número do pedido para ver o histórico (vazio para voltar): ")
	if !ok || escolha == "" {
		return
	}

	indice, err := strconv.Atoi(strings.TrimSpace(escolha))
	if err != nil || indice < 1 || indice > len(pedidos) {
		m.imprimir("\nNúmero inválido.\n")
		return
	}

	m.imprimir("%s", historicoDoPedido(pedidos[indice-1]))
}

// perguntar imprime o prompt e lê uma linha. O segundo retorno é falso
// quando a entrada acabou, e aí o menu encerra.
func (m *Menu) perguntar(prompt string) (string, bool) {
	m.imprimir("%s", prompt)

	if !m.leitor.Scan() {
		return "", false
	}
	return strings.TrimSpace(m.leitor.Text()), true
}

func (m *Menu) imprimir(formato string, args ...any) {
	m.mu.Lock()
	defer m.mu.Unlock()
	fmt.Fprintf(m.saida, formato, args...)
}

func tabelaDePedidos(pedidos []Pedido) string {
	var b strings.Builder
	fmt.Fprintf(&b, "\nSeus pedidos\n\n")
	for i, p := range pedidos {
		fmt.Fprintf(&b, "  %2d) %-12s %s %12s\n", i+1, p.ID, preencher(string(p.Status), 30), moeda(p.Total))
		if p.Detalhe != "" {
			fmt.Fprintf(&b, "      %s\n", p.Detalhe)
		}
	}
	fmt.Fprintln(&b)
	return b.String()
}

func historicoDoPedido(p Pedido) string {
	var b strings.Builder
	fmt.Fprintf(&b, "\nPedido %s, cliente %s\n\n", p.ID, p.Cliente)

	for _, item := range p.Itens {
		fmt.Fprintf(&b, "  %s x%-3d %12s\n", preencher(item.Nome, 22), item.Quantidade, moeda(item.PrecoUnit*float64(item.Quantidade)))
	}
	fmt.Fprintf(&b, "  %s      %12s\n\n", preencher("TOTAL", 22), moeda(p.Total))

	fmt.Fprintf(&b, "  Histórico\n")
	for _, marca := range p.Historico {
		fmt.Fprintf(&b, "    %s  %s %s\n", marca.Quando.Format("15:04:05"), preencher(string(marca.Status), 30), marca.Detalhe)
	}
	return b.String()
}

func resumoDoPedido(itens []evento.ItemPedido) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Resumo\n\n")

	var total float64
	for _, item := range itens {
		subtotal := item.PrecoUnit * float64(item.Quantidade)
		total += subtotal
		fmt.Fprintf(&b, "  %s x%-3d %12s\n", preencher(item.Nome, 22), item.Quantidade, moeda(subtotal))
	}
	fmt.Fprintf(&b, "  %s      %12s\n", preencher("TOTAL", 22), moeda(total))
	return b.String()
}

// preencher completa o texto com espaços até a largura pedida, contando
// caracteres e não bytes.
func preencher(texto string, largura int) string {
	faltam := largura - utf8.RuneCountInString(texto)
	if faltam <= 0 {
		return texto
	}
	return texto + strings.Repeat(" ", faltam)
}

// resolverProduto aceita o número da linha na vitrine ou o código do produto.
func resolverProduto(escolha string) (catalogo.Produto, bool) {
	escolha = strings.TrimSpace(escolha)

	if indice, err := strconv.Atoi(escolha); err == nil {
		if indice >= 1 && indice <= len(catalogo.Produtos) {
			return catalogo.Produtos[indice-1], true
		}
		return catalogo.Produto{}, false
	}

	return catalogo.Buscar(strings.ToUpper(escolha))
}

func sim(resposta string) bool {
	resposta = strings.ToLower(strings.TrimSpace(resposta))
	return resposta == "s" || resposta == "sim" || resposta == "y"
}

// moeda formata no padrão brasileiro: R$ 4.500,00.
func moeda(valor float64) string {
	texto := strconv.FormatFloat(valor, 'f', 2, 64)
	inteiro, centavos, _ := strings.Cut(texto, ".")

	var grupos []string
	for len(inteiro) > 3 {
		grupos = append([]string{inteiro[len(inteiro)-3:]}, grupos...)
		inteiro = inteiro[:len(inteiro)-3]
	}
	grupos = append([]string{inteiro}, grupos...)

	return "R$ " + strings.Join(grupos, ".") + "," + centavos
}
