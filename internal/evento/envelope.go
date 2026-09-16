// Package evento define o envelope comum a todos os eventos do sistema, os
// nomes das exchanges e as routing keys usadas pelos microsserviços.
//
// Este pacote é o contrato entre os cinco microsserviços: qualquer mudança
// aqui precisa ser combinada entre a dupla antes de valer.
package evento

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"
)

// Nomes das exchanges declaradas no RabbitMQ.
//
// eCommerce é do tipo direct: a mensagem vai para as filas cuja binding key é
// exatamente igual à routing key do evento.
//
// promocoes é do tipo topic: a binding key aceita os curingas * (exatamente
// uma palavra) e # (zero ou mais palavras).
const (
	ExchangeECommerce = "eCommerce"
	ExchangePromocoes = "promocoes"
)

// Routing keys da exchange eCommerce.
const (
	PedidoCriado        = "pedido.criado"
	PedidoExcluido      = "pedido.excluido"
	PedidoEstoqueOK     = "pedido.estoque_ok"
	EstoqueIndisponivel = "estoque.indisponivel"
	PagamentoAprovado   = "pagamento.aprovado"
	PagamentoRecusado   = "pagamento.recusado"
	PedidoEnviado       = "pedido.enviado"
)

// PrefixoPromocao antecede a categoria nas routing keys da exchange
// promocoes. A chave completa fica "promocao.categoria.A", por exemplo.
const PrefixoPromocao = "promocao.categoria."

// ChavePromocao monta a routing key de promoção de uma categoria.
func ChavePromocao(categoria string) string {
	return PrefixoPromocao + categoria
}

// Nomes dos microsserviços. O nome identifica o produtor do evento e também a
// pasta de chaves usada por ele, em chaves/<nome>.
const (
	ServicoPrincipal    = "principal"
	ServicoEstoque      = "estoque"
	ServicoPagamento    = "pagamento"
	ServicoEntrega      = "entrega"
	ServicoPromocoes    = "promocoes"
	ServicoConsumidores = "consumidores"
)

// Servicos lista todos os microsserviços que assinam eventos. É usada para
// gerar os pares de chaves e para distribuir as chaves públicas.
var Servicos = []string{
	ServicoPrincipal,
	ServicoEstoque,
	ServicoPagamento,
	ServicoEntrega,
	ServicoPromocoes,
}

// Envelope é o formato único de toda mensagem que trafega pelo broker. Dados
// carrega o payload específico do evento e Signature carrega a assinatura
// digital em base64, produzida com a chave privada de Produtor.
type Envelope struct {
	ID        string          `json:"id"`
	Tipo      string          `json:"tipo"`
	Produtor  string          `json:"produtor"`
	Timestamp string          `json:"timestamp"`
	Dados     json.RawMessage `json:"dados"`
	Signature string          `json:"signature"`
}

// NovoEnvelope monta um envelope ainda sem assinatura, serializando dados
// como JSON no campo Dados. Quem assina é o publicador.
func NovoEnvelope(tipo, produtor string, dados any) (Envelope, error) {
	bruto, err := json.Marshal(dados)
	if err != nil {
		return Envelope{}, fmt.Errorf("serializando dados do evento %s: %w", tipo, err)
	}
	return Envelope{
		ID:        novoID(),
		Tipo:      tipo,
		Produtor:  produtor,
		Timestamp: time.Now().Format(time.RFC3339Nano),
		Dados:     bruto,
	}, nil
}

// BytesCanonicos devolve a representação determinística do evento, que é o
// conteúdo hasheado e assinado. O campo Signature fica de fora justamente
// porque ele é o resultado dessa operação.
//
// Produtor e consumidor precisam gerar exatamente os mesmos bytes, senão a
// validação falha mesmo com a mensagem intacta. Por isso todos os campos são
// strings e o payload é mantido como json.RawMessage: a ida e volta pelo
// encoding/json preserva os bytes originais em vez de reserializar o objeto.
func (e Envelope) BytesCanonicos() ([]byte, error) {
	semAssinatura := struct {
		ID        string          `json:"id"`
		Tipo      string          `json:"tipo"`
		Produtor  string          `json:"produtor"`
		Timestamp string          `json:"timestamp"`
		Dados     json.RawMessage `json:"dados"`
	}{
		ID:        e.ID,
		Tipo:      e.Tipo,
		Produtor:  e.Produtor,
		Timestamp: e.Timestamp,
		Dados:     e.Dados,
	}

	conteudo, err := json.Marshal(semAssinatura)
	if err != nil {
		return nil, fmt.Errorf("montando bytes canônicos do evento %s: %w", e.Tipo, err)
	}
	return conteudo, nil
}

// DecodificarDados desserializa o payload do envelope em destino.
func (e Envelope) DecodificarDados(destino any) error {
	if err := json.Unmarshal(e.Dados, destino); err != nil {
		return fmt.Errorf("decodificando dados do evento %s: %w", e.Tipo, err)
	}
	return nil
}

func novoID() string {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		return time.Now().Format("20060102150405.000000000")
	}
	return hex.EncodeToString(b)
}
