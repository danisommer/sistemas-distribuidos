// Package principal implementa o microsserviço Principal: o menu de terminal
// que o usuário opera e o acompanhamento do status de cada pedido.
//
// Publica pedido.criado e pedido.excluido. Consome pedido.estoque_ok,
// estoque.indisponivel, pagamento.aprovado, pagamento.recusado e
// pedido.enviado, atualizando o status dos respectivos pedidos.
package principal

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"sync"
	"time"

	"ecommerce/internal/evento"
)

// Publicador é o pedaço da mensageria que o Principal usa.
type Publicador interface {
	PublicarECommerce(ctx context.Context, routingKey string, dados any) error
}

// Status é o estágio em que um pedido está.
type Status string

const (
	StatusAguardandoEstoque Status = "aguardando estoque"
	StatusEstoqueReservado  Status = "estoque reservado"
	StatusPagamentoAprovado Status = "pagamento aprovado"
	StatusEnviado           Status = "enviado"
	StatusSemEstoque        Status = "cancelado: sem estoque"
	StatusPagamentoRecusado Status = "cancelado: pagamento recusado"
	StatusCanceladoPeloUso  Status = "cancelado pelo usuário"
)

// Encerrado diz se o pedido chegou ao fim da linha, para bem ou para mal.
// Um pedido encerrado não aceita mais exclusão pelo menu.
func (s Status) Encerrado() bool {
	switch s {
	case StatusEnviado, StatusSemEstoque, StatusPagamentoRecusado, StatusCanceladoPeloUso:
		return true
	default:
		return false
	}
}

// Cancelado diz se o pedido terminou sem ser entregue.
func (s Status) Cancelado() bool {
	switch s {
	case StatusSemEstoque, StatusPagamentoRecusado, StatusCanceladoPeloUso:
		return true
	default:
		return false
	}
}

// Marca é uma linha do histórico do pedido.
type Marca struct {
	Quando  time.Time
	Status  Status
	Detalhe string
}

// Pedido é a visão do Principal sobre um pedido, montada a partir dos eventos
// que chegam.
type Pedido struct {
	ID        string
	Cliente   string
	Itens     []evento.ItemPedido
	Total     float64
	Status    Status
	Detalhe   string
	CriadoEm  time.Time
	Historico []Marca
}

// Registro guarda os pedidos da sessão em memória e é seguro para uso
// concorrente.
type Registro struct {
	mu      sync.RWMutex
	pedidos map[string]*Pedido
	ordem   []string
}

// NovoRegistro cria um registro vazio.
func NovoRegistro() *Registro {
	return &Registro{pedidos: make(map[string]*Pedido)}
}

// Criar registra um pedido novo e devolve uma cópia dele.
func (r *Registro) Criar(cliente string, itens []evento.ItemPedido) Pedido {
	r.mu.Lock()
	defer r.mu.Unlock()

	agora := time.Now()
	pedido := &Pedido{
		ID:       novoIDPedido(),
		Cliente:  cliente,
		Itens:    itens,
		Total:    totalDe(itens),
		Status:   StatusAguardandoEstoque,
		CriadoEm: agora,
		Historico: []Marca{
			{Quando: agora, Status: StatusAguardandoEstoque, Detalhe: "pedido criado"},
		},
	}

	r.pedidos[pedido.ID] = pedido
	r.ordem = append(r.ordem, pedido.ID)
	return *pedido
}

// Atualizar move o pedido para um novo status e guarda o histórico. Devolve
// falso se o pedido não existe.
func (r *Registro) Atualizar(pedidoID string, status Status, detalhe string) (Pedido, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()

	pedido, existe := r.pedidos[pedidoID]
	if !existe {
		return Pedido{}, false
	}

	pedido.Status = status
	pedido.Detalhe = detalhe
	pedido.Historico = append(pedido.Historico, Marca{
		Quando:  time.Now(),
		Status:  status,
		Detalhe: detalhe,
	})
	return *pedido, true
}

// Buscar devolve uma cópia do pedido.
func (r *Registro) Buscar(pedidoID string) (Pedido, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	pedido, existe := r.pedidos[pedidoID]
	if !existe {
		return Pedido{}, false
	}
	return *pedido, true
}

// Listar devolve cópias de todos os pedidos, na ordem em que foram criados.
func (r *Registro) Listar() []Pedido {
	r.mu.RLock()
	defer r.mu.RUnlock()

	lista := make([]Pedido, 0, len(r.ordem))
	for _, id := range r.ordem {
		if pedido, existe := r.pedidos[id]; existe {
			lista = append(lista, *pedido)
		}
	}
	return lista
}

// Quantidade devolve quantos pedidos existem.
func (r *Registro) Quantidade() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.ordem)
}

func totalDe(itens []evento.ItemPedido) float64 {
	var total float64
	for _, item := range itens {
		total += item.PrecoUnit * float64(item.Quantidade)
	}
	return total
}

// novoIDPedido gera um identificador curto e aleatório.
func novoIDPedido() string {
	b := make([]byte, 3)
	if _, err := rand.Read(b); err != nil {
		return fmt.Sprintf("PED-%06d", time.Now().UnixNano()%1000000)
	}
	return "PED-" + hex.EncodeToString(b)
}
