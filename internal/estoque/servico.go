// Package estoque implementa o microsserviço de gerenciamento de estoque.
//
// Consome pedido.criado e pedido.excluido; publica pedido.estoque_ok ou
// estoque.indisponivel.
package estoque

import (
	"context"
	"fmt"
	"log"
	"sort"
	"strings"
	"sync"

	"ecommerce/internal/catalogo"
	"ecommerce/internal/evento"
)

// Publicador é o pedaço da mensageria que o Estoque usa.
type Publicador interface {
	PublicarECommerce(ctx context.Context, routingKey string, dados any) error
}

// Servico guarda, em memória, o saldo de cada produto e o que está reservado
// por pedido.
type Servico struct {
	mu sync.Mutex

	saldo    map[string]int
	reservas map[string][]evento.ItemPedido

	publicador Publicador
}

// NovoServico inicia o estoque com as quantidades do catálogo.
func NovoServico(publicador Publicador) *Servico {
	saldo := make(map[string]int, len(catalogo.Produtos))
	for _, p := range catalogo.Produtos {
		saldo[p.ID] = p.QuantidadeInicial
	}

	s := &Servico{
		saldo:      saldo,
		reservas:   make(map[string][]evento.ItemPedido),
		publicador: publicador,
	}

	log.Printf("estoque inicial: %s", s.Resumo())
	return s
}

// Tratar é o ponto de entrada do consumidor: despacha pela routing key.
func (s *Servico) Tratar(ctx context.Context, env evento.Envelope) error {
	switch env.Tipo {
	case evento.PedidoCriado:
		return s.aoPedidoCriado(ctx, env)
	case evento.PedidoExcluido:
		return s.aoPedidoExcluido(ctx, env)
	default:
		log.Printf("evento %s ignorado: não é assunto do Estoque", env.Tipo)
		return nil
	}
}

// aoPedidoCriado verifica a disponibilidade dos produtos do pedido. Dando
// tudo certo, baixa o estoque e publica pedido.estoque_ok. Faltando algum
// item, não mexe em nada e publica estoque.indisponivel.
func (s *Servico) aoPedidoCriado(ctx context.Context, env evento.Envelope) error {
	var pedido evento.DadosPedidoCriado
	if err := env.DecodificarDados(&pedido); err != nil {
		return err
	}

	falta, err := s.reservar(pedido)
	if err != nil {
		return err
	}

	if falta != nil {
		log.Printf("pedido %s recusado: %s", pedido.PedidoID, falta.Motivo)
		return s.publicador.PublicarECommerce(ctx, evento.EstoqueIndisponivel, *falta)
	}

	log.Printf("pedido %s reservado. Estoque agora: %s", pedido.PedidoID, s.Resumo())

	return s.publicador.PublicarECommerce(ctx, evento.PedidoEstoqueOK, evento.DadosPedidoEstoqueOK{
		PedidoID: pedido.PedidoID,
		Itens:    pedido.Itens,
		Total:    pedido.Total,
	})
}

// reservar confere e baixa o estoque do pedido numa única seção crítica.
//
// Devolve nil quando reservou ou quando o pedido já estava reservado.
// Devolve o motivo preenchido quando não deu, e nesse caso o estoque fica
// exatamente como estava.
func (s *Servico) reservar(pedido evento.DadosPedidoCriado) (*evento.DadosEstoqueIndisponivel, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, jaReservado := s.reservas[pedido.PedidoID]; jaReservado {
		log.Printf("pedido %s já estava reservado, reentrega ignorada", pedido.PedidoID)
		return nil, nil
	}

	if len(pedido.Itens) == 0 {
		return &evento.DadosEstoqueIndisponivel{
			PedidoID: pedido.PedidoID,
			Motivo:   "pedido sem itens",
		}, nil
	}

	necessario := make(map[string]int, len(pedido.Itens))
	for _, item := range pedido.Itens {
		if item.Quantidade <= 0 {
			return &evento.DadosEstoqueIndisponivel{
				PedidoID:   pedido.PedidoID,
				ProdutoID:  item.ProdutoID,
				Nome:       item.Nome,
				Solicitado: item.Quantidade,
				Disponivel: s.saldo[item.ProdutoID],
				Motivo:     fmt.Sprintf("quantidade inválida para %s", item.ProdutoID),
			}, nil
		}
		necessario[item.ProdutoID] += item.Quantidade
	}

	for _, produtoID := range idsOrdenados(necessario) {
		quantidade := necessario[produtoID]

		disponivel, existe := s.saldo[produtoID]
		if !existe {
			return &evento.DadosEstoqueIndisponivel{
				PedidoID:   pedido.PedidoID,
				ProdutoID:  produtoID,
				Solicitado: quantidade,
				Disponivel: 0,
				Motivo:     fmt.Sprintf("produto %s não existe no catálogo", produtoID),
			}, nil
		}

		if disponivel < quantidade {
			nome := produtoID
			if p, ok := catalogo.Buscar(produtoID); ok {
				nome = p.Nome
			}
			return &evento.DadosEstoqueIndisponivel{
				PedidoID:   pedido.PedidoID,
				ProdutoID:  produtoID,
				Nome:       nome,
				Solicitado: quantidade,
				Disponivel: disponivel,
				Motivo: fmt.Sprintf("estoque insuficiente de %s: pedidas %d, disponíveis %d",
					nome, quantidade, disponivel),
			}, nil
		}
	}

	for produtoID, quantidade := range necessario {
		s.saldo[produtoID] -= quantidade
	}

	reservado := make([]evento.ItemPedido, len(pedido.Itens))
	copy(reservado, pedido.Itens)
	s.reservas[pedido.PedidoID] = reservado

	return nil, nil
}

// aoPedidoExcluido devolve ao estoque o que havia sido reservado para o
// pedido. Sem reserva, não altera nada.
func (s *Servico) aoPedidoExcluido(_ context.Context, env evento.Envelope) error {
	var excluido evento.DadosPedidoExcluido
	if err := env.DecodificarDados(&excluido); err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	itens, existe := s.reservas[excluido.PedidoID]
	if !existe {
		log.Printf("pedido %s excluído (%s), mas não havia reserva: nada a devolver",
			excluido.PedidoID, excluido.Motivo)
		return nil
	}

	for _, item := range itens {
		s.saldo[item.ProdutoID] += item.Quantidade
	}
	delete(s.reservas, excluido.PedidoID)

	log.Printf("pedido %s excluído (%s): %d item(ns) devolvidos. Estoque agora: %s",
		excluido.PedidoID, excluido.Motivo, len(itens), s.resumoSemTrava())
	return nil
}

// Resumo devolve o saldo atual em uma linha, para o log.
func (s *Servico) Resumo() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.resumoSemTrava()
}

// resumoSemTrava monta o texto assumindo que quem chamou já segura o mutex.
func (s *Servico) resumoSemTrava() string {
	var partes []string
	for _, p := range catalogo.Produtos {
		partes = append(partes, fmt.Sprintf("%s=%d", p.ID, s.saldo[p.ID]))
	}
	return strings.Join(partes, " ")
}

func idsOrdenados(m map[string]int) []string {
	ids := make([]string, 0, len(m))
	for id := range m {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}
