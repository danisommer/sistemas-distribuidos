package principal

import (
	"context"
	"strings"
	"testing"
	"unicode/utf8"

	"ecommerce/internal/catalogo"
	"ecommerce/internal/evento"
)

// rodarMenu roteiriza o teclado e devolve o que apareceu na tela. O menu
// recebe entrada e saída por interface justamente para isto: dá para testar
// o fluxo inteiro do terminal sem ninguém digitando.
func rodarMenu(t *testing.T, teclado string) (*Registro, *publicadorFalso, string) {
	t.Helper()

	registro := NovoRegistro()
	pub := &publicadorFalso{}
	var tela strings.Builder

	menu := NovoMenu(registro, pub, strings.NewReader(teclado), &tela)
	if err := menu.Rodar(context.Background()); err != nil {
		t.Fatalf("rodando o menu: %v", err)
	}
	return registro, pub, tela.String()
}

func TestMenuRealizaPedidoEPublicaPedidoCriado(t *testing.T) {
	// nome, opção 2, produto 1, quantidade 2, fecha a lista, confirma, sai
	registro, pub, _ := rodarMenu(t, "Daniel\n2\n1\n2\n\ns\n0\n")

	if len(pub.publicados) != 1 || pub.publicados[0].chave != evento.PedidoCriado {
		t.Fatalf("esperava publicar %s, publicou %v", evento.PedidoCriado, pub.chaves())
	}

	dados, ok := pub.publicados[0].dados.(evento.DadosPedidoCriado)
	if !ok {
		t.Fatalf("payload inesperado: %T", pub.publicados[0].dados)
	}
	if dados.Cliente != "Daniel" {
		t.Fatalf("cliente errado: %q", dados.Cliente)
	}
	if len(dados.Itens) != 1 || dados.Itens[0].ProdutoID != "P001" || dados.Itens[0].Quantidade != 2 {
		t.Fatalf("itens errados: %+v", dados.Itens)
	}
	// P001 custa 4500.
	if dados.Total != 9000 {
		t.Fatalf("total errado: %v", dados.Total)
	}

	if registro.Quantidade() != 1 {
		t.Fatalf("esperava 1 pedido no registro, vieram %d", registro.Quantidade())
	}
}

func TestMenuAceitaCodigoDoProdutoAlemDoNumero(t *testing.T) {
	_, pub, _ := rodarMenu(t, "Daniel\n2\np004\n3\n\ns\n0\n")

	dados := pub.publicados[0].dados.(evento.DadosPedidoCriado)
	if dados.Itens[0].ProdutoID != "P004" {
		t.Fatalf("esperava P004, veio %q", dados.Itens[0].ProdutoID)
	}
}

// O mesmo produto escolhido duas vezes vira uma linha só com a soma, em vez
// de duas linhas que o Estoque teria de agregar.
func TestMenuSomaOMesmoProdutoEscolhidoDuasVezes(t *testing.T) {
	_, pub, _ := rodarMenu(t, "Daniel\n2\n1\n2\n1\n3\n\ns\n0\n")

	dados := pub.publicados[0].dados.(evento.DadosPedidoCriado)
	if len(dados.Itens) != 1 {
		t.Fatalf("esperava 1 item somado, vieram %d: %+v", len(dados.Itens), dados.Itens)
	}
	if dados.Itens[0].Quantidade != 5 {
		t.Fatalf("esperava quantidade 5, veio %d", dados.Itens[0].Quantidade)
	}
}

func TestMenuNaoPublicaPedidoNaoConfirmado(t *testing.T) {
	registro, pub, _ := rodarMenu(t, "Daniel\n2\n1\n1\n\nn\n0\n")

	if len(pub.publicados) != 0 {
		t.Fatalf("não deveria publicar nada, publicou %v", pub.chaves())
	}
	if registro.Quantidade() != 0 {
		t.Fatal("pedido recusado não deveria entrar no registro")
	}
}

func TestMenuNaoPublicaPedidoVazio(t *testing.T) {
	_, pub, _ := rodarMenu(t, "Daniel\n2\n\n0\n")

	if len(pub.publicados) != 0 {
		t.Fatalf("não deveria publicar nada, publicou %v", pub.chaves())
	}
}

func TestMenuRejeitaQuantidadeInvalida(t *testing.T) {
	// quantidade 0 é recusada, depois o usuário desiste e sai
	_, pub, tela := rodarMenu(t, "Daniel\n2\n1\n0\n\n0\n")

	if len(pub.publicados) != 0 {
		t.Fatalf("não deveria publicar nada, publicou %v", pub.chaves())
	}
	if !strings.Contains(tela, "Quantidade inválida") {
		t.Fatal("a tela não avisou sobre a quantidade inválida")
	}
}

