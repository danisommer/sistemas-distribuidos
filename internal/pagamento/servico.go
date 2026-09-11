// Package pagamento implementa o microsserviço de processamento de
// pagamentos.
//
// Consome pedido.estoque_ok; publica pagamento.aprovado ou
// pagamento.recusado.
package pagamento

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"log"
	mrand "math/rand"
	"os"
	"strconv"
	"sync"
	"time"

	"ecommerce/internal/evento"
)

// Publicador é o pedaço da mensageria que o Pagamento usa. Depender da
// interface, e não de *mensageria.Publicador, deixa o serviço ser testado
// sem subir broker nenhum.
type Publicador interface {
	PublicarECommerce(ctx context.Context, routingKey string, dados any) error
}

// taxaAprovacaoPadrao é a fração de pagamentos aprovados na simulação.
const taxaAprovacaoPadrao = 0.7

// Tempo simulado de conversa com a operadora de cartão.
const (
	atrasoMinimoPadrao = 400 * time.Millisecond
	atrasoMaximoPadrao = 1500 * time.Millisecond
)

// motivosRecusa são as justificativas sorteadas quando o pagamento cai.
var motivosRecusa = []string{
	"saldo insuficiente",
	"cartão expirado",
	"transação negada pelo emissor",
	"suspeita de fraude",
	"limite diário excedido",
}

// Servico processa os pagamentos dos pedidos que passaram pelo estoque.
type Servico struct {
	mu sync.Mutex

	// processados evita cobrar duas vezes o mesmo pedido se o broker
	// reentregar uma mensagem cujo ack se perdeu.
	processados map[string]string

	taxaAprovacao float64

	// Faixa do atraso simulado. São campos, e não constantes, para os
	// testes poderem zerar a espera.
	atrasoMinimo time.Duration
	atrasoMaximo time.Duration

	publicador Publicador
}

// NovoServico monta o serviço. A taxa de aprovação pode ser ajustada pela
// variável de ambiente TAXA_APROVACAO, útil para a defesa: com 0 todo
// pagamento é recusado e dá para mostrar o estorno do estoque na hora.
func NovoServico(publicador Publicador) *Servico {
	taxa := taxaAprovacaoPadrao
	if bruto := os.Getenv("TAXA_APROVACAO"); bruto != "" {
		if valor, err := strconv.ParseFloat(bruto, 64); err == nil && valor >= 0 && valor <= 1 {
			taxa = valor
		} else {
			log.Printf("TAXA_APROVACAO=%q inválida, usando %.2f", bruto, taxa)
		}
	}

	log.Printf("simulação de pagamento com %.0f%% de aprovação", taxa*100)

	return &Servico{
		processados:   make(map[string]string),
		taxaAprovacao: taxa,
		atrasoMinimo:  atrasoMinimoPadrao,
		atrasoMaximo:  atrasoMaximoPadrao,
		publicador:    publicador,
	}
}

// Tratar é o ponto de entrada do consumidor.
func (s *Servico) Tratar(ctx context.Context, env evento.Envelope) error {
	if env.Tipo != evento.PedidoEstoqueOK {
		log.Printf("evento %s ignorado: não é assunto do Pagamento", env.Tipo)
		return nil
	}

	var pedido evento.DadosPedidoEstoqueOK
	if err := env.DecodificarDados(&pedido); err != nil {
		return err
	}

	if s.jaProcessado(pedido.PedidoID) {
		log.Printf("pedido %s já foi cobrado, reentrega ignorada", pedido.PedidoID)
		return nil
	}

	log.Printf("processando pagamento do pedido %s no valor de R$ %.2f", pedido.PedidoID, pedido.Total)
	s.simularComunicacaoComOperadora(ctx)

	// O enunciado pede que a aprovação seja decidida por variável aleatória.
	if mrand.Float64() < s.taxaAprovacao {
		return s.aprovar(ctx, pedido)
	}
	return s.recusar(ctx, pedido)
}

func (s *Servico) aprovar(ctx context.Context, pedido evento.DadosPedidoEstoqueOK) error {
	transacao := "TX-" + identificador()
	s.registrar(pedido.PedidoID, transacao)

	log.Printf("✓ pagamento do pedido %s APROVADO (transação %s)", pedido.PedidoID, transacao)

	return s.publicador.PublicarECommerce(ctx, evento.PagamentoAprovado, evento.DadosPagamentoAprovado{
		PedidoID:    pedido.PedidoID,
		TransacaoID: transacao,
		Valor:       pedido.Total,
		Itens:       pedido.Itens,
	})
}

func (s *Servico) recusar(ctx context.Context, pedido evento.DadosPedidoEstoqueOK) error {
	motivo := motivosRecusa[mrand.Intn(len(motivosRecusa))]
	s.registrar(pedido.PedidoID, "recusado: "+motivo)

	log.Printf("✗ pagamento do pedido %s RECUSADO (%s)", pedido.PedidoID, motivo)

	return s.publicador.PublicarECommerce(ctx, evento.PagamentoRecusado, evento.DadosPagamentoRecusado{
		PedidoID: pedido.PedidoID,
		Valor:    pedido.Total,
		Motivo:   motivo,
	})
}

// simularComunicacaoComOperadora segura o processamento por um tempo
// aleatório, só para o fluxo não parecer instantâneo na demonstração.
func (s *Servico) simularComunicacaoComOperadora(ctx context.Context) {
	faixa := s.atrasoMaximo - s.atrasoMinimo
	if faixa <= 0 {
		return
	}

	espera := s.atrasoMinimo + time.Duration(mrand.Int63n(int64(faixa)))
	select {
	case <-time.After(espera):
	case <-ctx.Done():
	}
}

func (s *Servico) jaProcessado(pedidoID string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, existe := s.processados[pedidoID]
	return existe
}

func (s *Servico) registrar(pedidoID, resultado string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.processados[pedidoID] = resultado
}

func identificador() string {
	b := make([]byte, 6)
	if _, err := rand.Read(b); err != nil {
		return strconv.FormatInt(time.Now().UnixNano(), 16)
	}
	return hex.EncodeToString(b)
}
