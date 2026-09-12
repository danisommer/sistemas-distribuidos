package mensageria

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"

	amqp "github.com/rabbitmq/amqp091-go"

	"ecommerce/internal/cripto"
	"ecommerce/internal/evento"
)

// Tratador processa um evento já validado. Devolver erro faz a mensagem ser
// descartada com log, em vez de voltar para a fila: reprocessar em laço um
// evento malformado só entope o broker.
type Tratador func(ctx context.Context, env evento.Envelope) error

// Consumidor é a fila própria de um microsserviço mais o laço que lê dela.
//
// Cada consumidor tem a sua fila, como pede o enunciado, e as bindings dizem
// quais routing keys aquela fila recebe.
type Consumidor struct {
	canal     *amqp.Channel
	fila      string
	chaveiro  *cripto.Chaveiro
	verificar bool
}

// VerificacaoAtiva diz se a assinatura dos eventos recebidos deve ser
// validada. O padrão é sim.
//
// Só existe para destravar o desenvolvimento enquanto internal/cripto/
// verificar.go ainda é um stub. Na entrega e na defesa, deixe ligado.
func VerificacaoAtiva() bool {
	valor := strings.ToLower(strings.TrimSpace(os.Getenv("VERIFICAR_ASSINATURA")))
	return valor != "off" && valor != "0" && valor != "false" && valor != "nao"
}

// NovoConsumidor declara a fila do microsserviço e prepara o laço de leitura.
//
// A fila é durable e as mensagens são confirmadas na mão (ack), então um
// evento em processamento quando o serviço cai volta para a fila e é
// entregue de novo quando ele sobe (tutorial 2).
func NovoConsumidor(conexao *Conexao, fila string, chaveiro *cripto.Chaveiro) (*Consumidor, error) {
	_, err := conexao.Canal.QueueDeclare(
		fila,
		true,  // durable
		false, // auto-delete
		false, // exclusive
		false, // no-wait
		nil,   // argumentos
	)
	if err != nil {
		return nil, fmt.Errorf("declarando fila %s: %w", fila, err)
	}

	// Entrega uma mensagem por vez: o broker só manda a próxima depois do
	// ack da anterior.
	if err := conexao.Canal.Qos(1, 0, false); err != nil {
		return nil, fmt.Errorf("configurando prefetch da fila %s: %w", fila, err)
	}

	c := &Consumidor{
		canal:     conexao.Canal,
		fila:      fila,
		chaveiro:  chaveiro,
		verificar: VerificacaoAtiva(),
	}

	if !c.verificar {
		registro.Printf("ATENÇÃO: VERIFICAR_ASSINATURA=off — a fila %s vai aceitar eventos sem validar a assinatura", fila)
	}

	return c, nil
}

// Vincular liga a fila a uma ou mais routing keys de uma exchange.
//
// Na exchange direct a binding key tem de ser igual à routing key. Na topic
// ela pode usar * e #, que é como o consumidor C2 assina todas as categorias
// de promoção com uma binding só.
func (c *Consumidor) Vincular(exchange string, chaves ...string) error {
	for _, chave := range chaves {
		err := c.canal.QueueBind(
			c.fila,
			chave,
			exchange,
			false, // no-wait
			nil,   // argumentos
		)
		if err != nil {
			return fmt.Errorf("vinculando fila %s a %s com a chave %s: %w", c.fila, exchange, chave, err)
		}
		registro.Printf("fila %s vinculada a %s com a binding key %q", c.fila, exchange, chave)
	}
	return nil
}

// Consumir bloqueia lendo a fila até o contexto ser cancelado ou a conexão
// cair.
func (c *Consumidor) Consumir(ctx context.Context, tratar Tratador) error {
	entregas, err := c.canal.Consume(
		c.fila,
		"",    // consumer tag gerada pelo servidor
		false, // auto-ack desligado: confirmamos na mão
		false, // exclusive
		false, // no-local
		false, // no-wait
		nil,   // argumentos
	)
	if err != nil {
		return fmt.Errorf("consumindo a fila %s: %w", c.fila, err)
	}

	registro.Printf("aguardando eventos na fila %s", c.fila)

	for {
		select {
		case <-ctx.Done():
			return nil

		case entrega, aberto := <-entregas:
			if !aberto {
				return fmt.Errorf("a fila %s parou de entregar (conexão com o broker caiu)", c.fila)
			}
			c.processar(ctx, entrega, tratar)
		}
	}
}

func (c *Consumidor) processar(ctx context.Context, entrega amqp.Delivery, tratar Tratador) {
	var env evento.Envelope
	if err := json.Unmarshal(entrega.Body, &env); err != nil {
		registro.Printf("✗ mensagem ilegível na fila %s, DESCARTADA: %v", c.fila, err)
		entrega.Nack(false, false)
		return
	}

	if c.verificar {
		if err := cripto.VerificarEnvelope(c.chaveiro, env); err != nil {
			if errors.Is(err, cripto.ErrNaoImplementado) {
				registro.Printf("✗ evento %s DESCARTADO: %v", env.Tipo, err)
				registro.Printf("  (para testar sem validação enquanto isso: VERIFICAR_ASSINATURA=off)")
			} else {
				registro.Printf("✗ assinatura inválida no evento %s vindo de %q, DESCARTADO: %v", env.Tipo, env.Produtor, err)
			}
			entrega.Nack(false, false)
			return
		}
	}

	registro.Printf("← recebido %s (evento %s, produtor %s)", env.Tipo, env.ID, env.Produtor)

	if err := tratar(ctx, env); err != nil {
		registro.Printf("✗ erro tratando %s (evento %s): %v", env.Tipo, env.ID, err)
		entrega.Nack(false, false)
		return
	}

	entrega.Ack(false)
}