func TestMenuExcluiPedidoEPublicaPedidoExcluido(t *testing.T) {
	// cria um pedido, depois opção 3, pedido 1, confirma, sai
	registro, pub, _ := rodarMenu(t, "Daniel\n2\n1\n1\n\ns\n3\n1\ns\n0\n")

	if len(pub.publicados) != 2 {
		t.Fatalf("esperava pedido.criado e pedido.excluido, veio %v", pub.chaves())
	}
	if pub.publicados[1].chave != evento.PedidoExcluido {
		t.Fatalf("segundo evento deveria ser %s, veio %s", evento.PedidoExcluido, pub.publicados[1].chave)
	}

	pedidos := registro.Listar()
	if pedidos[0].Status != StatusCanceladoPeloUso {
		t.Fatalf("status após exclusão: %q", pedidos[0].Status)
	}
}

// Um pedido já enviado não pode ser excluído: a mercadoria saiu.
func TestMenuRecusaExcluirPedidoJaEncerrado(t *testing.T) {
	registro := NovoRegistro()
	pub := &publicadorFalso{}
	var tela strings.Builder

	pedido := registro.Criar("Daniel", []evento.ItemPedido{
		{ProdutoID: "P001", Nome: "Notebook Gamer 16GB", Quantidade: 1, PrecoUnit: 4500},
	})
	registro.Atualizar(pedido.ID, StatusEnviado, "nota NF-0001")

	menu := NovoMenu(registro, pub, strings.NewReader("Daniel\n3\n1\n0\n"), &tela)
	if err := menu.Rodar(context.Background()); err != nil {
		t.Fatalf("rodando o menu: %v", err)
	}

	if len(pub.publicados) != 0 {
		t.Fatalf("não deveria publicar nada, publicou %v", pub.chaves())
	}
	if !strings.Contains(tela.String(), "não pode mais ser excluído") {
		t.Fatal("a tela não explicou por que a exclusão foi recusada")
	}
}

func TestMenuListaOsProdutos(t *testing.T) {
	_, _, tela := rodarMenu(t, "Daniel\n1\n0\n")

	for _, esperado := range []string{"P001", "Notebook Gamer 16GB", "P009", "Mochila 30L"} {
		if !strings.Contains(tela, esperado) {
			t.Fatalf("a vitrine não mostrou %q", esperado)
		}
	}
}

func TestMenuMostraStatusNaConsulta(t *testing.T) {
	// cria o pedido e depois consulta
	_, _, tela := rodarMenu(t, "Daniel\n2\n1\n1\n\ns\n4\n\n0\n")

	if !strings.Contains(tela, string(StatusAguardandoEstoque)) {
		t.Fatal("a consulta não mostrou o status do pedido")
	}
}

// A entrada acabando no meio do fluxo encerra o menu sem travar e sem
// publicar nada pela metade.
func TestMenuEncerraQuandoAEntradaAcaba(t *testing.T) {
	_, pub, _ := rodarMenu(t, "Daniel\n2\n1\n")

	if len(pub.publicados) != 0 {
		t.Fatalf("não deveria publicar nada, publicou %v", pub.chaves())
	}
}

// As colunas das tabelas têm de bater mesmo com acento, que ocupa dois bytes
// em UTF-8 e engana o %-22s do fmt.
func TestColunasAlinhamComAcento(t *testing.T) {
	_, _, tela := rodarMenu(t, "Daniel\n1\n0\n")

	var colunas []int
	for _, linha := range strings.Split(tela, "\n") {
		if indice := strings.Index(linha, "cat "); indice >= 0 {
			colunas = append(colunas, utf8.RuneCountInString(linha[:indice]))
		}
	}

	if len(colunas) != len(catalogo.Produtos) {
		t.Fatalf("esperava %d linhas de produto, achei %d", len(catalogo.Produtos), len(colunas))
	}
	for i, coluna := range colunas {
		if coluna != colunas[0] {
			t.Fatalf("linha %d começa a coluna da categoria em %d, e a primeira em %d", i+1, coluna, colunas[0])
		}
	}
}

func TestPreencherContaCaracteresENaoBytes(t *testing.T) {
	casos := []string{"Teclado mecânico", "Notebook Gamer 16GB", "Camiseta algodão", "TOTAL"}

	for _, texto := range casos {
		if largura := utf8.RuneCountInString(preencher(texto, 22)); largura != 22 {
			t.Errorf("preencher(%q, 22) deu %d caracteres", texto, largura)
		}
	}

	// Texto maior que a largura não é cortado.
	longo := strings.Repeat("a", 30)
	if preencher(longo, 22) != longo {
		t.Error("preencher não deveria cortar texto maior que a largura")
	}
}

func TestMoedaNoPadraoBrasileiro(t *testing.T) {
	casos := map[float64]string{
		0:          "R$ 0,00",
		79.90:      "R$ 79,90",
		4500:       "R$ 4.500,00",
		1234567.89: "R$ 1.234.567,89",
	}

	for valor, esperado := range casos {
		if veio := moeda(valor); veio != esperado {
			t.Errorf("moeda(%v): esperava %q, veio %q", valor, esperado, veio)
		}
	}
}
