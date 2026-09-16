package promocoes

import (
	"context"
	"ecommerce/internal/evento"
	"fmt"
	"math/rand/v2"
	"time"
)

const (
	TEMPO_PROMOCOES_MIN_MS      = 3000
	TEMPO_PROMOCOES_MAX_MS      = 8000
	DESCONTO_MIN                = 5
	DESCONTO_MAX                = 45
	DESCONTO_VALIDADE_MAX_HORAS = 10
)

type publicador interface {
	PublicarPromocao(ctx context.Context, categoria string, dados any) error
}

type Produto struct {
	ID, Nome, Categoria string
	Preco               float32
}

var produtos = []Produto{
	{ID: "P001", Nome: "Notebook Gamer 16GB", Categoria: "A", Preco: 4500.00},
	{ID: "P002", Nome: "Mouse sem fio", Categoria: "A", Preco: 150.00},
	{ID: "P003", Nome: "Teclado mecânico", Categoria: "A", Preco: 320.00},
	{ID: "P004", Nome: "Cafeteira expresso", Categoria: "B", Preco: 280.00},
	{ID: "P005", Nome: "Liquidificador 900W", Categoria: "B", Preco: 190.00},
	{ID: "P006", Nome: "Air Fryer 5L", Categoria: "B", Preco: 450.00},
	{ID: "P007", Nome: "Camiseta algodão", Categoria: "C", Preco: 79.90},
	{ID: "P008", Nome: "Tênis de corrida", Categoria: "C", Preco: 350.00},
	{ID: "P009", Nome: "Mochila 30L", Categoria: "C", Preco: 210.00},
}

var Categorias = []string{"A", "B", "C"}

type Servico struct {
	publicador publicador
}

func NovoServico(publicador publicador) *Servico {
	return &Servico{publicador}
}

func (s *Servico) EnviarPromocoes(ctx context.Context) error {
	timer := time.NewTimer(s.obterDuracaoAleatoriaEmMs(TEMPO_PROMOCOES_MIN_MS, TEMPO_PROMOCOES_MAX_MS))

	defer timer.Stop()

	for {
		select {
		case <-ctx.Done():
			fmt.Println("Encerrando serviço...")
			return nil
		case <-timer.C:
			if err := s.enviar(ctx); err != nil {
				return fmt.Errorf("Falha ao enviar promocoes. %w", err)
			}

			timer.Reset(s.obterDuracaoAleatoriaEmMs(TEMPO_PROMOCOES_MIN_MS, TEMPO_PROMOCOES_MAX_MS))
		}
	}
}

func (s *Servico) enviar(ctx context.Context) error {
	produtoPromocao := produtos[rand.IntN(len(produtos))]
	descontoPorcentagem := s.gerarDesconto(DESCONTO_MIN, DESCONTO_MAX)
	validadeDesconto := s.gerarValidadeDescontoUTC()
	valorDesconto := produtoPromocao.Preco * float32(100-descontoPorcentagem) / 100

	dadosPromocao := evento.DadosPromocao{
		ProdutoID: produtoPromocao.ID,
		Nome:      produtoPromocao.Nome,
		Categoria: produtoPromocao.Categoria,
		PrecoDe:   produtoPromocao.Preco,
		PrecoPor:  valorDesconto,
		Desconto:  descontoPorcentagem,
		ValidaAte: validadeDesconto,
	}

	if err := s.publicador.PublicarPromocao(ctx, dadosPromocao.Categoria, dadosPromocao); err != nil {
		return fmt.Errorf("Falhar ao publicar mensagem no broker. %w", err)
	}

	return nil
}

func (s *Servico) obterDuracaoAleatoriaEmMs(min int, max int) time.Duration {
	return time.Duration((min + rand.IntN(max-min))) * time.Millisecond
}

func (s *Servico) gerarDesconto(min int, max int) int {
	return (min + rand.IntN(max-min))
}

func (s *Servico) gerarValidadeDescontoUTC() string {
	duracaoDescontoHoras := time.Duration((1 + rand.IntN(DESCONTO_VALIDADE_MAX_HORAS-1))) * time.Hour
	return time.Now().Add(duracaoDescontoHoras).UTC().Format(time.RFC3339)
}
